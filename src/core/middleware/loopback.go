package middleware

import (
	"YourPlace/src/core"
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func LoopbackMiddleware(port int) gin.HandlerFunc { // This filter enforces clients to originate from 127.0.0.1 for certain API paths
	return func(c *gin.Context) {
		validHosts := map[string]bool{
			"localhost": true,
			"127.0.0.1": true,
			"::1":       true,
			"[::1]":     true,
		}
		// Get actual IP
		ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		// Get the request path
		requestPath := c.Request.RequestURI
		// To be included for localhost enforcement, your path must start with one of these prefixes
		paths := []string{"/settings", "/setup", "/debug", "/ipfs", "/health", "/test"}
		for _, path := range paths {
			if strings.HasPrefix(requestPath, path) {
				// IP address checks
				if ip != "127.0.0.1" && ip != "::1" {
					core.LogError("Client connecting from non-loopback address " + ip)
					c.AbortWithStatus(http.StatusBadRequest)
					return
				}
				if !validHosts[c.ClientIP()] {
					core.LogError("Client connecting from non-loopback address " + c.ClientIP())
					c.AbortWithStatus(http.StatusBadRequest)
					return
				}
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
