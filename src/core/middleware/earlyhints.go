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
var sharedScriptAssets = []string{"runtime.js", "vendors.js", "common.js"}
var sharedStyleAssets = []string{"vendors.css", "common.css"}

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
		for _, name := range sharedStyleAssets {
			// Styles are render-blocking, so hint them before scripts when the manifest has them.
			hasHints = addPreload(c, assetManifest, name, "style") || hasHints
		}
		hasHints = addPreload(c, assetManifest, pageName+".css", "style") || hasHints
		for _, name := range sharedScriptAssets {
			hasHints = addPreload(c, assetManifest, name, "script") || hasHints
		}
		hasHints = addPreload(c, assetManifest, pageName+".js", "script") || hasHints
		if hasHints {
			writeEarlyHints(c)
		}
		c.Next()
	}
}

func addPreload(c *gin.Context, assetManifest map[string]string, name string, as string) bool {
	if assetPath := resolveAsset(assetManifest, name); assetPath != "" {
		c.Writer.Header().Add("Link", "<"+assetPath+">; rel=preload; as="+as)
		return true
	}
	return false
}

type responseWriterUnwrapper interface {
	Unwrap() http.ResponseWriter
}

func writeEarlyHints(c *gin.Context) {
	if unwrapper, ok := c.Writer.(responseWriterUnwrapper); ok {
		// Write 103 on the underlying writer so Gin still controls the final 200/redirect/error response.
		unwrapper.Unwrap().WriteHeader(http.StatusEarlyHints)
	}
}

func resolveAsset(assetManifest map[string]string, name string) string {
	if assetManifest == nil {
		return ""
	}
	if hashed, ok := assetManifest[name]; ok {
		return strings.ReplaceAll(hashed, "/static/js/../", "/static/")
	}
	return ""
}
