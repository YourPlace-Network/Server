package routes

import (
	"YourPlace/src/core"
	"YourPlace/src/core/blockchain"
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/middleware"
	"YourPlace/src/core/security"
	"YourPlace/src/core/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Notification struct {
	UID         string `json:"uid"`
	Type        string `json:"type"`
	Message     string `json:"message"`
	Dismissable bool   `json:"dismissable"`
}
type UserNotification struct {
	ID             string `json:"id"`
	FromAddress    string `json:"fromAddress"`
	FromBlockchain string `json:"fromBlockchain"`
	ReactionType   string `json:"reactionType"`
	TargetTxHash   string `json:"targetTxHash"`
	Timestamp      string `json:"timestamp"`
	Type           string `json:"type"`
}

func NotificationRoutes(router *gin.Engine, title string, database *db.Database, cryptoSeed []byte, gateway bool) {
	router.GET("/notification", func(c *gin.Context) {
		notifications := GetAllNotifications(database, gateway)
		c.SecureJSON(http.StatusOK, gin.H{"notifications": notifications})
	})
	router.GET("/notification/:uid", func(c *gin.Context) {
		uid := c.Param("uid")
		if uid == "" {
			c.SecureJSON(http.StatusOK, gin.H{"error": "Missing notification UID"})
			return
		}
		notifications := GetAllNotifications(database, gateway)
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
	router.GET("/notifications", func(c *gin.Context) {
		token := middleware.GetCSRFToken(c)
		authenticated := false
		userAddress := ""
		userBlockchain := ""
		authCookie, err := c.Request.Cookie("yp_auth")
		if err == nil && security.ValidateCookie(authCookie, cryptoSeed, database) {
			authenticated = true
			userAddress, _ = security.GetCookieValue(authCookie, cryptoSeed, "address", database)
			userBlockchain, _ = security.GetCookieValue(authCookie, cryptoSeed, "blockchain", database)
		}
		c.HTML(http.StatusOK, "src/templates/pages/notifications.tmpl", gin.H{
			"title":                 title,
			"pageName":              "notifications",
			"csrfToken":             token,
			"isCookieAuthenticated": authenticated,
			"gatewayMode":           gateway,
			"userAddress":           userAddress,
			"userBlockchain":        userBlockchain,
		})
	})
	router.GET("/notifications/count", func(c *gin.Context) {
		userAddress, userBlockchain := getAuthenticatedUser(c, cryptoSeed, database)
		if userAddress == "" {
			c.SecureJSON(http.StatusOK, gin.H{"count": 0})
			return
		}
		lastSeen := database.UserNotificationGetSeen(userAddress, userBlockchain)
		count := database.UserNotificationGetCount(userAddress, userBlockchain, lastSeen)
		c.SecureJSON(http.StatusOK, gin.H{"count": count})
	})
	router.GET("/notifications/data", func(c *gin.Context) {
		userAddress, userBlockchain := getAuthenticatedUser(c, cryptoSeed, database)
		if userAddress == "" {
			c.SecureJSON(http.StatusOK, gin.H{"notifications": []UserNotification{}})
			return
		}
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "25"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		if limit > 100 {
			limit = 100
		}
		if offset > 1000 {
			offset = 1000
		}
		dbNotifs := database.UserNotificationGet(userAddress, userBlockchain, limit, offset)
		var notifications []UserNotification
		for _, n := range dbNotifs {
			notifications = append(notifications, UserNotification{
				ID:             n["id"],
				FromAddress:    n["fromAddress"],
				FromBlockchain: n["fromBlockchain"],
				ReactionType:   n["reactionType"],
				TargetTxHash:   n["targetTxHash"],
				Timestamp:      n["timestamp"],
				Type:           n["type"],
			})
		}
		if notifications == nil {
			notifications = []UserNotification{}
		}
		c.SecureJSON(http.StatusOK, gin.H{"notifications": notifications})
	})
	router.POST("/notifications/clear", func(c *gin.Context) {
		userAddress, userBlockchain := getAuthenticatedUser(c, cryptoSeed, database)
		if userAddress == "" {
			c.SecureJSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			return
		}
		database.UserNotificationClearAll(userAddress, userBlockchain)
		c.SecureJSON(http.StatusOK, gin.H{"status": "cleared"})
	})
	router.POST("/notifications/dismiss/:id", func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "Missing notification ID"})
			return
		}
		userAddress, _ := getAuthenticatedUser(c, cryptoSeed, database)
		if userAddress == "" {
			c.SecureJSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			return
		}
		database.UserNotificationDismiss(id)
		c.SecureJSON(http.StatusOK, gin.H{"status": "dismissed"})
	})
	router.POST("/notifications/seen", func(c *gin.Context) {
		userAddress, userBlockchain := getAuthenticatedUser(c, cryptoSeed, database)
		if userAddress == "" {
			c.SecureJSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			return
		}
		database.UserNotificationUpdateSeen(userAddress, userBlockchain, core.GetTimestamp())
		c.SecureJSON(http.StatusOK, gin.H{"status": "updated"})
	})
}
func getAuthenticatedUser(c *gin.Context, cryptoSeed []byte, database *db.Database) (string, string) {
	authCookie, err := c.Request.Cookie("yp_auth")
	if err != nil || !security.ValidateCookie(authCookie, cryptoSeed, database) {
		return "", ""
	}
	address, _ := security.GetCookieValue(authCookie, cryptoSeed, "address", database)
	chain, _ := security.GetCookieValue(authCookie, cryptoSeed, "blockchain", database)
	return address, chain
}
func GetAllNotifications(database *db.Database, gateway bool) []Notification {
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
	if !gateway {
		systemNotifications := getSystemNotifications(database)
		notifications = append(notifications, systemNotifications...)
	}
	return notifications
}
func getSystemNotifications(database *db.Database) []Notification {
	var notifications []Notification
	baseURL := database.SettingsGetValue("baseURL")
	if baseURL == blockchain.DefaultBlockchainNodes["base"][0] {
		notifications = append(notifications, Notification{
			UID:         "rpc_slow",
			Type:        "system",
			Message:     "You are using a blockchain node which is causing slow performance. <a href=\"/settings#base\" class=\"toastLink\">Click here</a> to set your own blockchain RPC node",
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
