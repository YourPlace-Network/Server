package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func NFTBodyLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var maximum int64
		switch strings.TrimRight(c.Request.URL.Path, "/") {
		case "/login/wallet/pera":
			maximum = 32768
		case "/settings/content/nft", "/nft/welcome/accept":
			maximum = 16384
		case "/settings/content/nft/registration":
			maximum = 8192
		case "/settings/content/nft/template":
			maximum = 2048
		case "/settings/content/nft/grants", "/settings/content/nft/retry", "/nft/welcome", "/nft/welcome/prepare":
			maximum = 1024
		default:
			c.Next()
			return
		}
		if c.Request.ContentLength > maximum {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maximum)
		c.Next()
	}
}
