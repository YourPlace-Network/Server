package middleware

import (
	"YourPlace/src/core"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		origin := c.Request.Header.Get("Origin")

		// Skip CORS checks for static assets (fonts, images, CSS, JS)
		if strings.HasPrefix(path, "/static/") {
			core.LogDebug("CORS: Static path " + path + " with origin: " + origin)
			if origin != "" {
				c.Header("Access-Control-Allow-Origin", origin)
			}
			c.Header("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
			c.Header("Cross-Origin-Resource-Policy", "cross-origin")
			c.Header("X-Debug-CORS", "static-asset-path")
		} else {
			core.LogDebug("CORS: Non-static path " + path + " with origin: " + origin)
			// Only allow localhost origins for API/page requests
			allowedOrigins := []string{
				"http://localhost:42424",
				"https://localhost:42424",
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
				core.LogDebug("CORS: Blocking non-static path " + path + " with disallowed origin: " + origin)
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Header("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
			c.Header("Cross-Origin-Resource-Policy", "same-origin")
		}
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
