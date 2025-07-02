package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"net/http"
)

func MarketplaceRoutes(router *gin.Engine, database *db.Database) {
	router.GET("/marketplace/listings", func(c *gin.Context) {
		listings := database.MarketplaceGetListings()
		c.SecureJSON(http.StatusOK, gin.H{"listings": listings})
	})
	router.GET("/marketplace/listings/:blockchain/:address", func(c *gin.Context) {
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
		listings := database.MarketplaceGetUserListings(address, blockchain)
		c.SecureJSON(http.StatusOK, gin.H{"listings": listings})
	})
	router.GET("/marketplace/listing/:id", func(c *gin.Context) {
		id := c.Param("id")
		if id == "" || !security.IsValidUUID(id) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid listing ID"})
			return
		}
		listing := database.MarketplaceGetListing(id)
		if listing == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Listing not found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"listing": listing})
	})
	router.GET("/marketplace/listing/:id/offers", func(c *gin.Context) {
		id := c.Param("id")
		if id == "" || !security.IsValidUUID(id) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid listing ID"})
			return
		}
		offers := database.MarketplaceGetOffers(id)
		c.SecureJSON(http.StatusOK, gin.H{"offers": offers})
	})
	router.GET("/marketplace/offer/:id", func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offer ID"})
			return
		}
		offer := database.MarketplaceGetOffer(id)
		if offer == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Offer not found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"offer": offer})
	})
	router.GET("/marketplace/transactions/:blockchain/:address", func(c *gin.Context) {
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
		transactions := database.MarketplaceGetUserTransactions(address, blockchain)
		c.SecureJSON(http.StatusOK, gin.H{"transactions": transactions})
	})
	router.GET("/marketplace/transaction/:id", func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction ID"})
			return
		}
		transaction := database.MarketplaceGetTransaction(id)
		if transaction == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"transaction": transaction})
	})
}
