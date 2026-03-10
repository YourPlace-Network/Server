package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func CommentRoutes(router *gin.Engine, database *db.Database) {
	router.GET("/comments/:blockchain/:txHash", getComments(database))
	router.GET("/comments/:blockchain/:txHash/count", getCommentCount(database))
	router.GET("/comments/:blockchain/:txHash/user/:address", getUserHasCommented(database))
}
func getComments(database *db.Database) gin.HandlerFunc {
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
		limitStr := c.DefaultQuery("limit", "50")
		offsetStr := c.DefaultQuery("offset", "0")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 100 { // Anti-DoS: max 100 comments per request
			limit = 50
		}
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 || offset > 1000 { // Anti-DoS: max offset 1000 to prevent deep pagination abuse
			offset = 0
		}
		comments := database.GetComments(txHash, blockchain, limit, offset)
		if comments == nil {
			comments = []map[string]interface{}{}
		}
		c.JSON(http.StatusOK, gin.H{"comments": comments})
	}
}
func getCommentCount(database *db.Database) gin.HandlerFunc {
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
		count := database.GetCommentCount(txHash, blockchain)
		c.JSON(http.StatusOK, gin.H{"count": count})
	}
}
func getUserHasCommented(database *db.Database) gin.HandlerFunc {
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
		if !security.IsValidAddressAnyChain(address) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		hasCommented := database.HasUserCommented(txHash, blockchain, address)
		c.JSON(http.StatusOK, gin.H{"hasCommented": hasCommented})
	}
}
