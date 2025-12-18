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
	router.GET("/post/:blockchain/:txHash", func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		txHash := c.Param("txHash")
		if !security.IsValidBlockchain(blockchain) {
			c.Redirect(http.StatusFound, "/")
			return
		}
		if !security.IsValidTxHash(txHash, blockchain) {
			c.Redirect(http.StatusFound, "/")
			return
		}
		title := GetTitle()
		gateway := IsGatewayMode()
		token := GetCSRFToken(c)
		authenticated := IsCookieAuthenticated(c)
		c.HTML(http.StatusOK, "src/templates/pages/post.tmpl", gin.H{
			"title":                 title,
			"pageName":              "post",
			"csrfToken":             token,
			"gatewayMode":           gateway,
			"isCookieAuthenticated": authenticated,
			"blockchain":            blockchain,
			"txHash":                txHash,
		})
	})
	router.GET("/post/data/:blockchain/:txHash", func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		txHash := c.Param("txHash")
		if !security.IsValidBlockchain(blockchain) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid blockchain"})
			return
		}
		if !security.IsValidTxHash(txHash, blockchain) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction hash"})
			return
		}
		var post map[string]interface{}
		var reactions map[string]interface{}
		var commentCount int64
		if blockchain == "algorand" {
			post = database.GetAlgorandPost(txHash, blockchain)
			reactions = database.GetAlgorandReactionCounts(txHash, blockchain)
			commentCount = database.GetAlgorandCommentCount(txHash, blockchain)
		} else {
			post = database.GetPost(txHash, blockchain)
			reactions = database.GetReactionCounts(txHash, blockchain)
			commentCount = database.GetCommentCount(txHash, blockchain)
		}
		if post == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
			return
		}
		post["reactions"] = reactions
		post["commentCount"] = commentCount
		c.SecureJSON(http.StatusOK, gin.H{"post": post})
	})
}
