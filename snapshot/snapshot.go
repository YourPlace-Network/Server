package main

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/db/blockchain"
	"YourPlace/src/core/host"
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	_cron "github.com/robfig/cron/v3"
)

var (
	debugMode = false // set via 'd' command line flag or debug file in data directory
	dataDir   = filepath.Join(host.GetHomeDir(), "YourPlaceSnapshot")
)

func main() {
	debugPtr := flag.Bool("d", false, "Enable Debug mode, default: false")
	flag.Parse()
	debugMode = *debugPtr
	host.CreateFolder(dataDir)
	core.LogInit("YourPlaceSnapshot")
	core.LogInfo("YourPlace Snapshot Service")
	if debugMode || host.DoesExist(host.GetDataDir()+"debug") {
		debugMode = true
		core.LogInfo("Running in debug mode")
		host.SetEnvVar("YourPlaceSnapshotDebug", "true")
	} else {
		host.DeleteEnvVar("YourPlaceSnapshotDebug")
	}
	debug.SetGCPercent(50)               // Run GC when heap grows by 50% (default is 100%)
	runtime.GOMAXPROCS(runtime.NumCPU()) // Set max goroutines to prevent runaway memory usage
	database := new(db.Database)
	dbPath := filepath.Join(dataDir, "yourplacesnapshot.sqlite.db")
	database.Init(dbPath, "sqlite")
	snapshotDir := filepath.Join(dataDir, "snapshots")
	if !database.Ping() {
		core.LogError("Could not connect to database")
		os.Exit(1)
	}
	database.SetDefaults()
	// Base URL and throttle must be set as environment variables BASE_RPC_URL and BASE_RPC_THROTTLE
	requiredVar := []string{"BASE_RPC_URL", "S3_ENDPOINT", "S3_BUCKET_NAME", "BASE_RPC_THROTTLE"}
	for _, v := range requiredVar {
		if host.GetEnvVar(v) == "" {
			core.LogError(v + " environment variable not set")
			os.Exit(1)
		}
	}
	baseURL := host.GetEnvVar("BASE_RPC_URL")
	baseThrottle := host.GetEnvVar("BASE_RPC_THROTTLE")
	database.SettingsUpdateValue("baseURL", baseURL)
	database.SettingsUpdateValue("baseThrottle", baseThrottle)
	_blockchain := new(blockchain.Blockchain)
	_blockchain.Init(database)
	c := _cron.New(_cron.WithSeconds())
	blockchain.IndexerRestartJobs(database, "base")
	c.AddFunc("@every 2m", func() {
		// Only log and run if indexer is not already running
		blockchain.IndexerMutex.Lock()
		if blockchain.IsIndexing {
			blockchain.IndexerMutex.Unlock()
			return // Silently skip if already running
		}
		blockchain.IndexerMutex.Unlock()
		core.LogDebug("Starting Base indexer run")
		blockchain.IndexerFetchData(database, _blockchain, "base")
		runtime.GC() // Force GC after indexer run to free memory
	})
	c.AddFunc("@every 60m", func() {
		// Only export snapshots if indexer is at 99% or higher
		progress := getIndexerProgress(database, _blockchain, "base")
		if progress < 99.0 {
			core.LogInfo("Skipping snapshot export - indexer progress is " + strconv.FormatFloat(progress, 'f', 2, 64) + "% (needs 99%+)")
			return
		}
		core.LogDebug("Exporting snapshots (indexer at " + strconv.FormatFloat(progress, 'f', 2, 64) + "%)")
		runtime.GC() // Free memory before snapshot export
		host.DeleteAll(snapshotDir)
		host.CreateFolder(snapshotDir)
		err := database.ExportSnapshots(snapshotDir)
		if err != nil {
			core.LogError("Error exporting snapshots:" + err.Error())
			return
		}
		handleS3Upload(snapshotDir)
		runtime.GC() // Free memory after snapshot export
	})
	c.Start()
	<-make(chan struct{})
}
func getIndexerProgress(database *db.Database, _blockchain *blockchain.Blockchain, blockchainName string) float64 {
	uuid := database.IndexerGetJobUUID(blockchainName)
	if uuid == "" {
		return 0.0
	}
	headBlock := database.IndexerGetHeadBlock(uuid)
	tailBlock := database.IndexerGetTailBlock(uuid)
	if headBlock == 0 || tailBlock == 0 {
		return 0.0
	}
	// Get the earliest block from blockchain configuration
	targetEarliestBlock := _blockchain.GetEarliestBlock(blockchainName)
	if targetEarliestBlock == nil {
		return 0.0
	}
	// Calculate progress: how much of the range from tail to head has been indexed
	totalRange := float64(headBlock - targetEarliestBlock.Uint64())
	if totalRange <= 0 {
		return 100.0
	}
	indexedRange := float64(headBlock - tailBlock)
	progress := (indexedRange / totalRange) * 100.0
	if progress > 100.0 {
		progress = 100.0
	}
	return progress
}
func handleS3Upload(snapshotDir string) {
	snapshotFiles, err := filepath.Glob(filepath.Join(snapshotDir, "yourplace*.db.snapshot"))
	if err != nil {
		core.LogError("Error globbing snapshot files for S3 upload" + err.Error())
		return
	}
	s3Endpoint := os.Getenv("S3_ENDPOINT")
	bucketName := os.Getenv("S3_BUCKET_NAME")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")
	var cfg aws.Config
	if accessKey != "" && secretKey != "" {
		// Use static credentials for DigitalOcean Spaces or custom S3
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(
					accessKey,
					secretKey,
					"",
				),
			),
			config.WithRegion("us-east-1"),
		)
	} else {
		// Use IAM role credentials for AWS (ECS task role)
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion("us-east-1"),
		)
	}
	if err != nil {
		core.LogError("Error loading S3 configuration: " + err.Error())
		return
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(s3Endpoint)
	})
	uploader := manager.NewUploader(client)
	for _, file := range snapshotFiles {
		snapshotFile, err := os.Open(file)
		if err != nil {
			core.LogError("Error opening snapshot file: " + err.Error())
			continue
		}
		defer snapshotFile.Close()
		core.LogInfo("Uploading snapshot file: " + file)
		_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(filepath.Base(file)),
			Body:   snapshotFile,
		})
		if err != nil {
			core.LogError("Error uploading snapshot file: " + err.Error())
		}
	}
	var fileNames []string
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(os.Getenv("S3_BUCKET_NAME")),
	}
	paginator := s3.NewListObjectsV2Paginator(client, params)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(context.TODO())
		if err != nil {
			core.LogError("Error listing S3 objects: " + err.Error())
			return
		}
		for _, obj := range output.Contents {
			if obj.Key != nil {
				fileNames = append(fileNames, *obj.Key)
			}
		}
	}
	var timestamps []int64
	for _, fileName := range fileNames {
		re := regexp.MustCompile(`yourplace(\d+)-`)
		matches := re.FindStringSubmatch(fileName)
		if len(matches) < 2 {
			continue
		}
		timestamp, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			core.LogError("Error parsing timestamp: " + err.Error())
			return
		}
		found := false
		for _, t := range timestamps {
			if t == timestamp {
				found = true
				break
			}
		}
		if !found {
			timestamps = append(timestamps, timestamp)
		}
	}
	if len(timestamps) >= 10 {
		oldest := slices.Min(timestamps)
		var objectsToDelete []types.ObjectIdentifier
		for _, fileName := range fileNames {
			re := regexp.MustCompile(`yourplace(\d+)-`)
			matches := re.FindStringSubmatch(fileName)
			if len(matches) < 2 {
				continue
			}
			if len(matches) > 1 {
				timestamp, err := strconv.ParseInt(matches[1], 10, 64)
				if err == nil && timestamp == oldest {
					objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
						Key: aws.String(fileName),
					})
				}
			}
		}
		if len(objectsToDelete) > 0 {
			_, err := client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
				Bucket: aws.String(bucketName),
				Delete: &types.Delete{
					Objects: objectsToDelete,
				},
			})
			if err != nil {
				core.LogError("Error deleting old snapshot files: " + err.Error())
			} else {
				core.LogInfo("Deleted " + strconv.Itoa(len(objectsToDelete)) + " old snapshot files")
			}
		}
	}
}
