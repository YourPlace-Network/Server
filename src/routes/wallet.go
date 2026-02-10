package routes

import (
	"YourPlace/src/core"
	"YourPlace/src/core/blockchain"
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/security"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func WalletRoutes(router *gin.Engine, title string, database *db.Database, cryptoSeed []byte, gateway bool) {
	if gateway {
		return
	}
	router.GET("/wallet/default/:blockchain", func(c *gin.Context) {
		blockchainName := c.Param("blockchain")
		wallet, err := database.WalletGetDefault(blockchainName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "No default wallet found",
			})
			return
		}
		c.JSON(http.StatusOK, wallet)
	})
	router.GET("/wallet/all", func(c *gin.Context) {
		wallets, err := database.WalletGetAll()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve wallets",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"wallets": wallets,
		})
	})
	router.POST("/wallet/create/eth", func(c *gin.Context) {
		var requestData struct {
			IsDefault bool `json:"isDefault"`
		}
		if err := c.ShouldBindJSON(&requestData); err != nil {
			requestData.IsDefault = false
		}
		walletData, err := blockchain.CreateEthereumWallet()
		if err != nil {
			core.LogError("Failed to create Ethereum wallet: " + err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create wallet",
			})
			return
		}
		if len(cryptoSeed) < 32 {
			core.LogError("Crypto seed too short for wallet encryption")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to encrypt wallet",
			})
			return
		}
		encryptedPrivateKey, err := security.EncryptString(string(cryptoSeed), walletData.PrivateKey)
		if err != nil {
			core.LogError("Failed to encrypt private key: " + err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to encrypt wallet",
			})
			return
		}
		secretName := fmt.Sprintf("YourPlace_Wallet_%s_%s", walletData.Blockchain, walletData.PublicKey)
		host.AddSecret(secretName, walletData.PrivateKey)
		err = database.WalletStore(walletData.PublicKey, walletData.Blockchain, walletData.Address, encryptedPrivateKey, requestData.IsDefault)
		if err != nil {
			core.LogError("Failed to store wallet: " + err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to store wallet",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"publicKey":  walletData.PublicKey,
			"blockchain": walletData.Blockchain,
			"address":    walletData.Address,
			"isDefault":  requestData.IsDefault,
		})
	})
	router.POST("/wallet/create/algo", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{
			"status": "not implemented",
		})
	})
	router.POST("/wallet/create/solana", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{
			"status": "not implemented",
		})
	})
	router.POST("/wallet/setDefault", func(c *gin.Context) {
		var requestData struct {
			PublicKey  string `json:"publicKey" binding:"required"`
			Blockchain string `json:"blockchain" binding:"required"`
		}
		if err := c.ShouldBindJSON(&requestData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request data",
			})
			return
		}
		err := database.WalletSetDefault(requestData.PublicKey, requestData.Blockchain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to set default wallet",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	})
}
