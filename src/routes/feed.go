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
		// Get limit parameter with default value of 26 (25 + 1 for pagination check)
		limitStr := c.DefaultQuery("limit", "26")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 26 {
			limit = 26
		}
		// Get offset parameter with default value of 0
		offsetStr := c.DefaultQuery("offset", "0")
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			offset = 0
		}
		posts := database.GetFollowersFeed(address, blockchain, limit, offset)
		c.SecureJSON(http.StatusOK, gin.H{"posts": posts})
	})
}
