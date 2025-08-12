package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func GatewayMiddleware(gateway bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if gateway {
			path := c.Request.URL.Path
			method := c.Request.Method

			// Blacklist API endpoints when in gateway mode
			blacklistedPaths := []string{
				"/files/",
			}

			for _, blacklistedPath := range blacklistedPaths {
				if strings.HasPrefix(path, blacklistedPath) && method == "POST" {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"status": "Function disabled in gateway mode",
					})
					return
				}
			}
		}
		c.Next()
	}
}
