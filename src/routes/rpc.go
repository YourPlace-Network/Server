package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/db/blockchain"
	"github.com/gin-gonic/gin"
	"strconv"
)

func RPCRoutes(router *gin.Engine, database *db.Database) {
	// Get RPC URL and rate limit from database settings
	baseURL := database.SettingsGetValue("baseURL")
	baseThrottle := database.SettingsGetValue("baseThrottle")
	rateLimit := 5
	if baseThrottle != "" {
		if parsed, err := strconv.Atoi(baseThrottle); err == nil {
			rateLimit = parsed
		}
	}
	proxy := blockchain.InitBaseRPCProxy(baseURL, rateLimit)
	router.POST("/rpc/base", func(c *gin.Context) {
		proxy.HandleHTTP(c.Writer, c.Request)
	})
}
