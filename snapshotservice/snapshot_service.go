package main

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/db/blockchain"
	"YourPlace/src/core/host"
	"flag"
	"fmt"
	_cron "github.com/robfig/cron/v3"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	logger      *log.Logger
	loggerMutex sync.Mutex
	debug       = false // set via 'd' command line flag or debug file in data directory
)

const (
	serviceName = "YourPlaceSnapshot"
)

func main() {
	flag.BoolVar(&debug, "d", false, "Enable Debug mode, default: false")
	flag.Parse()
	if debug || host.DoesExist(host.GetDataDir()+"debug") {
		debug = true
		core.LogInit("YourPlaceSnapshot", true)
		core.LogInfo("Running in debug mode")
		host.SetEnvVar("YourPlaceSnapshotDebug", "true")
	} else {
		core.LogInit("YourPlaceSnapshot", true)
		host.DeleteEnvVar("YourPlaceSnapshotDebug")
	}
	core.LogInfo("Initializing database")
	database := new(db.Database)
	database.Init(host.GetHomeDir()+host.PathSeparator, "sqlite", true)
	snapshotDir := filepath.Join(host.GetHomeDir()+host.PathSeparator, "snapshots")
	if !database.Ping() {
		fmt.Println("Could not connect to database")
	}
	database.SetDefaults()
	// Base URL and throttle must be set as environment variables BASE_RPC_URL and BASE_RPC_THROTTLE
	baseURL := database.SettingsGetValue("baseURL")
	var baseThrottle string
	baseURL = os.Getenv("BASE_RPC_URL")
	baseThrottle = os.Getenv("BASE_RPC_THROTTLE")
	database.SettingsUpdateValue("baseURL", baseURL)
	database.SettingsUpdateValue("baseThrottle", baseThrottle)
	core.LogInfo("Initializing blockchain")
	_blockchain := new(blockchain.Blockchain)
	_blockchain.Init(database)
	c := _cron.New(_cron.WithSeconds())
	blockchain.IndexerRestartJobs(database, "base")
	c.AddFunc("@every 2m", func() {
		core.LogDebug("Starting Base indexer run")
		blockchain.IndexerFetchData(database, _blockchain, "base")
	})
	c.AddFunc("@every 60m", func() {
		core.LogDebug("Exporting snapshots")
		host.DeleteAll(snapshotDir)
		host.CreateFolder(snapshotDir)
		database.ExportSnapshotsForService(snapshotDir)
	})
	c.Start()
	//TODO: add graceful shutdown
	<-make(chan struct{})

}
