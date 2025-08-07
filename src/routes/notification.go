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

type NotificationObject struct {
	UID         string `json:"uid"`
	Type        string `json:"type"`
	Message     string `json:"message"`
	Dismissable bool   `json:"dismissable"`
}

func NotificationRoutes(router *gin.Engine, database *db.Database) {
	router.GET("/notification", func(c *gin.Context) {
		notifications := GetAllNotifications(database)
		c.SecureJSON(http.StatusOK, gin.H{"notifications": notifications})
	})
	router.GET("/notification/:uid", func(c *gin.Context) {
		uid := c.Param("uid")
		if uid == "" {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "Missing notification UID"})
			return
		}
		notifications := GetAllNotifications(database)
		for _, notification := range notifications {
			if notification.UID == uid {
				c.SecureJSON(http.StatusOK, notification)
				return
			}
		}
		c.SecureJSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
	})
	router.POST("/notification/dismiss/:uid", func(c *gin.Context) {
		uid := c.Param("uid")
		if uid == "" {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "Missing notification UID"})
			return
		}
		database.NotificationDismiss(uid)
		c.SecureJSON(http.StatusOK, gin.H{"status": "dismissed"})
	})
}

func GetAllNotifications(database *db.Database) []NotificationObject {
	var notifications []NotificationObject
	dbNotifications := database.NotificationGetActive()
	for _, dbNotif := range dbNotifications {
		notifications = append(notifications, NotificationObject{
			UID:         dbNotif["uid"],
			Type:        "user",
			Message:     dbNotif["message"],
			Dismissable: true,
		})
	}
	systemNotifications := getSystemNotifications(database)
	notifications = append(notifications, systemNotifications...)
	return notifications
}
func getSystemNotifications(database *db.Database) []NotificationObject {
	var notifications []NotificationObject
	baseURL := database.SettingsGetValue("baseURL")
	if baseURL == blockchain.DefaultBlockchainNodes["base"][0] {
		notifications = append(notifications, NotificationObject{
			UID:         "system_rpc_slow",
			Type:        "system",
			Message:     "You are using a blockchain node which is causing slow performance. <a href=\"/settings#collapseBase\" class=\"toastLink\">Click here</a> to set your own blockchain RPC node",
			Dismissable: false,
		})
	}
	serverVersion := host.GetServerVersion()
	latestVersion := services.GetLatestServerVersion()
	if serverVersion != "" && latestVersion != "" && security.IsVersionGreater(serverVersion, latestVersion) {
		notifications = append(notifications, NotificationObject{
			UID:         "system_update_available",
			Type:        "system",
			Message:     "A new update is available. <a href=\"https://yourplace.network/download\" class=\"toastLink\" target=\"_blank\">Click here</a> to download",
			Dismissable: false,
		})
	}
	return notifications
}
