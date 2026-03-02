package routes

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/middleware"
	"YourPlace/src/core/security"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/algorand/go-algorand-sdk/v2/encoding/msgpack"
	_algotypes "github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/gin-gonic/gin"
	"github.com/spruceid/siwe-go"
)

type TxnLoginNonce struct {
	Nonce      string `json:"nonce" binding:"required"`
	Domain     string `json:"domain" binding:"required"`
	Expiration string `json:"expiration" binding:"required"`
}
type SiwaMessageParsed struct {
	Address   string
	Domain    string
	Nonce     string
	Statement string
	URI       string
	Version   string
	ChainID   string
	IssuedAt  string
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
		database.OnchainMN("base", payload.Address, "", 0)
		c.SecureJSON(http.StatusOK, gin.H{"status": "Base wallet login success"})
	})
	router.POST("/login/wallet/ethereum", func(c *gin.Context) {
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
		core.LogDebug("Received Ethereum SIWE message: " + payload.Message)
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
			core.LogDebug("Ethereum EIP-191 verification failed: " + err.Error())
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid signature"})
			return
		}
		authCookie := security.CreateAuthCookie(payload.Address, "ethereum", cryptoSeed, database)
		if authCookie == nil {
			core.LogError("Ethereum wallet login - failed to create auth cookie")
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to create auth cookie"})
			return
		}
		http.SetCookie(c.Writer, authCookie)
		database.OnchainMN("ethereum", payload.Address, "", 0)
		c.SecureJSON(http.StatusOK, gin.H{"status": "Ethereum wallet login success"})
	})
	router.POST("/login/wallet/base/local", func(c *gin.Context) {
		type Payload struct {
			Address   string `json:"address" binding:"required"`
			Message   string `json:"message" binding:"required"`
			Signature string `json:"signature" binding:"required"`
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
		core.LogDebug("Received local wallet SIWE message: " + payload.Message)
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
			core.LogDebug("Local wallet EIP-191 verification failed: " + err.Error())
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid signature"})
			return
		}
		authCookie := security.CreateAuthCookie(payload.Address, "base", cryptoSeed, database)
		if authCookie == nil {
			core.LogError("Local wallet login - failed to create auth cookie")
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to create auth cookie"})
			return
		}
		http.SetCookie(c.Writer, authCookie)
		database.OnchainMN("base", payload.Address, "", 0)
		c.SecureJSON(http.StatusOK, gin.H{"status": "Local wallet login success"})
	})
	router.POST("/login/wallet/pera", func(c *gin.Context) {
		type Payload struct {
			Address            string `json:"address" binding:"required"`
			EncodedTransaction string `json:"encodedTransaction" binding:"required"`
			Message            string `json:"message" binding:"required"`
			Signature          string `json:"signature" binding:"required"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid login json"})
			return
		}
		if !security.IsValidAlgoAddress(payload.Address) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid Algorand address"})
			return
		}
		core.LogDebug("Received SIWA message: " + payload.Message)
		siwaMessage, err := ParseSiwaMessage(payload.Message)
		if err != nil {
			core.LogDebug("Failed to parse SIWA message: " + err.Error())
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid SIWA message"})
			return
		}
		if !strings.EqualFold(siwaMessage.Address, payload.Address) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Address mismatch"})
			return
		}
		if siwaMessage.Domain != expectedDomain {
			core.LogDebug("Domain mismatch: got " + siwaMessage.Domain + ", expected " + expectedDomain)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid domain"})
			return
		}
		nonceHash := siwaMessage.Nonce
		nonce := database.AuthGetLoginNonceByHash(nonceHash)
		if nonce == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid or expired nonce"})
			return
		}
		database.AuthDeleteLoginNonce(nonce)
		if !VerifySiwaTransaction(payload.EncodedTransaction, payload.Signature, payload.Address, payload.Message) {
			core.LogDebug("SIWA transaction verification failed")
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid signature"})
			return
		}
		core.LogDebug("SIWA signature verified successfully for address: " + payload.Address)
		authCookie := security.CreateAuthCookie(payload.Address, "algorand", cryptoSeed, database)
		if authCookie == nil {
			core.LogError("Pera wallet login - failed to create auth cookie")
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to create auth cookie"})
			return
		}
		http.SetCookie(c.Writer, authCookie)
		database.OnchainMN("algorand", payload.Address, "", 0)
		c.SecureJSON(http.StatusOK, gin.H{"status": "Pera wallet login success"})
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
func ParseSiwaMessage(message string) (*SiwaMessageParsed, error) {
	parsed := &SiwaMessageParsed{}
	lines := strings.Split(message, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if i == 0 && strings.HasSuffix(line, " wants you to sign in with your Algorand account:") {
			parsed.Domain = strings.TrimSuffix(line, " wants you to sign in with your Algorand account:")
			continue
		}
		if security.IsValidAlgoAddress(line) && parsed.Address == "" {
			parsed.Address = line
			continue
		}
		if strings.HasPrefix(line, "URI: ") {
			parsed.URI = strings.TrimPrefix(line, "URI: ")
		} else if strings.HasPrefix(line, "Version: ") {
			parsed.Version = strings.TrimPrefix(line, "Version: ")
		} else if strings.HasPrefix(line, "Chain ID: ") {
			parsed.ChainID = strings.TrimPrefix(line, "Chain ID: ")
		} else if strings.HasPrefix(line, "Nonce: ") {
			parsed.Nonce = strings.TrimPrefix(line, "Nonce: ")
		} else if strings.HasPrefix(line, "Issued At: ") {
			parsed.IssuedAt = strings.TrimPrefix(line, "Issued At: ")
		} else if line != "" && parsed.Statement == "" && !strings.Contains(line, ": ") {
			if !regexp.MustCompile(`^[A-Z2-7]{58}$`).MatchString(line) {
				parsed.Statement = line
			}
		}
	}
	if parsed.Domain == "" || parsed.Address == "" || parsed.Nonce == "" {
		return nil, core.LogErrorReturn("Invalid SIWA message format")
	}
	return parsed, nil
}
func VerifySiwaTransaction(encodedTransaction string, signatureBase64 string, address string, expectedMessage string) bool {
	txnBytes, err := base64.StdEncoding.DecodeString(encodedTransaction)
	if err != nil {
		core.LogDebug("Failed to decode transaction from base64: " + err.Error())
		return false
	}
	var signedTxn _algotypes.SignedTxn
	err = msgpack.Decode(txnBytes, &signedTxn)
	if err != nil {
		core.LogDebug("Failed to decode signed transaction: " + err.Error())
		return false
	}
	txnSignature := base64.StdEncoding.EncodeToString(signedTxn.Sig[:])
	if txnSignature != signatureBase64 {
		core.LogDebug("Signature mismatch: txn signature doesn't match provided signature")
		return false
	}
	txn := signedTxn.Txn
	if txn.Sender.String() != address {
		core.LogDebug("Sender mismatch: expected " + address + ", got " + txn.Sender.String())
		return false
	}
	noteStr := string(txn.Note)
	if noteStr != expectedMessage {
		core.LogDebug("Note mismatch: transaction note doesn't match SIWA message")
		return false
	}
	algoAddr, err := _algotypes.DecodeAddress(address)
	if err != nil {
		core.LogDebug("Failed to decode Algorand address: " + err.Error())
		return false
	}
	publicKey := ed25519.PublicKey(algoAddr[:])
	txnBytesToSign := append([]byte("TX"), msgpack.Encode(txn)...)
	signatureBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		core.LogDebug("Failed to decode signature from base64: " + err.Error())
		return false
	}
	return ed25519.Verify(publicKey, txnBytesToSign, signatureBytes)
}
