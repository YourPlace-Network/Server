package middleware

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
)

func ContentTypeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.RequestURI
		if strings.HasPrefix(path, "/static/js/") && strings.HasSuffix(path, ".js") { // Ensure js content type for static JS assets
			c.Header("Content-Type", "application/javascript; charset=utf-8")
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

type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
