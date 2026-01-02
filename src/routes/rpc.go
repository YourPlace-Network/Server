package routes

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/db/blockchain"
	"strconv"

	"github.com/gin-gonic/gin"
)

func RPCRoutes(router *gin.Engine, database *db.Database) {
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
		core.LogDebug("RPC route /rpc/base hit")
		proxy.HandleHTTP(c.Writer, c.Request)
	})
}
