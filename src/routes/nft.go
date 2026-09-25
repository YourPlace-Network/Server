package routes

import (
	"YourPlace/src/core/blockchain"
	"YourPlace/src/core/db"
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func NFTRoutes(router *gin.Engine, database *db.Database, minter *blockchain.Minter) {
	router.GET("/profile/nft/collections", func(c *gin.Context) {
		collections, err := database.NFTCollections(c.Request.Context())
		if err != nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		c.JSON(http.StatusOK, gin.H{"collections": collections})
	})
	operator := func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if !minter.IsOperator(c.GetString("blockchain"), c.GetString("accountAddress")) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Operator access required"})
			return
		}
		c.Next()
	}
	router.GET("/settings/content/nft", operator, func(c *gin.Context) {
		config, err := database.NFTConfig(c.Request.Context())
		if err != nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		counts, err := database.NFTCounts(c.Request.Context())
		if err != nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		c.JSON(http.StatusOK, gin.H{"config": config, "counts": counts, "ready": minter.Readiness(), "registration": blockchain.NFTRegistered()})
	})
	router.POST("/settings/content/nft", operator, func(c *gin.Context) {
		var config db.NFTConfig
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
		if c.ShouldBindJSON(&config) != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		if err := minter.SaveConfig(ctx, config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "Saved"})
	})
	router.POST("/settings/content/nft/template", operator, func(c *gin.Context) {
		var source db.NFTSource
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2048)
		if c.ShouldBindJSON(&source) != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
		defer cancel()
		if err := minter.SelectTemplate(ctx, source); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": err.Error()})
			return
		}
		config, err := database.NFTConfig(ctx)
		if err != nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "Welcome NFT updated", "template": config.Template})
	})
	router.GET("/settings/content/nft/grants", operator, func(c *gin.Context) {
		offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
		if err != nil || offset < 0 || offset > 1000000 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		grants, err := database.NFTGrants(c.Request.Context(), "", offset, 25, false)
		if err != nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		for index := range grants {
			sanitizeNFTGrant(&grants[index])
		}
		c.JSON(http.StatusOK, gin.H{"grants": grants})
	})
	router.POST("/settings/content/nft/retry", operator, func(c *gin.Context) {
		var payload struct {
			ID string `json:"id"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
		if c.ShouldBindJSON(&payload) != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if _, err := uuid.Parse(payload.ID); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if err := minter.Retry(c.Request.Context(), payload.ID); err != nil {
			c.JSON(http.StatusConflict, gin.H{"status": "Grant cannot be retried"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "Queued"})
	})
	welcome := router.Group("/nft/welcome")
	welcome.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		identity, err := blockchain.WalletNFTIdentity(c.GetString("blockchain"), c.GetString("accountAddress"))
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set("nftIdentity", identity)
		c.Next()
	})
	welcome.GET("", func(c *gin.Context) {
		identity := c.GetString("nftIdentity")
		profile, err := database.NFTProfile(c.Request.Context(), identity)
		if err != nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		grants, err := database.NFTGrants(c.Request.Context(), identity, 0, 1, false)
		if err != nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		for index := range grants {
			sanitizeNFTGrant(&grants[index])
		}
		c.JSON(http.StatusOK, gin.H{"profile": profile, "grants": grants})
	})
	welcome.POST("/prepare", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		response, err := minter.Prepare(ctx, c.GetString("nftIdentity"))
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"status": err.Error()})
			return
		}
		c.JSON(http.StatusOK, response)
	})
	welcome.POST("/accept", func(c *gin.Context) {
		var payload struct {
			Signed []byte `json:"signed"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
		if c.ShouldBindJSON(&payload) != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		if err := minter.Accept(ctx, c.GetString("nftIdentity"), payload.Signed); err != nil {
			c.JSON(http.StatusConflict, gin.H{"status": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "Accepted"})
	})
}
func sanitizeNFTGrant(grant *db.NFTGrant) {
	grant.ApprovalReceived = len(grant.RecipientSignature) > 0
	grant.Acceptance, grant.RecipientSignature = nil, nil
	if grant.Transaction != nil {
		grant.Transaction.Raw = nil
	}
}
