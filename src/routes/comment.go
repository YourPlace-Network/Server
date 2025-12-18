package routes

import (
	"YourPlace/src/core/security"
	"net/http"
	"strconv"
	"github.com/gin-gonic/gin"
)

func CommentSetupRoutes(router *gin.Engine) {
	router.GET("/comments/:blockchain/:txHash", getComments)
	router.GET("/comments/:blockchain/:txHash/count", getCommentCount)
}
func getComments(c *gin.Context) {
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
	if err != nil || limit < 1 || limit > 100 {
		limit = 50
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}
	var comments []map[string]interface{}
	if blockchain == "algorand" {
		comments = _Database.GetAlgorandComments(txHash, blockchain, limit, offset)
	} else {
		comments = _Database.GetComments(txHash, blockchain, limit, offset)
	}
	if comments == nil {
		comments = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"comments": comments})
}
func getCommentCount(c *gin.Context) {
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
	var count int64
	if blockchain == "algorand" {
		count = _Database.GetAlgorandCommentCount(txHash, blockchain)
	} else {
		count = _Database.GetCommentCount(txHash, blockchain)
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}
