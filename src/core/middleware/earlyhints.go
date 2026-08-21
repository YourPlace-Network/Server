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
		preloads := make([]string, 0, 7)
		for _, name := range sharedStyleAssets {
			// Styles are render-blocking, so hint them before scripts when the manifest has them.
			preloads = addPreload(preloads, assetManifest, name, "style")
		}
		preloads = addPreload(preloads, assetManifest, pageName+".css", "style")
		for _, name := range sharedScriptAssets {
			preloads = addPreload(preloads, assetManifest, name, "script")
		}
		preloads = addPreload(preloads, assetManifest, pageName+".js", "script")
		if len(preloads) > 0 {
			writeEarlyHints(c, preloads)
		}
		c.Next()
	}
}

func addPreload(preloads []string, assetManifest map[string]string, name string, as string) []string {
	if assetPath := resolveAsset(assetManifest, name); assetPath != "" {
		return append(preloads, "<"+assetPath+">; rel=preload; as="+as)
	}
	return preloads
}

type responseWriterUnwrapper interface {
	Unwrap() http.ResponseWriter
}

func writeEarlyHints(c *gin.Context, preloads []string) {
	if unwrapper, ok := c.Writer.(responseWriterUnwrapper); ok {
		for _, preload := range preloads {
			c.Writer.Header().Add("Link", preload)
		}
		// Write 103 on the underlying writer so Gin still controls the final 200/redirect/error response.
		unwrapper.Unwrap().WriteHeader(http.StatusEarlyHints)
		c.Writer.Header().Del("Link")
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
