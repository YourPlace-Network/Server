package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var gatewaySettingsGetAllowed = map[string]bool{
	"/settings":                          true,
	"/settings/algorand/indexer/running": true,
	"/settings/algorand/indexer/status":  true,
	"/settings/algorand/indexerProgress": true,
	"/settings/algorand/throttle":        true,
	"/settings/base/indexer/running":     true,
	"/settings/base/indexer/status":      true,
	"/settings/base/indexerProgress":     true,
	"/settings/base/throttle":            true,
	"/settings/ethereum/indexer/running": true,
	"/settings/ethereum/indexer/status":  true,
	"/settings/ethereum/indexerProgress": true,
	"/settings/ethereum/throttle":        true,
	"/settings/indexer/running":          true,
	"/settings/indexer/status":           true,
	"/settings/server/version":           true,
	"/settings/wallet":                   true,
}

func IsGatewaySettingsGetAllowed(path string) bool {
	path = strings.TrimRight(path, "/")
	if path == "" {
		path = "/"
	}
	return gatewaySettingsGetAllowed[path]
}

func GatewayMiddleware(gateway bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if gateway {
			path := c.Request.URL.Path
			method := c.Request.Method
			if strings.HasPrefix(path, "/settings") {
				if method != "GET" {
					c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{
						"status": "Function disabled in gateway mode",
					})
					return
				}
				if !IsGatewaySettingsGetAllowed(path) {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"status": "Not available in gateway mode",
					})
					return
				}
			}

			blacklistedPathTuples := [][]string{
				{"/files", "POST"},
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
