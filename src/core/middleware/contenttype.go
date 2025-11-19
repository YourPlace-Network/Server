package middleware

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func ContentTypeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.RequestURI
		if strings.HasPrefix(path, "/static/js/") && strings.HasSuffix(path, ".js") { // Ensure js content type for static JS assets
			c.Header("Content-Type", "application/javascript; charset=utf-8")
		} else if strings.HasSuffix(path, ".woff") { // Ensure font content type for WOFF font files
			c.Header("Content-Type", "font/woff")
		} else if strings.HasSuffix(path, ".woff2") { // Ensure font content type for WOFF2 font files
			c.Header("Content-Type", "font/woff2")
		}
		// Create a response writer that captures the response
		writer := &responseWriter{body: bytes.NewBuffer(nil), ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()                                                                    // Process request
		if writer.body.Len() > 0 && c.Writer.Header().Get("Content-Length") == "" { // Set content length after the response is written
			c.Header("Content-Length", strconv.Itoa(writer.body.Len()))
		}
	}
}
func CacheControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		extension := strings.ToLower(filepath.Ext(path))
		// Process request first
		c.Next()
		// Skip if errors occurred in handlers
		if len(c.Errors) > 0 {
			return
		}
		switch extension {
		case ".jpg", ".jpeg", ".png", ".gif", ".ico", ".svg", ".svg+xml", ".webp", ".woff", ".woff2":
			c.Header("Cache-Control", "public, max-age=604800") // Static assets with a longer cache time of 7 days
		case ".css", ".js":
			// Check if file has content hash (8 hex chars before extension)
			// Pattern: filename.a1b2c3d4.js or filename.a1b2c3d4.chunk.js
			hasContentHash := regexp.MustCompile(`\.[0-9a-f]{8}(\.(chunk|js|css))?$`).MatchString(path)
			if hasContentHash {
				c.Header("Cache-Control", "public, max-age=604800, immutable") // 7 days for hashed files
			} else {
				c.Header("Cache-Control", "public, max-age=86400") // 24 hours for non-hashed files
			}
		case ".json":
			c.Header("Cache-Control", "private, max-age=60") // JSON/API responses with 1-minute cache
		default:
			c.Header("Cache-Control", "no-cache, no-store") // Default to no cache
		}
	}
}

type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
