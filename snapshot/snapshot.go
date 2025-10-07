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
		// Only log if indexer actually starts (returns true)
		if blockchain.IndexerFetchData(database, _blockchain, "base") {
			core.LogDebug("Starting Base indexer run")
		}
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
		uuid := database.IndexerGetJobUUID("base")
		headBlock := database.IndexerGetHeadBlock(uuid)
		tailBlock := database.IndexerGetTailBlock(uuid)
		err := database.ExportSnapshots(snapshotDir, headBlock, tailBlock)
		if err != nil {
			core.LogError("Error exporting snapshots:" + err.Error())
			return
		}
		handleS3Upload(snapshotDir, headBlock, tailBlock)
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
func handleS3Upload(snapshotDir string, headBlock uint64, tailBlock uint64) {
	snapshotFiles, err := filepath.Glob(filepath.Join(snapshotDir, "yourplace-snapshot-*.db.gz"))
	if err != nil {
		core.LogError("Error globbing snapshot files for S3 upload: " + err.Error())
		return
	}
	if len(snapshotFiles) == 0 {
		core.LogError("No snapshot files found to upload in " + snapshotDir)
		return
	}
	s3Endpoint := os.Getenv("S3_ENDPOINT")
	bucketName := os.Getenv("S3_BUCKET_NAME")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")
	core.LogInfo("S3 Upload Configuration - Endpoint: " + s3Endpoint + ", Bucket: " + bucketName + ", Using IAM: " + strconv.FormatBool(accessKey == ""))
	var cfg aws.Config
	if accessKey != "" && secretKey != "" {
		// Use static credentials for DigitalOcean Spaces or custom S3
		core.LogDebug("Using static credentials for S3")
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
		core.LogDebug("Using IAM role credentials for S3")
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
	uploadedCount := 0
	failedCount := 0
	for _, file := range snapshotFiles {
		snapshotFile, err := os.Open(file)
		if err != nil {
			core.LogError("Error opening snapshot file '" + file + "': " + err.Error())
			failedCount++
			continue
		}
		fileInfo, err := snapshotFile.Stat()
		if err != nil {
			core.LogError("Error getting file info for '" + file + "': " + err.Error())
			snapshotFile.Close()
			failedCount++
			continue
		}
		core.LogInfo("Uploading snapshot file: " + filepath.Base(file) + " (size: " + strconv.FormatInt(fileInfo.Size(), 10) + " bytes)")
		result, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(filepath.Base(file)),
			Body:   snapshotFile,
		})
		snapshotFile.Close()
		if err != nil {
			core.LogError("FAILED to upload snapshot file '" + filepath.Base(file) + "': " + err.Error())
			failedCount++
		} else {
			core.LogInfo("Successfully uploaded snapshot to: " + result.Location)
			uploadedCount++
		}
	}
	core.LogInfo("S3 Upload Summary - Uploaded: " + strconv.Itoa(uploadedCount) + ", Failed: " + strconv.Itoa(failedCount) + ", Total: " + strconv.Itoa(len(snapshotFiles)))
	if failedCount > 0 {
		core.LogError("Some snapshots failed to upload - check logs above for details")
		return
	}
	core.LogDebug("Listing S3 objects to clean up old snapshots")
	var fileNames []string
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String("yourplace-snapshot-"),
	}
	paginator := s3.NewListObjectsV2Paginator(client, params)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(context.TODO())
		if err != nil {
			core.LogError("Error listing S3 objects for cleanup: " + err.Error())
			return
		}
		for _, obj := range output.Contents {
			if obj.Key != nil {
				fileNames = append(fileNames, *obj.Key)
			}
		}
	}
	core.LogDebug("Found " + strconv.Itoa(len(fileNames)) + " snapshot files in S3")
	var timestamps []int64
	re := regexp.MustCompile(`yourplace-snapshot-ts(\d+)-`)
	for _, fileName := range fileNames {
		matches := re.FindStringSubmatch(fileName)
		if len(matches) < 2 {
			core.LogDebug("Skipping file with unexpected format: " + fileName)
			continue
		}
		timestamp, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			core.LogError("Error parsing timestamp from '" + fileName + "': " + err.Error())
			continue
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
	core.LogDebug("Found " + strconv.Itoa(len(timestamps)) + " unique snapshot timestamps")
	if len(timestamps) >= 10 {
		oldest := slices.Min(timestamps)
		core.LogInfo("Cleaning up old snapshots - oldest timestamp: " + strconv.FormatInt(oldest, 10))
		var objectsToDelete []types.ObjectIdentifier
		for _, fileName := range fileNames {
			matches := re.FindStringSubmatch(fileName)
			if len(matches) >= 2 {
				timestamp, err := strconv.ParseInt(matches[1], 10, 64)
				if err == nil && timestamp == oldest {
					objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
						Key: aws.String(fileName),
					})
					core.LogDebug("Marking for deletion: " + fileName)
				}
			}
		}
		if len(objectsToDelete) > 0 {
			deleteResult, err := client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
				Bucket: aws.String(bucketName),
				Delete: &types.Delete{
					Objects: objectsToDelete,
				},
			})
			if err != nil {
				core.LogError("Error deleting old snapshot files: " + err.Error())
			} else {
				deletedCount := len(deleteResult.Deleted)
				errorCount := len(deleteResult.Errors)
				core.LogInfo("Deleted " + strconv.Itoa(deletedCount) + " old snapshot files")
				if errorCount > 0 {
					core.LogError("Failed to delete " + strconv.Itoa(errorCount) + " files")
					for _, delErr := range deleteResult.Errors {
						core.LogError("Delete error for '" + *delErr.Key + "': " + *delErr.Message)
					}
				}
			}
		}
	} else {
		core.LogDebug("Not enough snapshot sets for cleanup (have " + strconv.Itoa(len(timestamps)) + ", need 10)")
	}
}
