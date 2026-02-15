package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var exactPageRoutes = map[string]string{
	"/":             "home",
	"/404":          "notFound",
	"/discover":     "home",
	"/faq":          "faq",
	"/login":        "login",
	"/logout":       "logout",
	"/mentalHealth": "mentalHealth",
	"/settings":     "settings",
	"/setup":        "setup",
	"/test":         "test",
}
var prefixPageRoutes = []struct {
	prefix   string
	pageName string
}{
	{"/p/", "profile"},
	{"/post/", "post"},
}
var sharedAssets = []string{"runtime.js", "vendors.js", "common.js"}

func EarlyHintsMiddleware(assetManifest map[string]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		pageName := exactPageRoutes[path]
		if pageName == "" {
			for _, pr := range prefixPageRoutes {
				if strings.HasPrefix(path, pr.prefix) {
					pageName = pr.pageName
					break
				}
			}
		}
		if pageName == "" {
			c.Next()
			return
		}
		hasHints := false
		for _, name := range sharedAssets {
			if assetPath := resolveAsset(assetManifest, name); assetPath != "" {
				c.Writer.Header().Add("Link", "<"+assetPath+">; rel=preload; as=script")
				hasHints = true
			}
		}
		if assetPath := resolveAsset(assetManifest, pageName+".js"); assetPath != "" {
			c.Writer.Header().Add("Link", "<"+assetPath+">; rel=preload; as=script")
			hasHints = true
		}
		if hasHints {
			c.Writer.WriteHeader(http.StatusEarlyHints)
		}
		c.Next()
	}
}

func resolveAsset(assetManifest map[string]string, name string) string {
	if assetManifest == nil {
		return ""
	}
	if hashed, ok := assetManifest[name]; ok {
		return hashed
	}
	if name == "common.js" || name == "runtime.js" || name == "vendors.js" {
		return ""
	}
	return "/static/js/" + name
}
