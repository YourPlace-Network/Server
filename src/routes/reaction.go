package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ReactionRoutes(router *gin.Engine, database *db.Database) {
	router.GET("/reactions/:blockchain/:txHash", getReactions(database))
	router.GET("/reactions/:blockchain/:txHash/user/:address", getUserReaction(database))
}
func getReactions(database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		txHash := c.Param("txHash")
		if !security.IsValidBlockchain(blockchain) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		if !security.IsValidTxHash(txHash, blockchain) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction hash"})
			return
		}
		var reactions map[string]interface{}
		reactions = database.GetReactionCounts(txHash, blockchain)
		if reactions == nil {
			reactions = map[string]interface{}{
				"likes":    int64(0),
				"dislikes": int64(0),
				"emoji":    map[string]int64{},
			}
		}
		c.JSON(http.StatusOK, reactions)
	}
}
func getUserReaction(database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		txHash := c.Param("txHash")
		address := c.Param("address")
		if !security.IsValidBlockchain(blockchain) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		if !security.IsValidTxHash(txHash, blockchain) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction hash"})
			return
		}
		if !security.IsValidAddress(address, blockchain) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		var userReaction string
		userReaction = database.GetUserReaction(txHash, blockchain, address)
		c.JSON(http.StatusOK, gin.H{"reaction": userReaction})
	}
}
