package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/db/blockchain"
	"YourPlace/src/core/host"
	"YourPlace/src/core/network"
	"YourPlace/src/core/security"
	"YourPlace/src/core/services"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"math/big"
	"net/http"
	"strconv"
	"time"
)

func SettingsRoutes(router *gin.Engine, title string, database *db.Database, _blockchain *blockchain.Blockchain, cryptoSeed []byte, gateway bool, ipfs *network.IPFS) {
	defaultUploadDirectory := host.GetDataDir() + "upload" + host.PathSeparator

	router.GET("/settings", func(c *gin.Context) { // Settings View
		token := csrf.Token(c.Request)
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
	router.GET("/settings/indexer/status", func(c *gin.Context) {
		blockchain.IndexerMutex.Lock()
		status := blockchain.IsIndexing
		blockchain.IndexerMutex.Unlock()
		c.SecureJSON(http.StatusOK, gin.H{
			"status":     "success",
			"isIndexing": status,
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
		pinningKeyMasked := security.MaskToken(pinningKey)
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
		if !security.IsValidNumberRange(payload.Throttle, 1, 250) {
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
		blockchain.IndexerStop()
		c.SecureJSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Indexer stopped",
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
		exportPath := host.GetDataDir() + "yourplace.db.snapshot"
		err := database.ExportSnapshot(exportPath)
		if err != nil {
			c.SecureJSON(http.StatusBadRequest, gin.H{"status": "failed", "error": err.Error()})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "exportPath": exportPath})
	})
	router.POST("/settings/database/importSnapshot", func(c *gin.Context) {
		importPath := host.GetDataDir() + "yourplace.db.snapshot"
		// todo halt indexer prior to import
		err := database.ImportSnapshot(importPath)
		if err != nil {
			c.SecureJSON(http.StatusBadRequest, gin.H{"status": "failed", "error": err.Error()})
			return
		}
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
			url = "https://" + url
		}
		host.AddSecret("ipfsPinningKey", security.SanitizeNonPrintable(payload.PinningKey))
		database.SettingsUpdateValue("ipfsPinningURL", url)
		ipfs.IPFSAddRemotePinning("ipfsPinning", url, payload.PinningKey)
		c.SecureJSON(http.StatusOK, gin.H{"status": "IPFS URL and Key saved"})
	})
}
