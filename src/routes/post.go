package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"net/http"
)

func PostRoutes(router *gin.Engine, database *db.Database) {
	router.GET("/posts/:blockchain/:address", func(c *gin.Context) {
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
		posts := database.ProfileGetPosts(address, blockchain)
		c.SecureJSON(http.StatusOK, gin.H{"posts": posts})
	})
}
