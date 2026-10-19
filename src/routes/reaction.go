package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ReactionRoutes(router *gin.Engine, database *db.Database) {
	router.GET("/reactions/:blockchain/:txHash", getReactions(database))
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
		address := c.Query("address")
		if address != "" && security.IsValidAddressAnyChain(address) {
			userReactions := database.GetUserReactions(txHash, blockchain, address)
			reactions["userReaction"] = userReactions["likeDislike"]
			reactions["userEmojiReaction"] = userReactions["emoji"]
		}
		c.JSON(http.StatusOK, reactions)
	}
}
