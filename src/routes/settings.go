package routes

import (
	"YourPlace/src/core"
	blockchain2 "YourPlace/src/core/blockchain"
	"YourPlace/src/core/db"
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

func SettingsRoutes(router *gin.Engine, title string, database *db.Database, _blockchain *blockchain2.Blockchain, cryptoSeed []byte, gateway bool, ipfs *network.IPFS, debug bool) {
	defaultUploadDirectory := host.GetDataDir() + "upload" + host.PathSeparator

	router.GET("/settings", func(c *gin.Context) { // Settings View
		token := middleware.GetCSRFToken(c)
		ipfsGateway := getConfiguredIPFSGateway(database)
		authenticated := false
		userAddress := ""
		userBlockchain := ""
		authCookie, err := c.Request.Cookie("yp_auth")
		if err == nil && security.ValidateCookie(authCookie, cryptoSeed, database) {
			authenticated = true
			userAddress, _ = security.GetCookieValue(authCookie, cryptoSeed, "address", database)
			userBlockchain, _ = security.GetCookieValue(authCookie, cryptoSeed, "blockchain", database)
		}
		c.HTML(http.StatusOK, "src/templates/pages/settings.tmpl", gin.H{
			"title":                 title,
			"pageName":              "settings",
			"csrfToken":             token,
			"ipfsGateway":           ipfsGateway,
			"isCookieAuthenticated": authenticated,
			"gatewayMode":           gateway,
			"userAddress":           userAddress,
			"userBlockchain":        userBlockchain,
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
		if gateway {
			c.SecureJSON(http.StatusOK, gin.H{
				"baseURL": blockchain2.DefaultBlockchainNodes["base"][0],
			})
		} else {
			baseURL := database.SettingsGetValue("baseURL")
			c.SecureJSON(http.StatusOK, gin.H{
				"baseURL": baseURL,
			})
		}
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
		baseURL := database.SettingsGetValue("baseURL")
		isDefault := baseURL == "" || baseURL == blockchain2.DefaultBlockchainNodes["base"][0]
		var throttleInt int
		if isDefault {
			throttleInt, _ = strconv.Atoi(blockchain2.DefaultBlockchainNodes["base"][1])
		} else {
			throttle := database.SettingsGetValue("baseThrottle")
			var err error
			throttleInt, err = strconv.Atoi(throttle)
			if err != nil {
				throttleInt, _ = strconv.Atoi(blockchain2.DefaultBlockchainNodes["base"][1])
			}
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"throttle":  throttleInt,
			"isDefault": isDefault,
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
		indexerOnBattery := database.SettingsGetValue("indexerOnBattery")
		indexerOnBatteryBool, _ := strconv.ParseBool(indexerOnBattery)
		isOnBattery := host.IsOnBattery()
		indexerRunning := database.SettingsGetValue("indexerRunning")
		if indexerRunning != "true" || (isOnBattery && !indexerOnBatteryBool) {
			baseIndexerStatus = "stopped"
		}
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
	router.GET("/settings/base/indexer/running", func(c *gin.Context) {
		indexerRunning := database.SettingsGetValue("baseIndexerRunning")
		indexerRunningBool := indexerRunning != "false"
		c.SecureJSON(http.StatusOK, gin.H{
			"indexerRunning": indexerRunningBool,
		})
	})
	router.GET("/settings/algorand/indexer/running", func(c *gin.Context) {
		indexerRunning := database.SettingsGetValue("algoIndexerRunning")
		indexerRunningBool := indexerRunning != "false"
		c.SecureJSON(http.StatusOK, gin.H{
			"indexerRunning": indexerRunningBool,
		})
	})
	router.GET("/settings/base/indexer/status", func(c *gin.Context) {
		baseUUID := database.IndexerGetJobUUID("base")
		baseIndexerStatus := database.IndexerGetJobStatus(baseUUID)
		indexerOnBattery := database.SettingsGetValue("indexerOnBattery")
		indexerOnBatteryBool, _ := strconv.ParseBool(indexerOnBattery)
		isOnBattery := host.IsOnBattery()
		globalIndexerRunning := database.SettingsGetValue("indexerRunning")
		baseIndexerRunning := database.SettingsGetValue("baseIndexerRunning")
		if globalIndexerRunning != "true" || baseIndexerRunning == "false" || (isOnBattery && !indexerOnBatteryBool) {
			baseIndexerStatus = "stopped"
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"status": baseIndexerStatus,
		})
	})
	router.GET("/settings/algorand/indexer/status", func(c *gin.Context) {
		algoUUID := database.IndexerGetJobUUID("algorand")
		algoIndexerStatus := database.IndexerGetJobStatus(algoUUID)
		indexerOnBattery := database.SettingsGetValue("indexerOnBattery")
		indexerOnBatteryBool, _ := strconv.ParseBool(indexerOnBattery)
		isOnBattery := host.IsOnBattery()
		globalIndexerRunning := database.SettingsGetValue("indexerRunning")
		algoIndexerRunning := database.SettingsGetValue("algoIndexerRunning")
		if globalIndexerRunning != "true" || algoIndexerRunning == "false" || (isOnBattery && !indexerOnBatteryBool) {
			algoIndexerStatus = "stopped"
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"status": algoIndexerStatus,
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
	router.GET("/settings/content/ipfsGateway", func(c *gin.Context) {
		gateway := getConfiguredIPFSGateway(database)
		c.SecureJSON(http.StatusOK, gin.H{
			"gateway": gateway,
		})
	})
	router.GET("/settings/base/indexerProgress", func(c *gin.Context) {
		earliestBlock := _blockchain.GetEarliestBlock("base")
		jobUUID := database.IndexerGetJobUUID("base")
		tailBlock := database.IndexerGetTailBlock(jobUUID)
		headBlock := database.IndexerGetHeadBlock(jobUUID)
		latestBlock, err := _blockchain.GetLatestBlock("base")
		if err != nil || latestBlock.Cmp(big.NewInt(0)) == 0 {
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
	router.GET("/settings/server/runtime", func(c *gin.Context) {
		envVars := make(map[string]string)
		flags := make(map[string]interface{})
		envMySQLDSN := os.Getenv("YOURPLACE_MYSQL_DSN")
		if envMySQLDSN != "" {
			envVars["YOURPLACE_MYSQL_DSN"] = security.MaskDSN(envMySQLDSN)
		}
		envOrigin := os.Getenv("YOURPLACE_ORIGIN")
		if envOrigin != "" {
			envVars["YOURPLACE_ORIGIN"] = envOrigin
		}
		flags["debug"] = debug
		flags["gateway"] = gateway
		c.SecureJSON(http.StatusOK, gin.H{
			"envVars": envVars,
			"flags":   flags,
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
	router.GET("/settings/services/algorand", func(c *gin.Context) {
		algodURL := database.SettingsGetValue("algoURL")
		if algodURL == "" {
			algodURL = blockchain2.DefaultBlockchainNodes["algorand"][0]
		}
		algodToken := database.SettingsGetValue("algodToken")
		c.SecureJSON(http.StatusOK, gin.H{
			"algodURL":   algodURL,
			"algodToken": algodToken,
		})
	})
	router.GET("/settings/services/spotify/clientid", func(c *gin.Context) {
		clientID := services.GetSpotifyClientID(database.SettingsGetValue("spotifyClientId"))
		c.SecureJSON(http.StatusOK, gin.H{
			"clientId":  clientID,
			"envLocked": services.IsSpotifyClientIDEnvLocked(),
		})
	})
	router.GET("/settings/services/xcom/settings", func(c *gin.Context) {
		crossPostEnabled := database.SettingsGetValue("xcomCrossPostEnabled")
		feedAggregationEnabled := database.SettingsGetValue("xcomFeedAggregationEnabled")
		c.SecureJSON(http.StatusOK, gin.H{
			"crossPostEnabled":       crossPostEnabled == "true",
			"feedAggregationEnabled": feedAggregationEnabled == "true",
		})
	})
	router.GET("/settings/services/xcom/credentials", func(c *gin.Context) {
		apiKey := database.SettingsGetValue("xcomApiKey")
		accessToken := database.SettingsGetValue("xcomAccessToken")
		rateLimited := false
		rateLimitRemaining := ""
		rateLimitUntil := database.MetaGetValue("xcomRateLimitUntil")
		if rateLimitUntil != "" {
			rateLimitTime, err := time.Parse(time.RFC3339, rateLimitUntil)
			if err == nil && time.Now().Before(rateLimitTime) {
				rateLimited = true
				rateLimitRemaining = time.Until(rateLimitTime).Round(time.Minute).String()
			}
		}
		if apiKey == "" || accessToken == "" {
			c.SecureJSON(http.StatusOK, gin.H{
				"apiKey":             "",
				"accessToken":        "",
				"hasCredentials":     false,
				"isValid":            false,
				"rateLimited":        rateLimited,
				"rateLimitRemaining": rateLimitRemaining,
			})
			return
		}
		apiSecret := host.GetSecret("xcomApiSecret")
		accessTokenSecret := host.GetSecret("xcomAccessTokenSecret")
		hasCredentials := apiSecret != "" && accessTokenSecret != ""
		isValid := database.SettingsGetValue("xcomCredentialsValid") == "true"
		c.SecureJSON(http.StatusOK, gin.H{
			"apiKey":             apiKey,
			"accessToken":        accessToken,
			"hasCredentials":     hasCredentials,
			"isValid":            isValid,
			"rateLimited":        rateLimited,
			"rateLimitRemaining": rateLimitRemaining,
		})
	})
	router.GET("/settings/services/xcom/test", func(c *gin.Context) {
		rateLimitUntil := database.MetaGetValue("xcomRateLimitUntil")
		if rateLimitUntil != "" {
			rateLimitTime, err := time.Parse(time.RFC3339, rateLimitUntil)
			if err == nil && time.Now().Before(rateLimitTime) {
				remaining := time.Until(rateLimitTime).Round(time.Minute)
				c.SecureJSON(http.StatusOK, gin.H{
					"isValid":     false,
					"rateLimited": true,
					"status":      "X.com API rate limited. Please wait " + remaining.String() + " before testing again.",
				})
				return
			}
		}
		apiKey := database.SettingsGetValue("xcomApiKey")
		accessToken := database.SettingsGetValue("xcomAccessToken")
		if apiKey == "" || accessToken == "" {
			database.SettingsUpdateValue("xcomCredentialsValid", "false")
			c.SecureJSON(http.StatusBadRequest, gin.H{"isValid": false, "status": "X.com credentials not configured"})
			return
		}
		apiSecret := host.GetSecret("xcomApiSecret")
		accessTokenSecret := host.GetSecret("xcomAccessTokenSecret")
		if apiSecret == "" || accessTokenSecret == "" {
			database.SettingsUpdateValue("xcomCredentialsValid", "false")
			c.SecureJSON(http.StatusBadRequest, gin.H{"isValid": false, "status": "X.com credentials not configured"})
			return
		}
		isValid, statusCode := services.XcomTestCredentials(apiKey, apiSecret, accessToken, accessTokenSecret)
		if statusCode == 429 {
			rateLimitExpiry := time.Now().Add(24 * time.Hour)
			database.MetaUpdateValue("xcomRateLimitUntil", rateLimitExpiry.Format(time.RFC3339))
			c.SecureJSON(http.StatusOK, gin.H{
				"isValid":     false,
				"rateLimited": true,
				"status":      "X.com API rate limited. Testing disabled for 24 hours.",
			})
			return
		}
		database.MetaUpdateValue("xcomRateLimitUntil", "")
		if isValid {
			database.SettingsUpdateValue("xcomCredentialsValid", "true")
			c.SecureJSON(http.StatusOK, gin.H{"isValid": true, "status": "X.com credentials are valid"})
		} else {
			database.SettingsUpdateValue("xcomCredentialsValid", "false")
			c.SecureJSON(http.StatusOK, gin.H{"isValid": false, "status": "X.com credentials are invalid"})
		}
	})
	router.GET("/settings/services/xcom/tier", func(c *gin.Context) {
		apiKey := database.SettingsGetValue("xcomApiKey")
		accessToken := database.SettingsGetValue("xcomAccessToken")
		if apiKey == "" || accessToken == "" {
			c.SecureJSON(http.StatusOK, gin.H{"isFreeTier": true, "hasCredentials": false})
			return
		}
		apiSecret := host.GetSecret("xcomApiSecret")
		accessTokenSecret := host.GetSecret("xcomAccessTokenSecret")
		if apiSecret == "" || accessTokenSecret == "" {
			c.SecureJSON(http.StatusOK, gin.H{"isFreeTier": true, "hasCredentials": false})
			return
		}
		isFreeTier := services.XcomIsFreeTier(apiKey, apiSecret, accessToken, accessTokenSecret)
		c.SecureJSON(http.StatusOK, gin.H{"isFreeTier": isFreeTier, "hasCredentials": true})
	})
	router.GET("/settings/services/xcom/scrape/credentials", func(c *gin.Context) {
		email := database.SettingsGetValue("xcomScrapeEmail")
		username := database.SettingsGetValue("xcomScrapeUsername")
		hasPassword := host.GetSecret("xcomScrapePassword") != ""
		isValid := database.SettingsGetValue("xcomScrapeCredentialsValid") == "true"
		c.SecureJSON(http.StatusOK, gin.H{
			"email":       email,
			"username":    username,
			"hasPassword": hasPassword,
			"isValid":     isValid,
		})
	})
	router.GET("/settings/services/xcom/scrape/test", func(c *gin.Context) {
		email := database.SettingsGetValue("xcomScrapeEmail")
		username := database.SettingsGetValue("xcomScrapeUsername")
		password := host.GetSecret("xcomScrapePassword")
		if email == "" || username == "" || password == "" {
			database.SettingsUpdateValue("xcomScrapeCredentialsValid", "")
			c.SecureJSON(http.StatusOK, gin.H{"isValid": false, "status": "X.com scraping credentials not configured"})
			return
		}
		cookies, err := services.LogInToXcom(email, username, password)
		if err != nil {
			database.SettingsUpdateValue("xcomScrapeCredentialsValid", "false")
			c.SecureJSON(http.StatusOK, gin.H{"isValid": false, "status": "Login failed: " + err.Error()})
			return
		}
		if len(cookies) == 0 {
			database.SettingsUpdateValue("xcomScrapeCredentialsValid", "false")
			c.SecureJSON(http.StatusOK, gin.H{"isValid": false, "status": "Login failed: no cookies returned"})
			return
		}
		database.SettingsUpdateValue("xcomScrapeCredentialsValid", "true")
		c.SecureJSON(http.StatusOK, gin.H{"isValid": true})
	})
	router.GET("/settings/algorand/url", func(c *gin.Context) {
		if gateway {
			c.SecureJSON(http.StatusOK, gin.H{
				"algoURL": blockchain2.DefaultBlockchainNodes["algorand"][0],
			})
		} else {
			algoURL := database.SettingsGetValue("algoURL")
			c.SecureJSON(http.StatusOK, gin.H{
				"algoURL": algoURL,
			})
		}
	})
	router.GET("/settings/algorand/throttle", func(c *gin.Context) {
		algoURL := database.SettingsGetValue("algoURL")
		isDefault := algoURL == "" || algoURL == blockchain2.DefaultBlockchainNodes["algorand"][0]
		var throttleInt int
		if isDefault {
			throttleInt, _ = strconv.Atoi(blockchain2.DefaultBlockchainNodes["algorand"][1])
		} else {
			throttle := database.SettingsGetValue("algoThrottle")
			var err error
			throttleInt, err = strconv.Atoi(throttle)
			if err != nil {
				throttleInt, _ = strconv.Atoi(blockchain2.DefaultBlockchainNodes["algorand"][1])
			}
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"throttle":  throttleInt,
			"isDefault": isDefault,
		})
	})
	router.GET("/settings/algorand/indexerProgress", func(c *gin.Context) {
		earliestBlock := _blockchain.GetEarliestBlock("algorand")
		jobUUID := database.IndexerGetJobUUID("algorand")
		tailBlock := database.IndexerGetTailBlock(jobUUID)
		headBlock := database.IndexerGetHeadBlock(jobUUID)
		latestBlock, err := _blockchain.GetLatestBlock("algorand")
		if err != nil || latestBlock.Cmp(big.NewInt(0)) == 0 {
			c.SecureJSON(http.StatusBadRequest, gin.H{"status": "Could not get Algorand latest block"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"earliestBlock": earliestBlock,
			"tailBlock":     tailBlock,
			"headBlock":     headBlock,
			"latestBlock":   latestBlock,
		})
	})
	router.GET("/settings/ethereum/url", func(c *gin.Context) {
		if gateway {
			c.SecureJSON(http.StatusOK, gin.H{
				"ethereumURL": blockchain2.DefaultBlockchainNodes["ethereum"][0],
			})
		} else {
			ethereumURL := database.SettingsGetValue("ethereumURL")
			c.SecureJSON(http.StatusOK, gin.H{
				"ethereumURL": ethereumURL,
			})
		}
	})
	router.GET("/settings/ethereum/throttle", func(c *gin.Context) {
		ethereumURL := database.SettingsGetValue("ethereumURL")
		isDefault := ethereumURL == "" || ethereumURL == blockchain2.DefaultBlockchainNodes["ethereum"][0]
		var throttleInt int
		if isDefault {
			throttleInt, _ = strconv.Atoi(blockchain2.DefaultBlockchainNodes["ethereum"][1])
		} else {
			throttle := database.SettingsGetValue("ethereumThrottle")
			var err error
			throttleInt, err = strconv.Atoi(throttle)
			if err != nil {
				throttleInt, _ = strconv.Atoi(blockchain2.DefaultBlockchainNodes["ethereum"][1])
			}
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"throttle":  throttleInt,
			"isDefault": isDefault,
		})
	})
	router.GET("/settings/ethereum/indexer/running", func(c *gin.Context) {
		indexerRunning := database.SettingsGetValue("ethereumIndexerRunning")
		indexerRunningBool := indexerRunning != "false"
		c.SecureJSON(http.StatusOK, gin.H{
			"indexerRunning": indexerRunningBool,
		})
	})
	router.GET("/settings/ethereum/indexer/status", func(c *gin.Context) {
		ethUUID := database.IndexerGetJobUUID("ethereum")
		ethIndexerStatus := database.IndexerGetJobStatus(ethUUID)
		indexerOnBattery := database.SettingsGetValue("indexerOnBattery")
		indexerOnBatteryBool, _ := strconv.ParseBool(indexerOnBattery)
		isOnBattery := host.IsOnBattery()
		globalIndexerRunning := database.SettingsGetValue("indexerRunning")
		ethIndexerRunning := database.SettingsGetValue("ethereumIndexerRunning")
		if globalIndexerRunning != "true" || ethIndexerRunning == "false" || (isOnBattery && !indexerOnBatteryBool) {
			ethIndexerStatus = "stopped"
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"status": ethIndexerStatus,
		})
	})
	router.GET("/settings/ethereum/indexerProgress", func(c *gin.Context) {
		earliestBlock := _blockchain.GetEarliestBlock("ethereum")
		jobUUID := database.IndexerGetJobUUID("ethereum")
		tailBlock := database.IndexerGetTailBlock(jobUUID)
		headBlock := database.IndexerGetHeadBlock(jobUUID)
		latestBlock, err := _blockchain.GetLatestBlock("ethereum")
		if err != nil || latestBlock.Cmp(big.NewInt(0)) == 0 {
			c.SecureJSON(http.StatusBadRequest, gin.H{"status": "Could not get Ethereum latest block"})
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
			database.SettingsUpdateValue("baseURL", blockchain2.DefaultBlockchainNodes["base"][0])
			database.SettingsUpdateValue("baseThrottle", blockchain2.DefaultBlockchainNodes["base"][1])
			if proxy := blockchain2.GetBaseRPCProxy(); proxy != nil {
				defaultThrottle, _ := strconv.Atoi(blockchain2.DefaultBlockchainNodes["base"][1])
				proxy.UpdateRateLimit(defaultThrottle)
			}
			c.SecureJSON(http.StatusOK, gin.H{"status": "success", "defaultBaseURL": blockchain2.DefaultBlockchainNodes["base"][0], "defaultBaseThrottle": blockchain2.DefaultBlockchainNodes["base"][1]})
			return
		}
		if !security.IsValidURL(payload.BaseURL) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Base URL"})
			return
		}
		database.SettingsUpdateValue("baseURL", payload.BaseURL)
		blockchain2.BaseIndexerStop()
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
		baseURL := database.SettingsGetValue("baseURL")
		if baseURL == "" || baseURL == blockchain2.DefaultBlockchainNodes["base"][0] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Cannot change throttle when using default RPC"})
			return
		}
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
		if proxy := blockchain2.GetBaseRPCProxy(); proxy != nil {
			proxy.UpdateRateLimit(payload.Throttle)
		}
		blockchain2.BaseIndexerStop()
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
			blockchain2.BaseIndexerStop()
			database.IndexerResetJobs("base")
			c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
		}
	})
	router.POST("/settings/algorand/url", func(c *gin.Context) {
		type Payload struct {
			AlgoURL string `json:"algoURL" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Algorand JSON"})
			return
		}
		if payload.AlgoURL == "default" {
			database.SettingsUpdateValue("algoURL", blockchain2.DefaultBlockchainNodes["algorand"][0])
			database.SettingsUpdateValue("algoThrottle", blockchain2.DefaultBlockchainNodes["algorand"][1])
			c.SecureJSON(http.StatusOK, gin.H{"status": "success", "defaultAlgoURL": blockchain2.DefaultBlockchainNodes["algorand"][0], "defaultAlgoThrottle": blockchain2.DefaultBlockchainNodes["algorand"][1]})
			return
		}
		if !security.IsValidURL(payload.AlgoURL) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Algorand URL"})
			return
		}
		database.SettingsUpdateValue("algoURL", payload.AlgoURL)
		blockchain2.AlgoIndexerStop()
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/algorand/throttle", func(c *gin.Context) {
		algoURL := database.SettingsGetValue("algoURL")
		if algoURL == "" || algoURL == blockchain2.DefaultBlockchainNodes["algorand"][0] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Cannot change throttle when using default RPC"})
			return
		}
		type Payload struct {
			Throttle int `json:"throttle" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Algorand throttle value"})
			return
		}
		if !security.IsValidNumberRange(payload.Throttle, 0, 1000) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid throttle range"})
			return
		}
		database.SettingsUpdateValue("algoThrottle", strconv.Itoa(payload.Throttle))
		blockchain2.AlgoIndexerStop()
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/algorand/indexerReset", func(c *gin.Context) {
		type Payload struct {
			IndexerReset bool `json:"indexerReset" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Algorand indexer reset value"})
			return
		}
		if payload.IndexerReset {
			blockchain2.AlgoIndexerStop()
			database.IndexerResetJobs("algorand")
			c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
		}
	})
	router.POST("/settings/base/indexerCatchUp", func(c *gin.Context) {
		type Payload struct {
			IndexerCatchUp string `json:"indexerCatchUp" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
			return
		}
		if payload.IndexerCatchUp != "full" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid indexer catch up value"})
			return
		}
		success, message := blockchain2.BaseIndexerCatchUpAll(database)
		if !success {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"status": message})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/algorand/indexerCatchUp", func(c *gin.Context) {
		type Payload struct {
			IndexerCatchUp string `json:"indexerCatchUp" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
			return
		}
		if payload.IndexerCatchUp != "full" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid indexer catch up value"})
			return
		}
		success, message := blockchain2.AlgoIndexerCatchUpAll(database)
		if !success {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"status": message})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/ethereum/url", func(c *gin.Context) {
		type Payload struct {
			EthereumURL string `json:"ethereumURL" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Ethereum JSON"})
			return
		}
		if payload.EthereumURL == "default" {
			database.SettingsUpdateValue("ethereumURL", blockchain2.DefaultBlockchainNodes["ethereum"][0])
			database.SettingsUpdateValue("ethereumThrottle", blockchain2.DefaultBlockchainNodes["ethereum"][1])
			if proxy := blockchain2.GetEthereumRPCProxy(); proxy != nil {
				defaultThrottle, _ := strconv.Atoi(blockchain2.DefaultBlockchainNodes["ethereum"][1])
				proxy.UpdateRateLimit(defaultThrottle)
			}
			c.SecureJSON(http.StatusOK, gin.H{"status": "success", "defaultEthereumURL": blockchain2.DefaultBlockchainNodes["ethereum"][0], "defaultEthereumThrottle": blockchain2.DefaultBlockchainNodes["ethereum"][1]})
			return
		}
		if !security.IsValidURL(payload.EthereumURL) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Ethereum URL"})
			return
		}
		database.SettingsUpdateValue("ethereumURL", payload.EthereumURL)
		blockchain2.EthereumIndexerStop()
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/ethereum/throttle", func(c *gin.Context) {
		ethereumURL := database.SettingsGetValue("ethereumURL")
		if ethereumURL == "" || ethereumURL == blockchain2.DefaultBlockchainNodes["ethereum"][0] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Cannot change throttle when using default RPC"})
			return
		}
		type Payload struct {
			Throttle int `json:"throttle" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Ethereum throttle value"})
			return
		}
		if !security.IsValidNumberRange(payload.Throttle, 0, 10000) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid throttle range"})
			return
		}
		database.SettingsUpdateValue("ethereumThrottle", strconv.Itoa(payload.Throttle))
		if proxy := blockchain2.GetEthereumRPCProxy(); proxy != nil {
			proxy.UpdateRateLimit(payload.Throttle)
		}
		blockchain2.EthereumIndexerStop()
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/ethereum/indexer/running", func(c *gin.Context) {
		type Payload struct {
			IndexerRunning bool `json:"indexerRunning"`
		}
		var payload Payload
		err := c.ShouldBindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid JSON"})
			return
		}
		if payload.IndexerRunning {
			database.SettingsUpdateValue("ethereumIndexerRunning", "true")
		} else {
			database.SettingsUpdateValue("ethereumIndexerRunning", "false")
			blockchain2.EthereumIndexerStop()
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/ethereum/indexerReset", func(c *gin.Context) {
		type Payload struct {
			IndexerReset bool `json:"indexerReset" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Ethereum indexer reset value"})
			return
		}
		if payload.IndexerReset {
			blockchain2.EthereumIndexerStop()
			database.IndexerResetJobs("ethereum")
			c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
		}
	})
	router.POST("/settings/ethereum/indexerCatchUp", func(c *gin.Context) {
		type Payload struct {
			IndexerCatchUp string `json:"indexerCatchUp" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
			return
		}
		if payload.IndexerCatchUp != "full" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid indexer catch up value"})
			return
		}
		success, message := blockchain2.EthereumIndexerCatchUpAll(database)
		if !success {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"status": message})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
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
		database.SettingsUpdateValue("algoIndexerRunning", "false")
		database.SettingsUpdateValue("baseIndexerRunning", "false")
		database.SettingsUpdateValue("ethereumIndexerRunning", "false")
		blockchain2.AlgoIndexerStop()
		blockchain2.BaseIndexerStop()
		blockchain2.EthereumIndexerStop()
		c.SecureJSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Indexer stopped",
		})
	})
	router.POST("/settings/indexer/start", func(c *gin.Context) {
		database.SettingsUpdateValue("indexerRunning", "true")
		database.SettingsUpdateValue("algoIndexerRunning", "true")
		database.SettingsUpdateValue("baseIndexerRunning", "true")
		database.SettingsUpdateValue("ethereumIndexerRunning", "true")
		c.SecureJSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Indexer started",
		})
	})
	router.POST("/settings/base/indexer/running", func(c *gin.Context) {
		type Payload struct {
			IndexerRunning bool `json:"indexerRunning"`
		}
		var payload Payload
		err := c.ShouldBindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid JSON"})
			return
		}
		if payload.IndexerRunning {
			database.SettingsUpdateValue("baseIndexerRunning", "true")
		} else {
			database.SettingsUpdateValue("baseIndexerRunning", "false")
			blockchain2.BaseIndexerStop()
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/algorand/indexer/running", func(c *gin.Context) {
		type Payload struct {
			IndexerRunning bool `json:"indexerRunning"`
		}
		var payload Payload
		err := c.ShouldBindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid JSON"})
			return
		}
		if payload.IndexerRunning {
			database.SettingsUpdateValue("algoIndexerRunning", "true")
		} else {
			database.SettingsUpdateValue("algoIndexerRunning", "false")
			blockchain2.AlgoIndexerStop()
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
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
		blockchain2.BaseIndexerStop()
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
		pattern := filepath.Join(host.GetDataDir(), "snapshotsyourplacelast*")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Error searching for snapshot files"})
			return
		}
		if len(matches) == 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "No snapshot file found"})
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
		blockchain2.BaseIndexerStop()
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
		if !security.IsValidURL(payload.PinningURL) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid IPFS Pinning URL format"})
			return
		}
		if len(payload.PinningKey) <= 5 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "IPFS Pinning Key must be longer than 5 characters"})
			return
		}
		url := payload.PinningURL
		if !security.IsHttpProtocol(payload.PinningURL) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "IPFS Pinning URL must use HTTP or HTTPS protocol"})
			return
		}
		host.DeleteSecret("ipfsPinningKey")
		host.AddSecret("ipfsPinningKey", security.SanitizeNonPrintable(payload.PinningKey))
		database.SettingsUpdateValue("ipfsPinningURL", url)
		_ = ipfs.IPFSRemoveRemotePinning("ipfsPinning")
		success := ipfs.IPFSAddRemotePinning("ipfsPinning", url, payload.PinningKey)
		success2 := ipfs.IPFSAutoAddRemotePinning("ipfsPinning")
		if success && success2 {
			c.SecureJSON(http.StatusOK, gin.H{"status": "IPFS URL and Key saved"})
		} else {
			c.SecureJSON(http.StatusInternalServerError, gin.H{"status": "Failed to configure IPFS pinning service. Please check your URL and credentials."})
		}
	})
	router.POST("/settings/content/ipfsPinning/remove", func(c *gin.Context) {
		host.DeleteSecret("ipfsPinningKey")
		database.SettingsUpdateValue("ipfsPinningURL", "")
		success := ipfs.IPFSRemoveRemotePinning("ipfsPinning")
		if success {
			c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
		} else {
			c.SecureJSON(http.StatusInternalServerError, gin.H{"status": "failed"})
		}
	})
	router.POST("/settings/content/ipfsGateway", func(c *gin.Context) {
		type Payload struct {
			Gateway string `json:"gateway" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid IPFS Gateway JSON"})
			return
		}
		if payload.Gateway == "default" {
			defaultGateway := network.GetDefaultIPFSGateway()
			database.SettingsUpdateValue("ipfsGateway", defaultGateway)
			ipfs.IPFSSetGateway(defaultGateway)
			c.SecureJSON(http.StatusOK, gin.H{"status": "success", "gateway": defaultGateway})
			return
		}
		gateway := security.SanitizeNonPrintable(payload.Gateway)
		gateway = security.SanitizeHostname(gateway)
		if gateway == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid IPFS Gateway hostname"})
			return
		}
		database.SettingsUpdateValue("ipfsGateway", gateway)
		ipfs.IPFSSetGateway(gateway)
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "gateway": gateway})
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
	router.POST("/settings/services/xcom/crosspost", func(c *gin.Context) {
		type Payload struct {
			Enabled bool `json:"enabled"`
		}
		var payload Payload
		err := c.ShouldBindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid JSON"})
			return
		}
		if payload.Enabled {
			database.SettingsUpdateValue("xcomCrossPostEnabled", "true")
		} else {
			database.SettingsUpdateValue("xcomCrossPostEnabled", "false")
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/services/xcom/feedaggregation", func(c *gin.Context) {
		type Payload struct {
			Enabled bool `json:"enabled"`
		}
		var payload Payload
		err := c.ShouldBindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid JSON"})
			return
		}
		if payload.Enabled {
			database.SettingsUpdateValue("xcomFeedAggregationEnabled", "true")
		} else {
			database.SettingsUpdateValue("xcomFeedAggregationEnabled", "false")
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/settings/services/spotify/clientid", func(c *gin.Context) {
		if services.IsSpotifyClientIDEnvLocked() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Spotify client ID is set via environment variable and cannot be changed from the settings page"})
			return
		}
		type Payload struct {
			ClientID string `json:"clientId" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Spotify client ID JSON"})
			return
		}
		clientID := security.SanitizeNonPrintable(payload.ClientID)
		if !services.IsValidSpotifyClientID(clientID) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Spotify client ID format (expected 32-character hex string)"})
			return
		}
		database.SettingsUpdateValue("spotifyClientId", clientID)
		c.SecureJSON(http.StatusOK, gin.H{"status": "Spotify client ID saved"})
	})
	router.POST("/settings/services/spotify/clientid/remove", func(c *gin.Context) {
		if services.IsSpotifyClientIDEnvLocked() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Spotify client ID is set via environment variable and cannot be removed from the settings page"})
			return
		}
		database.SettingsUpdateValue("spotifyClientId", "")
		c.SecureJSON(http.StatusOK, gin.H{"status": "Spotify client ID removed"})
	})
	router.POST("/settings/services/xcom/credentials", func(c *gin.Context) {
		type Payload struct {
			ApiKey            string `json:"apiKey" required:"true"`
			ApiSecret         string `json:"apiSecret" required:"true"`
			AccessToken       string `json:"accessToken" required:"true"`
			AccessTokenSecret string `json:"accessTokenSecret" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid X.com credentials JSON"})
			return
		}
		if len(payload.ApiKey) < 10 || len(payload.ApiSecret) < 10 || len(payload.AccessToken) < 10 || len(payload.AccessTokenSecret) < 10 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "X.com credentials must be at least 10 characters"})
			return
		}
		apiKey := security.SanitizeNonPrintable(payload.ApiKey)
		apiSecret := security.SanitizeNonPrintable(payload.ApiSecret)
		accessToken := security.SanitizeNonPrintable(payload.AccessToken)
		accessTokenSecret := security.SanitizeNonPrintable(payload.AccessTokenSecret)
		isValid, statusCode := services.XcomTestCredentials(apiKey, apiSecret, accessToken, accessTokenSecret)
		if statusCode == 429 {
			rateLimitExpiry := time.Now().Add(24 * time.Hour)
			database.MetaUpdateValue("xcomRateLimitUntil", rateLimitExpiry.Format(time.RFC3339))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"status":      "X.com API rate limited. Please try again in 24 hours.",
				"rateLimited": true,
			})
			return
		}
		if !isValid {
			database.SettingsUpdateValue("xcomCredentialsValid", "false")
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "X.com credentials are invalid"})
			return
		}
		database.MetaUpdateValue("xcomRateLimitUntil", "")
		host.DeleteSecret("xcomApiSecret")
		host.DeleteSecret("xcomAccessTokenSecret")
		host.AddSecret("xcomApiSecret", apiSecret)
		host.AddSecret("xcomAccessTokenSecret", accessTokenSecret)
		database.SettingsUpdateValue("xcomApiKey", apiKey)
		database.SettingsUpdateValue("xcomAccessToken", accessToken)
		database.SettingsUpdateValue("xcomCredentialsValid", "true")
		services.XcomClearUserCache()
		c.SecureJSON(http.StatusOK, gin.H{"status": "X.com credentials saved"})
	})
	router.POST("/settings/services/xcom/credentials/remove", func(c *gin.Context) {
		host.DeleteSecret("xcomApiSecret")
		host.DeleteSecret("xcomAccessTokenSecret")
		database.SettingsUpdateValue("xcomApiKey", "")
		database.SettingsUpdateValue("xcomAccessToken", "")
		database.SettingsUpdateValue("xcomCredentialsValid", "")
		services.XcomClearUserCache()
		c.SecureJSON(http.StatusOK, gin.H{"status": "X.com credentials removed"})
	})
	router.POST("/settings/services/xcom/scrape/credentials", func(c *gin.Context) {
		type Payload struct {
			Email    string `json:"email" required:"true"`
			Username string `json:"username" required:"true"`
			Password string `json:"password" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid X.com scraping credentials JSON"})
			return
		}
		if len(payload.Email) < 5 || len(payload.Username) < 3 || len(payload.Password) < 5 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "X.com scraping credentials are too short"})
			return
		}
		email := security.SanitizeNonPrintable(payload.Email)
		username := security.SanitizeNonPrintable(payload.Username)
		password := security.SanitizeNonPrintable(payload.Password)
		host.DeleteSecret("xcomScrapePassword")
		host.AddSecret("xcomScrapePassword", password)
		database.SettingsUpdateValue("xcomScrapeEmail", email)
		database.SettingsUpdateValue("xcomScrapeUsername", username)
		c.SecureJSON(http.StatusOK, gin.H{"status": "X.com scraping credentials saved"})
	})
	router.POST("/settings/services/xcom/scrape/credentials/remove", func(c *gin.Context) {
		host.DeleteSecret("xcomScrapePassword")
		database.SettingsUpdateValue("xcomScrapeEmail", "")
		database.SettingsUpdateValue("xcomScrapeUsername", "")
		database.SettingsUpdateValue("xcomScrapeCredentialsValid", "")
		c.SecureJSON(http.StatusOK, gin.H{"status": "X.com scraping credentials removed"})
	})
}
