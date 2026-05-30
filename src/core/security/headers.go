package security

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
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
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")

		// Global security headers
		//ipfsApiPort := strconv.Itoa(port + 1)
		ipfsGatewayPort := strconv.Itoa(port + 2)
		c.Header("Content-Security-Policy",
			"default-src 'none'; "+
				"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://*.bridge.walletconnect.org https://*.spotifycdn.com https://open.spotify.com https://sdk.scdn.co https://www.googletagmanager.com https://www.youtube-nocookie.com; "+ // unsafe-inline and unsafe-eval are due to bootstrap
				"img-src 'self' https://* data: blob: http://localhost:"+ipfsGatewayPort+" http://*.ipfs.localhost:"+ipfsGatewayPort+"; "+ // wildcard all HTTPS connections to allow for 3rd party image embeds
				"style-src 'self' https://fonts.googleapis.com 'unsafe-inline'; "+
				"media-src 'self' data: blob: https://* http://localhost:"+ipfsGatewayPort+" http://*.ipfs.localhost:"+ipfsGatewayPort+"; "+ // blob: is required by Spotify Web Playback SDK for audio chunks
				"font-src 'self' https://fonts.gstatic.com data:; "+
				"connect-src 'self' data: blob: https://*:* wss://*:* http://localhost:"+ipfsGatewayPort+" http://*.ipfs.localhost:"+ipfsGatewayPort+"; "+ // blob: is required for editor attachment previews and wildcard TLS connections allow P2P traffic
				"frame-src https://*; "+
				"frame-ancestors 'self' https://app.yourplace.network https://yourplace.network; "+
				"worker-src 'self' blob:; "+ // Spotify Web Playback SDK spawns blob: workers for audio decoding
				"base-uri 'self'; "+
				"form-action 'self'; "+
				"object-src 'none'; ")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}
}
