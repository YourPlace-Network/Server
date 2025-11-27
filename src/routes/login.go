package routes

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/middleware"
	"YourPlace/src/core/security"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spruceid/siwe-go"
)

type TxnLoginNonce struct {
	Nonce      string `json:"nonce" binding:"required"`
	Domain     string `json:"domain" binding:"required"`
	Expiration string `json:"expiration" binding:"required"`
}

func LoginRoutes(router *gin.Engine, title string, database *db.Database, cryptoSeed []byte, domain string, port int, installed bool, gateway bool) {
	var expectedDomain string
	if gateway {
		expectedDomain = domain
	} else {
		expectedDomain = domain + ":" + strconv.Itoa(port)
	}
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
		nonce, err := GenerateLoginNonce(database, expectedDomain)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to generate login nonce"})
			return
		}
		issuedAt := time.Now().UTC().Format(time.RFC3339)
		c.SecureJSON(http.StatusOK, gin.H{
			"nonce":    nonce,
			"domain":   expectedDomain,
			"issuedAt": issuedAt,
		})
	})

	router.POST("/login/wallet/base", func(c *gin.Context) {
		type Payload struct {
			Signature string `json:"signature" binding:"required"`
			Message   string `json:"message" binding:"required"`
			Address   string `json:"address" binding:"required"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid login json"})
			return
		}
		if !security.IsValidEthAddress(payload.Address) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid login address"})
			return
		}
		if !strings.HasPrefix(payload.Signature, "0x") {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid login signature prefix"})
			return
		}
		if len(payload.Signature) < 132 || len(payload.Signature) > 10000 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid login signature length"})
			return
		}
		core.LogDebug("Received SIWE message: " + payload.Message)
		message, err := siwe.ParseMessage(payload.Message)
		if err != nil {
			core.LogError("Failed to parse SIWE message: " + err.Error())
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid SIWE message"})
			return
		}
		if !strings.EqualFold(message.GetAddress().Hex(), payload.Address) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Address mismatch"})
			return
		}
		if message.GetDomain() != expectedDomain {
			core.LogDebug("Domain mismatch: got " + message.GetDomain() + ", expected " + expectedDomain)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid domain"})
			return
		}
		nonceHash := message.GetNonce()
		nonce := database.AuthGetLoginNonceByHash(nonceHash)
		if nonce == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid or expired nonce"})
			return
		}
		database.AuthDeleteLoginNonce(nonce)
		_, err = message.Verify(payload.Signature, nil, nil, nil)
		if err != nil {
			core.LogDebug("EIP-191 verification failed, trying ERC-1271 for smart wallet: " + err.Error())
			// Check if this is an EIP-6492 signature (undeployed smart wallet)
			isERC6492 := security.IsERC6492Signature(payload.Signature)
			isDeployed := security.IsContractDeployed(payload.Address, database)
			core.LogDebug("Signature analysis: EIP-6492=" + strconv.FormatBool(isERC6492) + ", ContractDeployed=" + strconv.FormatBool(isDeployed))
			if isERC6492 && !isDeployed {
				core.LogDebug("Detected undeployed smart wallet with EIP-6492 signature - wallet deployment required")
				c.AbortWithStatusJSON(http.StatusPreconditionRequired, gin.H{
					"status": "wallet_not_deployed",
					"error":  "Smart wallet not deployed. Please send a transaction to deploy your wallet first.",
				})
				return
			}
			if !isDeployed {
				core.LogDebug("Contract not deployed at address, cannot verify ERC-1271 signature")
				c.AbortWithStatusJSON(http.StatusPreconditionRequired, gin.H{
					"status": "wallet_not_deployed",
					"error":  "Smart wallet not deployed. Please send a transaction to deploy your wallet first.",
				})
				return
			}
			if !security.ValidateERC1271Signature(payload.Message, payload.Signature, payload.Address, database) {
				core.LogDebug("Both EIP-191 and ERC-1271 signature verification failed")
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid signature"})
				return
			}
			core.LogDebug("ERC-1271 smart wallet signature validated successfully")
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
	nonceObj.Nonce = hex.EncodeToString([]byte(security.Nonce(128)))
	expirationTimestamp := uint64(time.Now().Add(time.Hour).Unix())
	nonceObj.Expiration = strconv.FormatUint(expirationTimestamp, 10)
	nonceObj.Domain = domain
	jsonData, err := json.Marshal(nonceObj)
	if err != nil {
		core.LogError("Failed to marshal login nonce")
		return "", err
	}
	nonceHash := hex.EncodeToString(security.HashBytes(jsonData))
	database.AuthUpdateLoginNonce(nonceObj.Nonce, domain, expirationTimestamp, nonceHash)
	return nonceHash, nil
}
func parseSignature(sigHex string) ([]byte, error) {
	sigHex = strings.TrimPrefix(sigHex, "0x")
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return nil, err
	}
	return sigBytes, nil
}
