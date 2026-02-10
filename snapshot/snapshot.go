package main

import (
	"YourPlace/src/core"
	blockchain2 "YourPlace/src/core/blockchain"
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"context"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	database.SetDefaultSettings()
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
	algoURL := host.GetEnvVar("ALGO_RPC_URL")
	if algoURL == "" {
		algoURL = blockchain2.DefaultBlockchainNodes["algorand"][0]
	}
	algoThrottle := host.GetEnvVar("ALGO_RPC_THROTTLE")
	if algoThrottle == "" {
		algoThrottle = blockchain2.DefaultBlockchainNodes["algorand"][1]
	}
	algoToken := host.GetEnvVar("ALGO_TOKEN")
	database.SettingsUpdateValue("algoURL", algoURL)
	database.SettingsUpdateValue("algoThrottle", algoThrottle)
	if algoToken != "" {
		database.SettingsUpdateValue("algodToken", algoToken)
	}
	_blockchain := new(blockchain2.Blockchain)
	_blockchain.Init(database)
	c := _cron.New(_cron.WithSeconds())
	blockchain2.BaseIndexerRestartJobs(database, "base")
	blockchain2.AlgoIndexerRestartJobs(database, "algorand")
	c.AddFunc("@every 2m", func() {
		if blockchain2.BaseIndexerFetchData(database, _blockchain) {
			core.LogDebug("Starting Base indexer run")
		}
		if blockchain2.AlgorandIndexerFetchData(database, _blockchain) {
			core.LogDebug("Starting Algorand indexer run")
		}
		runtime.GC()
	})
	c.AddFunc("@every 60m", func() {
		exportBaseSnapshot(database, _blockchain, snapshotDir)
		exportAlgorandSnapshot(database, _blockchain, snapshotDir)
	})
	c.Start()
	<-make(chan struct{})
}
func exportAlgorandSnapshot(database *db.Database, _blockchain *blockchain2.Blockchain, snapshotDir string) {
	progress := getAlgorandIndexerProgress(database, _blockchain)
	if progress < 99.0 {
		core.LogInfo("[Algo] Skipping snapshot export - indexer progress is " + strconv.FormatFloat(progress, 'f', 2, 64) + "% (needs 99%+)")
		return
	}
	core.LogDebug("[Algo] Exporting snapshots (indexer at " + strconv.FormatFloat(progress, 'f', 2, 64) + "%)")
	runtime.GC()
	algoSnapshotDir := filepath.Join(snapshotDir, "algorand")
	host.DeleteAll(algoSnapshotDir)
	host.CreateFolder(algoSnapshotDir)
	uuid := database.IndexerGetJobUUID("algorand")
	headBlock := database.IndexerGetHeadBlock(uuid)
	tailBlock := database.IndexerGetTailBlock(uuid)
	err := database.ExportSnapshots(algoSnapshotDir, "algorand", headBlock, tailBlock)
	if err != nil {
		core.LogError("[Algo] Error exporting snapshots: " + err.Error())
		return
	}
	handleS3Upload(algoSnapshotDir, "algorand", headBlock, tailBlock)
	runtime.GC()
}
func exportBaseSnapshot(database *db.Database, _blockchain *blockchain2.Blockchain, snapshotDir string) {
	progress := getBaseIndexerProgress(database, _blockchain)
	if progress < 99.0 {
		core.LogInfo("[Base] Skipping snapshot export - indexer progress is " + strconv.FormatFloat(progress, 'f', 2, 64) + "% (needs 99%+)")
		return
	}
	core.LogDebug("[Base] Exporting snapshots (indexer at " + strconv.FormatFloat(progress, 'f', 2, 64) + "%)")
	runtime.GC()
	baseSnapshotDir := filepath.Join(snapshotDir, "base")
	host.DeleteAll(baseSnapshotDir)
	host.CreateFolder(baseSnapshotDir)
	uuid := database.IndexerGetJobUUID("base")
	headBlock := database.IndexerGetHeadBlock(uuid)
	tailBlock := database.IndexerGetTailBlock(uuid)
	err := database.ExportSnapshots(baseSnapshotDir, "base", headBlock, tailBlock)
	if err != nil {
		core.LogError("[Base] Error exporting snapshots: " + err.Error())
		return
	}
	handleS3Upload(baseSnapshotDir, "base", headBlock, tailBlock)
	runtime.GC()
}
func getAlgorandIndexerProgress(database *db.Database, _blockchain *blockchain2.Blockchain) float64 {
	uuid := database.IndexerGetJobUUID("algorand")
	if uuid == "" {
		return 0.0
	}
	headBlock := database.IndexerGetHeadBlock(uuid)
	tailBlock := database.IndexerGetTailBlock(uuid)
	if headBlock == 0 || tailBlock == 0 {
		return 0.0
	}
	targetEarliestBlock := _blockchain.GetEarliestBlock("algorand")
	if targetEarliestBlock == nil {
		return 0.0
	}
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
func getBaseIndexerProgress(database *db.Database, _blockchain *blockchain2.Blockchain) float64 {
	uuid := database.IndexerGetJobUUID("base")
	if uuid == "" {
		return 0.0
	}
	headBlock := database.IndexerGetHeadBlock(uuid)
	tailBlock := database.IndexerGetTailBlock(uuid)
	if headBlock == 0 || tailBlock == 0 {
		return 0.0
	}
	targetEarliestBlock := _blockchain.GetEarliestBlock("base")
	if targetEarliestBlock == nil {
		return 0.0
	}
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
func handleS3Upload(snapshotDir string, blockchain string, headBlock uint64, tailBlock uint64) {
	snapshotFiles, err := filepath.Glob(filepath.Join(snapshotDir, blockchain+"-snapshot-*.db.gz"))
	if err != nil {
		core.LogError("Error globbing snapshot files for S3 upload: " + err.Error())
		return
	}
	metadataFiles, err := filepath.Glob(filepath.Join(snapshotDir, blockchain+"-snapshot-*.json"))
	if err != nil {
		core.LogError("Error globbing metadata files for S3 upload: " + err.Error())
		return
	}
	if len(snapshotFiles) == 0 {
		core.LogError("No snapshot files found to upload in " + snapshotDir)
		return
	}
	allFiles := append(snapshotFiles, metadataFiles...)
	s3Endpoint := os.Getenv("S3_ENDPOINT")
	bucketName := os.Getenv("S3_BUCKET_NAME")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")
	core.LogInfo("S3 Upload Configuration - Endpoint: " + s3Endpoint + ", Bucket: " + bucketName + ", Using IAM: " + strconv.FormatBool(accessKey == ""))
	var cfg aws.Config
	if accessKey != "" && secretKey != "" {
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
	for _, file := range allFiles {
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
	core.LogInfo("S3 Upload Summary - Uploaded: " + strconv.Itoa(uploadedCount) + ", Failed: " + strconv.Itoa(failedCount) + ", Total: " + strconv.Itoa(len(allFiles)))
	if failedCount > 0 {
		core.LogError("Some snapshots failed to upload - check logs above for details")
		return
	}
}
