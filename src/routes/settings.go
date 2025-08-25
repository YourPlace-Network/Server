package routes

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/db/blockchain"
	"YourPlace/src/core/host"
	"YourPlace/src/core/middleware"
	"YourPlace/src/core/network"
	"YourPlace/src/core/security"
	"YourPlace/src/core/services"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func SettingsRoutes(router *gin.Engine, title string, database *db.Database, _blockchain *blockchain.Blockchain, cryptoSeed []byte, gateway bool, ipfs *network.IPFS, debug bool) {
	defaultUploadDirectory := host.GetDataDir() + "upload" + host.PathSeparator

	router.GET("/settings", func(c *gin.Context) { // Settings View
		token := middleware.GetCSRFToken(c)
		authenticated := false
		authCookie, err := c.Request.Cookie("yp_auth")
		if err == nil && security.ValidateCookie(authCookie, cryptoSeed, database) {
			authenticated = true
		}
		c.HTML(http.StatusOK, "src/templates/pages/settings.tmpl", gin.H{
			"title":                 title,
			"pageName":              "settings",
			"csrfToken":             token,
			"isCookieAuthenticated": authenticated,
			"gatewayMode":           gateway,
		})
	})
	router.GET("/settings/uploadDirectory", func(c *gin.Context) { // Get file upload directory
		uploadDirectory := database.SettingsGetValue("uploadDirectory")
		if uploadDirectory == "" {
			uploadDirectory = defaultUploadDirectory
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"uploadDirectory": uploadDirectory,
		})
	})
	router.GET("/settings/ipfs/port", func(c *gin.Context) {
		ipfsPort := database.SettingsGetValue("ipfsPort")
		if ipfsPort == "" {
			ipfsPort = "42423"
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"port": ipfsPort,
		})
	})
	router.GET("/settings/base/url", func(c *gin.Context) {
		baseURL := database.SettingsGetValue("baseURL")
		c.SecureJSON(http.StatusOK, gin.H{
			"baseURL": baseURL,
		})
	})
	router.GET("/settings/base/dataDirectory", func(c *gin.Context) {
		dataDirectory := host.GetDataDir()
		baseDataDirectory := database.SettingsGetValue("baseDataDirectory")
		if baseDataDirectory != "" {
			dataDirectory = baseDataDirectory
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"dataDirectory": dataDirectory,
		})
	})
	router.GET("/settings/base/fullNode", func(c *gin.Context) {
		baseFullNode := database.SettingsGetValue("baseFullNode")
		baseFullNodeTemp := false
		if baseFullNode == "true" {
			baseFullNodeTemp = true
		} else {
			baseFullNodeTemp = false
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"baseFullNode": baseFullNodeTemp,
		})
	})
	router.GET("/settings/base/throttle", func(c *gin.Context) {
		throttle := database.SettingsGetValue("baseThrottle")
		throttleInt, err := strconv.Atoi(throttle)
		if err != nil {
			c.SecureJSON(http.StatusBadRequest, gin.H{"status": "failed"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"throttle": throttleInt,
		})
	})
	router.GET("/settings/post/history", func(c *gin.Context) {
		historyDays := database.SettingsGetValue("historyDays")
		historyDaysInt, err := strconv.Atoi(historyDays)
		if err != nil {
			c.SecureJSON(http.StatusBadRequest, gin.H{"status": "Could not get post history days"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"days": historyDaysInt,
		})
	})
	router.GET("/settings/ai/spiceometer", func(c *gin.Context) {
		spiceometer := database.SettingsGetValue("spiceometer")
		spiceometerInt, err := strconv.Atoi(spiceometer)
		if err != nil {
			c.SecureJSON(http.StatusBadRequest, gin.H{"status": "Could not get spiceometer value"})
			return
		}
		status := http.StatusMethodNotAllowed
		if spiceometerInt == 1 {
			status = http.StatusOK
		}
		c.SecureJSON(status, gin.H{
			"enable": spiceometerInt,
		})
	})
	router.GET("/settings/ai/ollama/enabled", func(c *gin.Context) {
		client := &http.Client{
			Timeout: 5 * time.Second,
		}
		resp, err := client.Get("http://localhost:11434/api/health")
		if err != nil {
			c.SecureJSON(http.StatusServiceUnavailable, gin.H{"status": "Ollama disabled"})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			c.SecureJSON(http.StatusOK, gin.H{"status": "Ollama enabled"})
			return
		} else {
			c.SecureJSON(http.StatusServiceUnavailable, gin.H{"status": "Ollama disabled"})
			return
		}
	})
	router.GET("/settings/database/importSnapshotStatus", func(c *gin.Context) {
		status := database.MetaGetValue("importSnapshotStatus")
		c.SecureJSON(http.StatusOK, gin.H{"status": status})
	})
	router.GET("/settings/database/exportSnapshotStatus", func(c *gin.Context) {
		status := database.MetaGetValue("exportSnapshotStatus")
		c.SecureJSON(http.StatusOK, gin.H{"status": status})
	})
	router.GET("/settings/indexer/status", func(c *gin.Context) {
		baseUUID := database.IndexerGetJobUUID("base")
		baseIndexerStatus := database.IndexerGetJobStatus(baseUUID)
		c.SecureJSON(http.StatusOK, gin.H{
			"status": baseIndexerStatus,
		})
	})
	router.GET("/settings/indexer/running", func(c *gin.Context) {
		indexerRunning := database.SettingsGetValue("indexerRunning")
		indexerRunningBool := false
		if indexerRunning == "true" {
			indexerRunningBool = true
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"indexerRunning": indexerRunningBool,
		})
	})
	router.GET("/settings/indexer/onBattery", func(c *gin.Context) {
		indexerOnBattery := database.SettingsGetValue("indexerOnBattery")
		indexerOnBatteryBool := false
		if indexerOnBattery == "true" {
			indexerOnBatteryBool = true
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"indexerOnBattery": indexerOnBatteryBool,
		})
	})
	router.GET("/settings/content/ipfsPinning", func(c *gin.Context) {
		pinningURL := database.SettingsGetValue("ipfsPinningURL")
		pinningKey := host.GetSecret("ipfsPinningKey")
		pinningKeyMasked := ""
		if len(pinningKey) > 0 {
			pinningKeyMasked = "**********"
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"pinningURL": pinningURL,
			"pinningKey": pinningKeyMasked,
		})
	})
	router.GET("/settings/base/indexerProgress", func(c *gin.Context) {
		earliestBlock := _blockchain.GetEarliestBlock("base")
		jobUUID := database.IndexerGetJobUUID("base")
		tailBlock := database.IndexerGetTailBlock(jobUUID)
		headBlock := database.IndexerGetHeadBlock(jobUUID)
		latestBlock, err := _blockchain.GetLatestBlock("base")
		if err != nil || latestBlock == big.NewInt(0) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"status": "Could not get Base latest block"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"earliestBlock": earliestBlock,
			"tailBlock":     tailBlock,
			"headBlock":     headBlock,
			"latestBlock":   latestBlock,
		})
	})
	router.GET("/settings/server/debug", func(c *gin.Context) {
		c.SecureJSON(http.StatusOK, gin.H{
			"debug": debug,
		})
	})
	router.GET("/settings/server/version", func(c *gin.Context) {
		version := host.GetServerVersion()
		helperVersion, err := host.HelperCall("version")
		if err != nil {
			core.LogError("Error getting helper version: " + err.Error())
			helperVersion = "?"
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"version":       version,
			"helperVersion": helperVersion,
		})
	})
	router.GET("/settings/server/logs/view", func(c *gin.Context) {
		log, logPath := core.LogRead(200, 3)
		if log == "" || logPath == "" {
			log = "No logs available"
			logPath = ""
		}
		c.SecureJSON(http.StatusOK, gin.H{"logs": log, "logPath": logPath})
	})
	router.GET("/settings/helper/logs/view", func(c *gin.Context) {
		log, logPath := core.LogReadHelper(200, 3)
		if log == "" || logPath == "" {
			log = "No logs available"
			logPath = ""
		}
		c.SecureJSON(http.StatusOK, gin.H{"logs": log, "logPath": logPath})
	})
	router.GET("/settings/database/snapshotDirectory", func(c *gin.Context) {
		snapshotPath := host.GetDataDir() + "yourplace.db.snapshot"
		c.SecureJSON(http.StatusOK, gin.H{"snapshotDirectory": snapshotPath})
	})

	router.POST("/settings/uploadDirectory", func(c *gin.Context) {
		type Payload struct {
			UploadDirectory string `json:"uploadDirectory" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid directory json"})
			return
		}
		if payload.UploadDirectory == "default" {
			database.SettingsUpdateValue("uploadDirectory", defaultUploadDirectory)
			c.SecureJSON(http.StatusOK, gin.H{"status": "success", "defaultUploadDirectory": defaultUploadDirectory})
			return
		}
		_uploadDirectory := security.SanitizePathTraversal(payload.UploadDirectory)
		_uploadDirectory = security.SanitizeCommandInjection(_uploadDirectory)
		if host.DoesExist(_uploadDirectory) {
			database.SettingsUpdateValue("uploadDirectory", _uploadDirectory)
			c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Directory does not exist"})
		return
	})
	router.POST("/settings/base/url", func(c *gin.Context) {
		type Payload struct {
			BaseURL string `json:"baseURL" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Base JSON"})
			return
		}
		if payload.BaseURL == "default" {
			database.SettingsUpdateValue("baseURL", blockchain.DefaultBlockchainNodes["base"][0])
			c.SecureJSON(http.StatusOK, gin.H{"status": "success", "defaultBaseURL": blockchain.DefaultBlockchainNodes["base"][0]})
			return
		}
		if !security.IsValidURL(payload.BaseURL) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Base URL"})
			return
		}
		database.SettingsUpdateValue("baseURL", payload.BaseURL)
		blockchain.IndexerStop()
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/base/dataDirectory", func(c *gin.Context) {
		type Payload struct {
			DataDirectory string `json:"dataDirectory" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Base Data Directory JSON"})
			return
		}
		if payload.DataDirectory == "default" {
			database.SettingsUpdateValue("baseDataDirectory", host.GetDataDir())
			c.SecureJSON(http.StatusOK, gin.H{"status": "success", "baseDataDirectory": host.GetDataDir()})
			return
		}
		if host.DoesExist(payload.DataDirectory) && host.IsDirWriteable(payload.DataDirectory) {
			database.SettingsUpdateValue("baseDataDirectory", payload.DataDirectory)
			c.SecureJSON(http.StatusOK, gin.H{"status": "success", "baseDataDirectory": payload.DataDirectory})
			return
		} else {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Directory does not exist"})
			return
		}
	})
	router.POST("/settings/base/fullNode", func(c *gin.Context) {
		type Payload struct {
			BaseFullNode bool `json:"baseFullNode"`
		}
		var payload Payload
		err := c.ShouldBindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Base Full Node JSON"})
			return
		}
		if payload.BaseFullNode {
			database.SettingsUpdateValue("baseFullNode", "true")
		} else {
			database.SettingsUpdateValue("baseFullNode", "false")
		}
		c.SecureJSON(http.StatusOK, gin.H{"baseFullNode": payload.BaseFullNode})
	})
	router.POST("/settings/base/throttle", func(c *gin.Context) {
		type Payload struct {
			Throttle int `json:"throttle" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Base throttle value"})
			return
		}
		if !security.IsValidNumberRange(payload.Throttle, 0, 10000) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid throttle range"})
			return
		}
		database.SettingsUpdateValue("baseThrottle", strconv.Itoa(payload.Throttle))
		blockchain.IndexerStop()
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/base/indexerReset", func(c *gin.Context) {
		type Payload struct {
			IndexerReset bool `json:"indexerReset" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Base indexer reset value"})
			return
		}
		if payload.IndexerReset {
			blockchain.IndexerStop()
			database.IndexerResetJobs("base")
			c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
		}
	})
	router.POST("/settings/post/history", func(c *gin.Context) {
		type Payload struct {
			Days int `json:"days" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid directory json"})
			return
		}
		if !security.IsValidNumberRange(payload.Days, 1, 365) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid days range"})
			return
		}
		database.SettingsUpdateValue("historyDays", strconv.Itoa(payload.Days))
	})
	router.POST("/settings/ai/spiceometer", func(c *gin.Context) {
		type Payload struct {
			Spiceometer int `json:"enable" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid spiceometer json"})
			return
		}
		if !security.IsValidNumberRange(payload.Spiceometer, 0, 1) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid spiceometer value"})
			return
		}
		if payload.Spiceometer == 1 {
			go services.OllamaSetup()
			database.SettingsUpdateValue("spiceometer", strconv.Itoa(payload.Spiceometer))
		} else if payload.Spiceometer == 0 {
			database.SettingsUpdateValue("spiceometer", strconv.Itoa(payload.Spiceometer))
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/indexer/stop", func(c *gin.Context) {
		database.SettingsUpdateValue("indexerRunning", "false")
		blockchain.IndexerStop()
		c.SecureJSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Indexer stopped",
		})
	})
	router.POST("/settings/indexer/start", func(c *gin.Context) {
		database.SettingsUpdateValue("indexerRunning", "true")
		c.SecureJSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Indexer started",
		})
	})
	router.POST("/settings/indexer/onBattery", func(c *gin.Context) {
		type Payload struct {
			IndexerOnBattery bool `json:"indexerOnBattery" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid indexerOnBattery json"})
			return
		}
		if payload.IndexerOnBattery == true {
			database.SettingsUpdateValue("indexerOnBattery", "true")
		} else {
			database.SettingsUpdateValue("indexerOnBattery", "false")
		}
		blockchain.IndexerStop()
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/database/exportSnapshot", func(c *gin.Context) {
		exportDB := host.GetDataDir() + "yourplace.db.snapshot"
		database.MetaUpdateValue("exportSnapshotStatus", "running")
		go func() {
			host.DeleteIfExists(exportDB)
			err := database.ExportSnapshot(exportDB)
			if err != nil {
				core.LogDebug("Error sanitizing database: " + err.Error())
				database.MetaUpdateValue("exportSnapshotStatus", "failed")
				return
			}
			database.MetaUpdateValue("exportSnapshotStatus", "complete")
		}()
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "exportPath": exportDB})
	})
	router.POST("/settings/database/importSnapshot", func(c *gin.Context) {
		dataDir := host.GetDataDir()
		pattern := filepath.Join(dataDir, "snapshotsyourplacelast*")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Error searching for snapshot files"})
			return
		}
		if len(matches) == 0 {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"status": "No snapshot file found with prefix 'yourplacelast'"})
			return
		}
		// Use the first matching file (or most recent if multiple)
		var importPath string
		if len(matches) == 1 {
			importPath = matches[0]
		} else {
			// If multiple matches, use the most recently modified
			var latestTime time.Time
			for _, match := range matches {
				info, err := os.Stat(match)
				if err == nil && info.ModTime().After(latestTime) {
					latestTime = info.ModTime()
					importPath = match
				}
			}
		}
		database.MetaUpdateValue("importSnapshotStatus", "running")
		blockchain.IndexerStop()
		go func() {
			for i := 0; i < 100; i++ {
				uuids := database.IndexerGetRunningJobsUUIDs()
				if len(uuids) == 0 {
					break
				}
				time.Sleep(5 * time.Second)
			}
			err := database.ImportSnapshot(importPath)
			if err != nil {
				database.MetaUpdateValue("importSnapshotStatus", "failed")
				return
			}
			database.MetaUpdateValue("importSnapshotStatus", "complete")
		}()
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "importPath": importPath})
	})
	router.POST("/settings/content/ipfsPinning", func(c *gin.Context) {
		type Payload struct {
			PinningURL string `json:"pinningURL" required:"true"`
			PinningKey string `json:"pinningKey" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid IPFS Pinning JSON"})
			return
		}
		if !security.IsValidURL(payload.PinningURL) || len(payload.PinningKey) <= 5 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid IPFS Pinning URL or Key"})
			return
		}
		url := payload.PinningURL
		if !security.IsHttpProtocol(payload.PinningURL) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid IPFS Pinning URL or Key"})
			return
		}
		host.AddSecret("ipfsPinningKey", security.SanitizeNonPrintable(payload.PinningKey))
		database.SettingsUpdateValue("ipfsPinningURL", url)
		success := ipfs.IPFSAddRemotePinning("ipfsPinning", url, payload.PinningKey)
		if success {
			c.SecureJSON(http.StatusOK, gin.H{"status": "IPFS URL and Key saved"})
		} else {
			c.SecureJSON(http.StatusInternalServerError, gin.H{"status": "Failed to configure IPFS pinning service. Please check your URL and credentials."})
		}
	})
	router.POST("/settings/server/debug", func(c *gin.Context) {
		type Payload struct {
			Debug bool `json:"debug" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid debug mode JSON"})
			return
		}
		host.SetDebugMode(payload.Debug)
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/uninstall", func(c *gin.Context) {
		type Payload struct {
			UploadFiles    bool `json:"uploadFiles" required:"true"`
			BlockchainData bool `json:"blockchainData" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid uninstall JSON"})
			return
		}
		uninstalled := host.UnInstall(payload.UploadFiles, payload.BlockchainData)
		if uninstalled {
			c.SecureJSON(http.StatusOK, gin.H{"status": "success", "message": "Uninstalling YourPlace. Please wait..."})
		} else {
			c.SecureJSON(http.StatusInternalServerError, gin.H{"status": "failed"})
		}
	})
	router.POST("/settings/privacy/torHiddenService", func(c *gin.Context) {
		type Payload struct {
			Enabled bool `json:"enabled" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid TOR hidden service JSON"})
			return
		}
		if payload.Enabled {
			_, err = network.StartTorHiddenService()
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Failed to start TOR hidden service"})
				return
			}
			database.SettingsUpdateValue("torHiddenService", "true")
		} else {
			_, err = network.StopTorHiddenService()
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Failed to stop TOR hidden service"})
				return
			}
			database.SettingsUpdateValue("torHiddenService", "false")
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "enabled": payload.Enabled})
	})
}
