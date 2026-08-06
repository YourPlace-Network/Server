package middleware

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var hashedStaticAssetPattern = regexp.MustCompile(`\.[0-9a-f]{8}(\.chunk)?\.(css|js)$`)

func ContentTypeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/static/js/") && strings.HasSuffix(path, ".js") { // Ensure js content type for static JS assets
			c.Header("Content-Type", "application/javascript; charset=utf-8")
		} else if strings.HasSuffix(path, ".woff") { // Ensure font content type for WOFF font files
			c.Header("Content-Type", "font/woff")
		} else if strings.HasSuffix(path, ".woff2") { // Ensure font content type for WOFF2 font files
			c.Header("Content-Type", "font/woff2")
		} else if strings.HasPrefix(path, "/static/css/") && strings.HasSuffix(path, ".css") {
			c.Header("Content-Type", "text/css; charset=utf-8")
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

		switch {
		case strings.HasPrefix(path, "/static/") && hashedStaticAssetPattern.MatchString(path):
			// Hashed build assets are content-addressed, so browsers and edge caches can keep them across deploys.
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		case strings.HasPrefix(path, "/static/") && (extension == ".css" || extension == ".js"):
			c.Header("Cache-Control", "public, max-age=86400")
		case strings.HasPrefix(path, "/static/") && isCacheableStaticExtension(extension):
			c.Header("Cache-Control", "public, max-age=604800")
		case extension == ".json":
			c.Header("Cache-Control", "private, max-age=60")
		default:
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}

		c.Next()
	}
}

func isCacheableStaticExtension(extension string) bool {
	switch extension {
	case ".jpg", ".jpeg", ".png", ".gif", ".ico", ".svg", ".webp", ".woff", ".woff2":
		return true
	default:
		return false
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
