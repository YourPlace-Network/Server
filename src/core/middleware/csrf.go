package middleware

import (
	"YourPlace/src/core"
	"YourPlace/src/core/security"
	"crypto/subtle"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	CSRFTokenLength = 32
	CSRFTokenHeader = "X-CSRF-Token"
	CSRFCookieName  = "csrf_token"
	CSRFFormField   = "csrf_token"
)

type CSRFConfig struct {
	CryptoSeed []byte
	MaxAge     int // seconds
}

func CSRFMiddleware(config CSRFConfig) gin.HandlerFunc {
	config.MaxAge = 3600 // 1-hour default

	return func(c *gin.Context) {
		// Skip CSRF for safe methods and excluded paths
		if isCSRFExcluded(c) {
			c.Next()
			return
		}
		method := c.Request.Method
		// For safe methods, generate and set token
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			existingToken, err := c.Cookie(CSRFCookieName)
			if err == nil && existingToken != "" && isValidTokenFormat(existingToken, config.CryptoSeed, config.MaxAge) {
				// Reuse existing valid token
				c.Set("csrfToken", existingToken)
				c.Next()
				return
			}
			token, err := generateCSRFToken(config.CryptoSeed)
			if err != nil {
				core.LogError("Failed to generate CSRF token: " + err.Error())
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			setCSRFCookie(c, token, config)
			c.Set("csrfToken", token)
			c.Next()
			return
		}
		// For unsafe methods, validate token
		if method == "POST" || method == "PUT" || method == "DELETE" || method == "PATCH" {
			if !validateCSRFToken(c, config) {
				core.LogDebug("CSRF validation failed for " + c.Request.URL.Path)
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
		c.Next()
	}
}

func generateCSRFToken(seed []byte) (string, error) {
	// Generate random bytes
	tokenBytes := security.RandomBytes(CSRFTokenLength)
	// Create timestamp
	timestamp := time.Now().Unix()
	// Combine token with timestamp and sign with seed
	payload := fmt.Sprintf("%s:%d", security.Base64EncodeBytes(tokenBytes), timestamp)
	signature := security.HMAC([]byte(payload), seed)
	// Return signed token
	return fmt.Sprintf("%s.%s", payload, security.Base64Encode(signature)), nil
}
func validateCSRFToken(c *gin.Context, config CSRFConfig) bool {
	// Get token from multiple sources
	token := getCSRFTokenFromRequest(c)
	if token == "" {
		return false
	}
	// Validate token format and signature
	if !isValidTokenFormat(token, config.CryptoSeed, config.MaxAge) {
		return false
	}
	// Validate against cookie token
	cookieToken, err := c.Cookie(CSRFCookieName)
	if err != nil || cookieToken == "" {
		return false
	}
	// Constant-time comparison
	return subtle.ConstantTimeCompare([]byte(token), []byte(cookieToken)) == 1
}
func getCSRFTokenFromRequest(c *gin.Context) string {
	// Check header first
	token := c.GetHeader(CSRFTokenHeader)
	if token != "" {
		return token
	}
	// Check form field
	token = c.PostForm(CSRFFormField)
	if token != "" {
		return token
	}
	// Check JSON body
	var jsonBody map[string]interface{}
	if c.ShouldBindJSON(&jsonBody) == nil {
		if csrfToken, exists := jsonBody[CSRFFormField]; exists {
			if tokenStr, ok := csrfToken.(string); ok {
				return tokenStr
			}
		}
	}
	return ""
}
func isValidTokenFormat(token string, seed []byte, maxAge int) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	payload := parts[0]
	providedSig := security.Base64Decode(parts[1])
	if providedSig == "" || len(providedSig) == 0 {
		return false // Invalid base64 encoding
	}
	// Verify signature
	expectedSig := security.HMAC([]byte(payload), seed)
	if subtle.ConstantTimeCompare([]byte(providedSig), []byte(expectedSig)) != 1 {
		return false
	}
	// Check expiration
	payloadParts := strings.Split(payload, ":")
	if len(payloadParts) != 2 {
		return false
	}
	timestamp, err := strconv.ParseInt(payloadParts[1], 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-timestamp > int64(maxAge) {
		return false // Token has expired
	}
	return true // Token is valid
}
func setCSRFCookie(c *gin.Context, token string, config CSRFConfig) {
	// Extract domain from request host
	host := c.Request.Host
	domain := ""
	if host == "localhost:42424" || host == "127.0.0.1:42424" {
		domain = "localhost"
	}

	c.SetCookie(
		CSRFCookieName,
		token,
		config.MaxAge,
		"/",
		domain,
		false, // Secure: false for HTTP localhost
		true,  // HttpOnly
	)
}
func isCSRFExcluded(c *gin.Context) bool {
	path := c.Request.URL.Path
	method := c.Request.Method
	// Exclude safe methods for certain paths
	safeMethods := []string{"GET", "HEAD", "OPTIONS"}
	for _, safeMethod := range safeMethods {
		if method == safeMethod {
			return false // Don't exclude, we need to set tokens
		}
	}
	// Exclude specific API endpoints that handle their own validation
	excludedPaths := []string{
		"/login/wallet/base",
		"/rpc/base",
		"/rpc/ethereum",
		"/settings/database/exportSnapshot",
		"/settings/database/importSnapshot",
	}
	for _, excludedPath := range excludedPaths {
		if path == excludedPath {
			return true
		}
	}
	// Exclude static assets
	if strings.HasPrefix(path, "/static/") {
		return true
	}
	return false
}

// Helper function to get CSRF token for templates
func GetCSRFToken(c *gin.Context) string {
	if token, exists := c.Get("csrfToken"); exists {
		if tokenStr, ok := token.(string); ok {
			return tokenStr
		}
	}
	return ""
}
