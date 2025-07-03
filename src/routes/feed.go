package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func FeedRoutes(router *gin.Engine, database *db.Database) {
	router.GET("/feed/:blockchain/:address", func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchain) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchain) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address"})
			return
		}
		// Get limit parameter with default value of 50
		limitStr := c.DefaultQuery("limit", "50")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 200 {
			limit = 50
		}
		posts := database.GetFollowersFeed(address, blockchain, limit)
		c.SecureJSON(http.StatusOK, gin.H{"posts": posts})
	})
}
