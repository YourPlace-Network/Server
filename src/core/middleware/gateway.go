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

			blacklistedPathTuples := [][]string{
				{"/files", "POST"},
				{"/settings", "POST"},
				{"/setup", "POST"}, {"/setup", "GET"},
				{"/notifications", "POST"},
			}

			for _, blacklistedPath := range blacklistedPathTuples {
				if strings.HasPrefix(path, blacklistedPath[0]) && method == blacklistedPath[1] {
					c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{
						"status": "Function disabled in gateway mode",
					})
					return
				}
			}
		}
		c.Next()
	}
}
