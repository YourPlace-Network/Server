package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"net/http"

	"github.com/gin-gonic/gin"
)

func MentalHealthRoutes(router *gin.Engine, title string, database *db.Database, cryptoSeed []byte, gateway bool) {
	router.GET("/mentalHealth", func(c *gin.Context) {
		authenticated := false
		userAddress := ""
		userBlockchain := ""
		authCookie, err := c.Request.Cookie("yp_auth")
		if err == nil && security.ValidateCookie(authCookie, cryptoSeed, database) {
			authenticated = true
			userAddress, _ = security.GetCookieValue(authCookie, cryptoSeed, "address", database)
			userBlockchain, _ = security.GetCookieValue(authCookie, cryptoSeed, "blockchain", database)
		}
		c.HTML(http.StatusOK, "src/templates/pages/mentalHealth.tmpl", gin.H{
			"title":                 title,
			"pageName":              "mentalHealth",
			"isCookieAuthenticated": authenticated,
			"gatewayMode":           gateway,
			"userAddress":           userAddress,
			"userBlockchain":        userBlockchain,
		})
	})
}
