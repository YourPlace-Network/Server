package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"net/http"
)

func FAQRoutes(router *gin.Engine, title string, database *db.Database, cryptoSeed []byte, gateway bool) {
	router.GET("/faq", func(c *gin.Context) {
		authenticated := false
		authCookie, err := c.Request.Cookie("yp_auth")
		if err == nil && security.ValidateCookie(authCookie, cryptoSeed, database) {
			authenticated = true
		}
		token := csrf.Token(c.Request)
		c.HTML(http.StatusOK, "src/templates/pages/faq.tmpl", gin.H{
			"title":                 title,
			"pageName":              "faq",
			"csrfToken":             token,
			"isCookieAuthenticated": authenticated,
			"gatewayMode":           gateway,
		})
	})
}
