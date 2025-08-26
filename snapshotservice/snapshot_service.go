package main

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/db/blockchain"
	"YourPlace/src/core/host"
	"flag"
	_cron "github.com/robfig/cron/v3"
	"os"
	"path/filepath"
)

var (
	debug   = false // set via 'd' command line flag or debug file in data directory
	dataDir = filepath.Join(host.GetHomeDir(), "YourPlaceSnapshot")
)

func main() {
	debugPtr := flag.Bool("d", false, "Enable Debug mode, default: false")
	flag.Parse()
	debug = *debugPtr
	host.CreateFolder(dataDir)
	core.LogInit("yourplacesnapshot")
	if debug || host.DoesExist(host.GetDataDir()+"debug") {
		debug = true
		core.LogInfo("Running in debug mode")
		host.SetEnvVar("YourPlaceSnapshotDebug", "true")
	} else {
		host.DeleteEnvVar("YourPlaceSnapshotDebug")
	}
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
	baseURL := host.GetEnvVar("BASE_RPC_URL")
	if baseURL == "" {
		core.LogWarn("BASE_RPC_URL environment variable not set. Defaulting to public RPC node (slow)")
	}
	baseThrottle := host.GetEnvVar("BASE_RPC_THROTTLE")
	if baseThrottle == "" {
		core.LogWarn("BASE_RPC_THROTTLE environment variable not set. Defaulting to 5 (slow)")
	}
	database.SettingsUpdateValue("baseURL", baseURL)
	database.SettingsUpdateValue("baseThrottle", baseThrottle)
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
		database.ExportSnapshots(snapshotDir)
	})
	c.Start()
	//TODO: add graceful shutdown
	<-make(chan struct{})

}
