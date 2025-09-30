package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/db/blockchain"
	"YourPlace/src/core/host"
	"YourPlace/src/core/security"
	"YourPlace/src/core/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Types of notifications: system, user

type Notification struct {
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
func GetAllNotifications(database *db.Database) []Notification {
	var notifications []Notification
	dbNotifications := database.NotificationGetActive()
	for _, dbNotif := range dbNotifications {
		notifications = append(notifications, Notification{
			UID:         dbNotif["uid"],
			Type:        dbNotif["type"],
			Message:     dbNotif["message"],
			Dismissable: dbNotif["dismissable"] == "1",
		})
	}
	systemNotifications := getSystemNotifications(database)
	notifications = append(notifications, systemNotifications...)
	return notifications
}
func getSystemNotifications(database *db.Database) []Notification {
	var notifications []Notification
	baseURL := database.SettingsGetValue("baseURL")
	if baseURL == blockchain.DefaultBlockchainNodes["base"][0] {
		notifications = append(notifications, Notification{
			UID:         "rpc_slow",
			Type:        "system",
			Message:     "You are using a blockchain node which is causing slow performance. <a href=\"/settings#collapseBase\" class=\"toastLink\">Click here</a> to set your own blockchain RPC node",
			Dismissable: true,
		})
	}
	serverVersion := host.GetServerVersion()
	latestVersion := services.GetLatestServerVersion()
	if serverVersion != "" && latestVersion != "" && security.IsVersionGreater(serverVersion, latestVersion) {
		notifications = append(notifications, Notification{
			UID:         "update_available",
			Type:        "system",
			Message:     "A new update is available. <a href=\"https://yourplace.network/download\" class=\"toastLink\" target=\"_blank\">Click here</a> to download",
			Dismissable: true,
		})
	}
	return notifications
}
func getUserNotifications(database *db.Database, chain string, address string) []Notification {
	var notifications []Notification
	return notifications
}
