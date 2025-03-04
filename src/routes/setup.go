package routes

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"net/http"
	"os"
	"strconv"
)

func SetupRoutes(router *gin.Engine, database *db.Database, title string, favicon []byte, port int) {
	defaultUploadDirectory := host.GetDataDir() + "upload" + host.PathSeparator

	router.GET("/setup", func(c *gin.Context) {
		installedDate := database.MetaGetValue("installedDate")
		if len(installedDate) > 5 {
			c.Request.URL.Path = "/"
			router.HandleContext(c)
			return
		}
		uploadDirectory := database.SettingsGetValue("uploadDirectory")
		if uploadDirectory != "" {
			uploadDirectory = defaultUploadDirectory
		}
		token := csrf.Token(c.Request)
		c.HTML(http.StatusOK, "src/templates/pages/setup.tmpl", gin.H{
			"title":                  title,
			"defaultUploadDirectory": defaultUploadDirectory,
			"csrfToken":              token,
			"pageName":               "setup",
		})
	})
	router.GET("/setup/installed", func(c *gin.Context) {
		isInstalled := IsInstalled(database)
		if isInstalled {
			c.JSON(http.StatusOK, gin.H{"status": "installed"})
		} else {
			c.JSON(http.StatusProcessing, gin.H{"status": "not installed"})
		}
	})

	router.POST("/setup", func(c *gin.Context) {
		type Payload struct {
			Address         string `json:"address" binding:"required"`
			Birthdate       string `json:"birthdate" binding:"required"`
			UploadDirectory string `json:"uploadDirectory" binding:"required"`
			Wallet          string `json:"wallet" binding:"required"`
			Blockchain      string `json:"blockchain" binding:"required"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "invalid setup json"})
			return
		}
		birthdateInt, _ := strconv.Atoi(payload.Birthdate)
		if !security.IsValidBirthDate(int64(birthdateInt)) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "invalid birthdate"})
			return
		}
		if !security.IsValidWallet(payload.Wallet) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "invalid wallet"})
			return
		}
		if !security.IsValidBlockchain(payload.Blockchain) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "invalid blockchain"})
			return
		}
		if !security.IsValidAddress(payload.Address, payload.Blockchain) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "invalid wallet address"})
			return
		}
		_uploadDirectory := security.SanitizePathTraversal(payload.UploadDirectory)
		_uploadDirectory = security.SanitizeCommandInjection(_uploadDirectory)
		if !host.DoesExist(_uploadDirectory) {
			host.CreateFolder(_uploadDirectory)
		}
		core.LogDebug("--- Setting Up YourPlace ---")
		core.LogDebug("Address: " + payload.Address)
		core.LogDebug("Birthdate: " + payload.Birthdate)
		core.LogDebug("UploadDirectory: " + _uploadDirectory)
		core.LogDebug("Wallet: " + payload.Wallet)
		core.LogDebug("Blockchain: " + payload.Blockchain)
		database.MetaUpdateValue("accountAddress", payload.Address)
		database.SettingsUpdateValue("uploadDirectory", _uploadDirectory)
		err = os.WriteFile(host.GetInstallDir()+"favicon.ico", favicon, 0644)
		if err != nil {
			core.LogWarn("Could not write the ico file: " + err.Error())
		}
		host.CreateShortcut(port)
		if host.InstallHelper() == false { // Install the YourPlace helper
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "error installing helper"})
			return
		}
		installedDate := strconv.FormatUint(core.GetTimestamp(), 10)
		database.MetaUpdateValue("installedDate", installedDate)

		response, err := host.HelperCall("restart") // Restart the YourPlace server
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "error restarting server via the helper: " + response})
			return
		}

		// This response likely won't even be sent, because the server gets restarted. But that's fine because the front-end doesn't expect a response
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
}

// ---------- Installer Functions ----------- //
func IsInstalled(database *db.Database) bool {
	dbPath := host.GetDataDir() + host.PathSeparator + "yourplace.sqlite.db"
	if !host.DoesExist(dbPath) {
		core.LogDebug("Could not find DB path: " + dbPath)
		return false
	}
	installedValue := database.MetaGetValue("installedDate")
	if len(installedValue) == 0 {
		return false
	}
	return true
}
