package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/middleware"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"net/http"
)

func FAQRoutes(router *gin.Engine, title string, database *db.Database, cryptoSeed []byte, gateway bool) {
	router.GET("/faq", func(c *gin.Context) {
		authenticated := false
		userAddress := ""
		userBlockchain := ""
		authCookie, err := c.Request.Cookie("yp_auth")
		if err == nil && security.ValidateCookie(authCookie, cryptoSeed, database) {
			authenticated = true
			userAddress, _ = security.GetCookieValue(authCookie, cryptoSeed, "address", database)
			userBlockchain, _ = security.GetCookieValue(authCookie, cryptoSeed, "blockchain", database)
		}
		token := middleware.GetCSRFToken(c)
		c.HTML(http.StatusOK, "src/templates/pages/faq.tmpl", gin.H{
			"title":                 title,
			"pageName":              "faq",
			"csrfToken":             token,
			"isCookieAuthenticated": authenticated,
			"gatewayMode":           gateway,
			"userAddress":           userAddress,
			"userBlockchain":        userBlockchain,
		})
	})
}
