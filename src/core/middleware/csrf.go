package middleware

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
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
	Database   *db.Database
	Secure     bool
	SameSite   http.SameSite
	MaxAge     int // seconds
}

func CSRFMiddleware(config CSRFConfig) gin.HandlerFunc {
	if config.MaxAge == 0 {
		config.MaxAge = 3600 // 1 hour default
	}
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
			if err == nil && existingToken != "" && isValidTokenFormat(existingToken, config.CryptoSeed) {
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
	payload := fmt.Sprintf("%s:%d", base64.URLEncoding.EncodeToString(tokenBytes), timestamp)
	signature := security.HMAC([]byte(payload), seed)
	// Return signed token
	return fmt.Sprintf("%s.%s", payload, base64.URLEncoding.EncodeToString([]byte(signature))), nil
}
func validateCSRFToken(c *gin.Context, config CSRFConfig) bool {
	// Get token from multiple sources
	token := getCSRFTokenFromRequest(c)
	if token == "" {
		return false
	}
	// Validate token format and signature
	if !isValidTokenFormat(token, config.CryptoSeed) {
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
func isValidTokenFormat(token string, seed []byte) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	payload := parts[0]
	providedSig, err := base64.URLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	// Verify signature
	expectedSig := security.HMAC([]byte(payload), seed)
	return subtle.ConstantTimeCompare(providedSig, []byte(expectedSig)) == 1
}
func setCSRFCookie(c *gin.Context, token string, config CSRFConfig) {
	c.SetCookie(
		CSRFCookieName,
		token,
		config.MaxAge,
		"/",
		"localhost",
		config.Secure,
		true, // HttpOnly
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
