package security

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

func Headers(port int) gin.HandlerFunc {
	//portStr := strconv.FormatInt(int64(port), 10)
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if c.Request.Method == "OPTIONS" { // Handle preflight requests
			if origin == "" || strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Max-Age", "86400") // 24 hours
				c.Status(http.StatusOK)
				return
			}
		}

		// Handle regular requests
		if origin == "" || strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
			c.Header("Access-Control-Allow-Credentials", "true")
		} else {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{})
			return
		}

		// Global security headers
		//ipfsApiPort := strconv.Itoa(port + 1)
		ipfsGatewayPort := strconv.Itoa(port + 2)
		c.Header("Content-Security-Policy",
			"default-src 'none'; "+
				"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://*.bridge.walletconnect.org https://www.googletagmanager.com; "+ // unsafe-inline and unsafe-eval are due to bootstrap
				"img-src 'self' https://* data: blob: http://localhost:"+ipfsGatewayPort+" http://*.ipfs.localhost:"+ipfsGatewayPort+"; "+ // wildcard all HTTPS connections to allow for 3rd party image embeds
				"style-src 'self' https://fonts.googleapis.com 'unsafe-inline'; "+
				"media-src 'self' data: https://*; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"connect-src 'self' data: https://* wss://*:* http://localhost:"+ipfsGatewayPort+" http://*.ipfs.localhost:"+ipfsGatewayPort+"; "+ // this must wildcard all TLS connections to allow for P2P traffic
				"frame-src https://*; ")
		c.Header("X-Content-Options", "nosniff")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		//c.Header("X-Frame-Options", fmt.Sprintf("sameorigin"))
		return
	}
}
