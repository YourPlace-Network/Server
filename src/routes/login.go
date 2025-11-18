package routes

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/middleware"
	"YourPlace/src/core/security"
	"encoding/hex"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type TxnLoginNonce struct {
	Nonce      string `json:"nonce" binding:"required"`
	Domain     string `json:"domain" binding:"required"`
	Expiration string `json:"expiration" binding:"required"`
}

func LoginRoutes(router *gin.Engine, title string, database *db.Database, cryptoSeed []byte, domain string, installed bool, gateway bool) {
	router.GET("/logout", func(c *gin.Context) {
		cookie, err := c.Request.Cookie("yp_auth")
		if err == nil {
			security.InvalidateCookie(cookie, cryptoSeed, database)
		}
		c.HTML(http.StatusOK, "src/templates/pages/logout.tmpl", gin.H{
			"pageName": "logout",
		})
	})
	router.GET("/login", func(c *gin.Context) {
		csrfToken := middleware.GetCSRFToken(c)
		c.HTML(http.StatusOK, "src/templates/pages/login.tmpl", gin.H{
			"title":       title,
			"pageName":    "login",
			"csrfToken":   csrfToken,
			"gatewayMode": gateway,
		})
	})
	router.GET("/login/check", func(c *gin.Context) {
		cookie, err := c.Request.Cookie("yp_auth")
		if err == nil {
			if security.ValidateCookie(cookie, cryptoSeed, database) {
				c.SecureJSON(http.StatusOK, gin.H{"status": "Logged in"})
				return
			}
		}
		c.SecureJSON(http.StatusUnauthorized, gin.H{"status": "Not logged in"})
		return
	})
	router.GET("/login/nonce", func(c *gin.Context) {
		nonce, err := GenerateLoginNonce(database, domain)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to generate login nonce"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"nonce": nonce})
	})

	router.POST("/login/wallet/base", func(c *gin.Context) {
		type Payload struct {
			Signature string `json:"signature" binding:"required"`
			Payload   string `json:"payload" binding:"required"`
			Address   string `json:"address" binding:"required"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid login json"})
			return
		}
		if !security.IsValidEthAddress(payload.Address) { // validate address format
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid login address"})
			return
		}
		if len(payload.Payload) == 0 || len(payload.Payload) > 1000 { // validate payload length
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid login payload length"})
			return
		}
		if len(payload.Signature) != 132 { // validate signature length 0x + 130 hex characters for ETH signature
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid login signature length"})
			return
		}
		if !strings.HasPrefix(payload.Signature, "0x") { // validate signature prefix
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid login signature prefix"})
			return
		}
		if !security.IsValidEthSignature(payload.Payload, payload.Signature, payload.Address) {
			core.LogDebug("Base wallet login eth signature is invalid")
			if !security.IsValidERC6492Signature(payload.Payload, payload.Signature, payload.Address) {
				core.LogDebug("Base wallet login ERC6492 signature is invalid")
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid login signature"})
				return
			}
		}

		if installed && !gateway { // only do this check if the server is installed and not in gateway mode
			serverOwnerAddress := database.AuthGetServerOwnerAddress()
			if serverOwnerAddress == "" {
				core.LogError("Base wallet login - failed to get server owner address from database")
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Server configuration error"})
				return
			}
			if strings.ToLower(serverOwnerAddress) != strings.ToLower(payload.Address) { // Make sure the person logged in is the person who set up the server
				core.LogDebug("Base wallet login AuthZ failed - invalid server owner address")
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "Invalid login address"})
				return
			}
		}
		authCookie := security.CreateAuthCookie(payload.Address, "base", cryptoSeed, database)
		if authCookie == nil {
			core.LogError("Base wallet login - failed to create auth cookie")
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to create auth cookie"})
			return
		}
		http.SetCookie(c.Writer, authCookie)
		c.SecureJSON(http.StatusOK, gin.H{"status": "Base wallet login success"})
	})
}

func GenerateLoginNonce(database *db.Database, domain string) (string, error) {
	var nonceObj TxnLoginNonce
	nonceObj.Nonce = security.Base64Encode(security.Nonce(128))
	expirationTimestamp := uint64(time.Now().Add(time.Hour).Unix())
	nonceObj.Expiration = strconv.FormatUint(expirationTimestamp, 10)
	nonceObj.Domain = domain
	jsonData, err := json.Marshal(nonceObj)
	if err != nil {
		core.LogError("Failed to marshal login nonce")
		return "", err
	}
	nonceHash := security.Base64Encode(security.Hash(string(jsonData)))
	database.AuthUpdateLoginNonce(nonceObj.Nonce, domain, expirationTimestamp, "YourPlace_Login:"+nonceHash)
	return "YourPlace_Login:" + nonceHash, nil
}
func parseSignature(sigHex string) ([]byte, error) {
	sigHex = strings.TrimPrefix(sigHex, "0x")
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return nil, err
	}
	return sigBytes, nil
}
