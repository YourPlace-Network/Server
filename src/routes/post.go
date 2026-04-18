package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/middleware"
	"YourPlace/src/core/security"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func PostRoutes(router *gin.Engine, database *db.Database, title string, cryptoSeed []byte) {
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
		limitStr := c.DefaultQuery("limit", "21")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 21 {
			limit = 21
		}
		offsetStr := c.DefaultQuery("offset", "0")
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 || offset > 10000 {
			offset = 0
		}
		totalCount := database.ProfileGetPostCount(address, blockchain)
		posts := database.ProfileGetPosts(address, blockchain, limit, offset)
		c.SecureJSON(http.StatusOK, gin.H{"posts": posts, "totalCount": totalCount})
	})
	router.GET("/posts/:blockchain/:address/comments", func(c *gin.Context) {
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
		limitStr := c.DefaultQuery("limit", "21")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 21 {
			limit = 21
		}
		offsetStr := c.DefaultQuery("offset", "0")
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 || offset > 10000 {
			offset = 0
		}
		totalCount := database.ProfileGetCommentCount(address, blockchain)
		comments := database.ProfileGetComments(address, blockchain, limit, offset)
		c.SecureJSON(http.StatusOK, gin.H{"posts": comments, "totalCount": totalCount})
	})
	router.GET("/post/:blockchain/:txHash", func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		txHash := c.Param("txHash")
		if !security.IsValidBlockchain(blockchain) {
			c.Redirect(http.StatusNotFound, "/404")
			return
		}
		if !security.IsValidTxHash(txHash, blockchain) {
			c.Redirect(http.StatusNotFound, "/404")
			return
		}
		authenticated := false
		userAddress := ""
		userBlockchain := ""
		authCookie, err := c.Request.Cookie("yp_auth")
		if err == nil && security.ValidateCookie(authCookie, cryptoSeed, database) {
			authenticated = true
			userAddress, _ = security.GetCookieValue(authCookie, cryptoSeed, "address", database)
			userBlockchain, _ = security.GetCookieValue(authCookie, cryptoSeed, "blockchain", database)
		}
		pageTitle := "Post | " + title
		gateway := host.IsGatewayMode()
		token := middleware.GetCSRFToken(c)
		c.HTML(http.StatusOK, "src/templates/pages/post.tmpl", gin.H{
			"title":                 pageTitle,
			"pageName":              "post",
			"csrfToken":             token,
			"gatewayMode":           gateway,
			"isCookieAuthenticated": authenticated,
			"blockchain":            blockchain,
			"txHash":                txHash,
			"userAddress":           userAddress,
			"userBlockchain":        userBlockchain,
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
		post = database.GetPost(txHash, blockchain)
		reactions = database.GetReactionCounts(txHash, blockchain)
		commentCount = database.GetCommentCount(txHash, blockchain)
		if post == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
			return
		}
		var emojiCount int64
		if emojiMap, ok := reactions["emoji"].(map[string]int64); ok {
			for _, count := range emojiMap {
				emojiCount += count
			}
		}
		post["reactions"] = reactions
		post["commentCount"] = commentCount
		post["emojiCount"] = emojiCount
		userAddress := c.Query("address")
		if userAddress != "" && security.IsValidAddressAnyChain(userAddress) {
			userReactions := database.GetUserReactions(txHash, blockchain, userAddress)
			if userReactions["likeDislike"] != "" {
				post["userReaction"] = userReactions["likeDislike"]
			}
			if userReactions["emoji"] != "" {
				post["userEmojiReaction"] = userReactions["emoji"]
			}
		}
		c.SecureJSON(http.StatusOK, gin.H{"post": post})
	})
}
