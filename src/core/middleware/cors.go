package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		currentOrigin := scheme + "://" + c.Request.Host
		// Only allow localhost origins for local development and same-origin requests
		allowedOrigins := []string{
			"http://localhost:42424",
			"https://localhost:42424",
			currentOrigin,
		}

		originAllowed := false
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				c.Header("Access-Control-Allow-Origin", origin)
				originAllowed = true
				break
			}
		}

		if !originAllowed && origin != "" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Header("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
		c.Header("Cross-Origin-Resource-Policy", "same-origin")
		c.Header("Access-Control-Allow-Methods", "GET, POST, HEAD, OPTIONS, PUT, DELETE")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-CSRF-Token, X-Requested-With, Cache-Control, Content-Length")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "3600")
		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		if c.Request.Method == "HEAD" {
			c.Header("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.Next()
	}
}
