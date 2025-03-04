package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"net/http"
)

func MentalHealthRoutes(router *gin.Engine, title string, database *db.Database, cryptoSeed []byte) {
	router.GET("/mentalHealth", func(c *gin.Context) {
		authenticated := false
		authCookie, err := c.Request.Cookie("yp_auth")
		if err == nil && security.ValidateCookie(authCookie, cryptoSeed, database) {
			authenticated = true
		}
		c.HTML(http.StatusOK, "src/templates/pages/mentalHealth.tmpl", gin.H{
			"title":                 title,
			"pageName":              "mentalHealth",
			"isCookieAuthenticated": authenticated,
		})
	})
}
