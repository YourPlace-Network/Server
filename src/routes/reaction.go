package routes

import (
	"YourPlace/src/core/security"
	"net/http"
	"github.com/gin-gonic/gin"
)

func ReactionSetupRoutes(router *gin.Engine) {
	router.GET("/reactions/:blockchain/:txHash", getReactions)
	router.GET("/reactions/:blockchain/:txHash/user/:address", getUserReaction)
}
func getReactions(c *gin.Context) {
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
	if blockchain == "algorand" {
		reactions = _Database.GetAlgorandReactionCounts(txHash, blockchain)
	} else {
		reactions = _Database.GetReactionCounts(txHash, blockchain)
	}
	if reactions == nil {
		reactions = map[string]interface{}{
			"likes":    int64(0),
			"dislikes": int64(0),
			"emoji":    map[string]int64{},
		}
	}
	c.JSON(http.StatusOK, reactions)
}
func getUserReaction(c *gin.Context) {
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
	if blockchain == "algorand" {
		userReaction = _Database.GetAlgorandUserReaction(txHash, blockchain, address)
	} else {
		userReaction = _Database.GetUserReaction(txHash, blockchain, address)
	}
	c.JSON(http.StatusOK, gin.H{"reaction": userReaction})
}
