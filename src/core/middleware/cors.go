package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func CORSMiddleware(port int) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Cross-Origin-Resource-Policy", "cross-origin")
		c.Header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "3600")

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		// Handle HEAD requests for all routes
		if c.Request.Method == "HEAD" {
			c.Header("Content-Type", "application/json")
			c.Status(http.StatusOK)
			c.Abort()
			return
		}
		c.Next()
	}
}
