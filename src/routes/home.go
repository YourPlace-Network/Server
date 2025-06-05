package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/middleware"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"net/http"
)

func HomeRoutes(router *gin.Engine, title string, favicon []byte, installed bool, database *db.Database, cryptoSeed []byte, gateway bool) {
	router.GET("/", func(c *gin.Context) {
		token := middleware.GetCSRFToken(c)
		authenticated := false
		authCookie, err := c.Request.Cookie("yp_auth")
		if err == nil && security.ValidateCookie(authCookie, cryptoSeed, database) {
			authenticated = true
		}
		c.HTML(http.StatusOK, "src/templates/pages/home.tmpl", gin.H{
			"title":                 title,
			"pageName":              "home",
			"csrfToken":             token,
			"isCookieAuthenticated": authenticated,
			"gatewayMode":           gateway,
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
