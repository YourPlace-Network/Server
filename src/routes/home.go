package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/middleware"
	"YourPlace/src/core/security"
	"net/http"

	"github.com/gin-gonic/gin"
)

func HomeRoutes(router *gin.Engine, title string, favicon []byte, installed bool, database *db.Database, cryptoSeed []byte, gateway bool) {
	router.GET("/", func(c *gin.Context) {
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
		c.HTML(http.StatusOK, "src/templates/pages/home.tmpl", gin.H{
			"title":                 title,
			"pageName":              "home",
			"csrfToken":             token,
			"ipfsGateway":           ipfsGateway,
			"isCookieAuthenticated": authenticated,
			"gatewayMode":           gateway,
			"userAddress":           userAddress,
			"userBlockchain":        userBlockchain,
		})
	})
	router.GET("/favicon.ico", func(c *gin.Context) {
		c.Data(http.StatusOK, "image/x-icon", favicon)
	})
	router.GET("/ping", func(c *gin.Context) {
		if installed {
			c.SecureJSON(http.StatusOK, gin.H{"status": "pong"})
		} else {
			c.SecureJSON(http.StatusServiceUnavailable, gin.H{"status": "Not installed"})
		}
	})
}
