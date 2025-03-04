package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/db/blockchain"
	"YourPlace/src/core/host"
	"YourPlace/src/core/security"
	"YourPlace/src/core/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

func NotificationRoutes(router *gin.Engine, database *db.Database) {
	router.GET("/notification/toasts", func(c *gin.Context) {
		rpcToasts := GetDefaultRPCNodeNotification(database)
		updateToasts := GetServerUpdateNotification()
		toasts := append(rpcToasts, updateToasts...)
		c.SecureJSON(http.StatusOK, gin.H{"toasts": toasts})
	})
}

func GetDefaultRPCNodeNotification(database *db.Database) []string {
	baseURL := database.SettingsGetValue("baseURL")
	if baseURL == blockchain.DefaultBlockchainNodes["base"][0] {
		return []string{"You are using a blockchain node which is causing slow performance. <a href=\"/settings#collapseBase\" class=\"toastLink\">Click here</a> to set your own blockchain RPC node"}
	}
	return nil
}
func GetServerUpdateNotification() []string {
	serverVersion := host.GetServerVersion()
	if serverVersion == "" {
		return nil
	}
	latestVersion := services.GetLatestServerVersion()
	if latestVersion == "" {
		return nil
	}
	if security.IsVersionGreater(serverVersion, latestVersion) {
		return []string{"A new update is available. <a href=\"https://yourplace.network/download\" class=\"toastLink\" target=\"_blank\">Click here</a> to download"}
	}
	return nil
}
