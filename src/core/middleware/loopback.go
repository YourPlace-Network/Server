package middleware

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type ExceptionRule struct {
	Method string // HTTP method (GET, POST, etc.)
	Path   string // Exact path to match
}

func LoopbackMiddleware(port int, gateway bool, gatewayMintEnabled bool) gin.HandlerFunc {
	// This filter enforces clients to originate from 127.0.0.1 for certain API paths to be included for localhost enforcement. Your path must start with one of these prefixes
	includedPaths := []string{
		"/settings",   // Settings endpoints
		"/setup",      // Setup endpoints
		"/debug",      // Debug endpoints
		"/ipfs",       // IPFS endpoints
		"/health",     // Health check endpoints
		"/test",       // Test endpoints
		"/service/ai", // AI service endpoints
		"/files",      // File management endpoints
		"/wallet",     // Wallet management endpoints
	}
	return func(c *gin.Context) {
		requestPath := c.Request.RequestURI
		if gateway && gatewayMintEnabled && c.Request.Method == "POST" && (strings.HasPrefix(requestPath, "/files/avatar/sign") || strings.HasPrefix(requestPath, "/files/nft/sign")) {
			c.Next()
			return
		}
		if gateway && c.Request.Method == "GET" { // In gateway mode, allow specific settings endpoints
			if strings.HasPrefix(requestPath, "/settings/algorand/url") ||
				strings.HasPrefix(requestPath, "/settings/base/url") ||
				strings.HasPrefix(requestPath, "/settings/services/algorand") {
				c.Next()
				return
			}
		}
		// Check if this request matches an exception rule
		validHosts := map[string]bool{
			"localhost":              true,
			"localhost6":             true,
			"localhost.localdomain":  true,
			"1.0.0.127.in-addr.arpa": true,
			"127.0.0.1/8":            true,
			"127.0.0.1":              true,
			"127.0.0.0":              true,
			"127.0.0.2":              true,
			"127.255.255.255":        true,
			"::1":                    true,
			"::1/128":                true,
			"0:0:0:0:0:0:0:1":        true,
			"[::1]":                  true,
			"1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.ip6.arpa": true,
		}
		// Get the request path
		requestPath = c.Request.RequestURI
		for _, path := range includedPaths {
			if strings.HasPrefix(requestPath, path) {
				// Get actual IP
				ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
				if err != nil {
					c.AbortWithStatus(http.StatusBadRequest)
					return
				}
				// IP address checks
				if ip != "127.0.0.1" && ip != "::1" {
					c.AbortWithStatus(http.StatusBadRequest)
					return
				}
				clientIP := c.ClientIP()
				if !validHosts[clientIP] {
					c.AbortWithStatus(http.StatusBadRequest)
					return
				}
				break // Found a match and passed validation
			}
		}
		c.Next()
	}
}

func LoopbackRedirectMiddleware(port int) gin.HandlerFunc { // This filter enforces clients to originate from localhost
	return func(c *gin.Context) {
		host := c.Request.Host
		if strings.HasPrefix(host, "127.0.0.1") {
			scheme := "http"
			if c.Request.TLS != nil {
				scheme = "https"
			}
			newURL := url.URL{
				Scheme:   scheme,
				Host:     "localhost:" + strconv.Itoa(port),
				Path:     c.Request.URL.Path,
				RawQuery: c.Request.URL.RawQuery,
			}
			c.Redirect(http.StatusMovedPermanently, newURL.String())
			c.Abort()
			return
		}
	}
}
