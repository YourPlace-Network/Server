package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/middleware"
	"YourPlace/src/core/security"
	"net/http"

	"github.com/gin-gonic/gin"
)

func undergroundHomeHandler(title string, database *db.Database, cryptoSeed []byte, gateway bool) gin.HandlerFunc {
	return func(c *gin.Context) {
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
		c.HTML(http.StatusOK, "src/templates/pages/underground.tmpl", gin.H{
			"title":                 title,
			"pageName":              "underground",
			"csrfToken":             token,
			"ipfsGateway":           ipfsGateway,
			"isCookieAuthenticated": authenticated,
			"gatewayMode":           gateway,
			"userAddress":           userAddress,
			"userBlockchain":        userBlockchain,
		})
	}
}
