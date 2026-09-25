package routes

import (
	blockchain2 "YourPlace/src/core/blockchain"
	"YourPlace/src/core/db"
	"strconv"

	"github.com/gin-gonic/gin"
)

func RPCRoutes(router *gin.Engine, database *db.Database, blockchain *blockchain2.Blockchain) {
	router.GET("/rpc/algorand/*path", func(c *gin.Context) { blockchain.Algorand.HandleWalletRPC(c.Writer, c.Request) })
	router.POST("/rpc/algorand/v2/transactions", func(c *gin.Context) { blockchain.Algorand.HandleWalletRPC(c.Writer, c.Request) })
	// Base RPC Setup
	baseURL := database.SettingsGetValue("baseURL")
	baseThrottle := database.SettingsGetValue("baseThrottle")
	baseRateLimit := 5
	if baseThrottle != "" {
		if parsed, err := strconv.Atoi(baseThrottle); err == nil {
			baseRateLimit = parsed
		}
	}
	baseProxy := blockchain2.InitBaseRPCProxy(baseURL, baseRateLimit)

	// Ethereum RPC Setup
	ethereumURL := database.SettingsGetValue("ethereumURL")
	ethereumThrottle := database.SettingsGetValue("ethereumThrottle")
	ethereumRateLimit := 5
	if ethereumThrottle != "" {
		if parsed, err := strconv.Atoi(ethereumThrottle); err == nil {
			ethereumRateLimit = parsed
		}
	}
	ethereumProxy := blockchain2.InitEthereumRPCProxy(ethereumURL, ethereumRateLimit)

	router.POST("/rpc/base", func(c *gin.Context) {
		baseProxy.HandleHTTP(c.Writer, c.Request)
	})
	router.POST("/rpc/ethereum", func(c *gin.Context) {
		ethereumProxy.HandleHTTP(c.Writer, c.Request)
	})
}
