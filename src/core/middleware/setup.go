package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

// If YourPlace isn't installed, redirect all HTTP requests to /setup path

var excludedTuplesSetup = [][]string{ // These paths & methods are excluded from the setup middleware
	{"/setup", "GET"}, {"/setup", "POST"},
	{"/static/", "GET"},
	{"/favicon.ico", "GET"},
	{"/settings/base/url", "GET"},
	{"/login/nonce", "GET"}, {"/login/wallet/base", "POST"},
}

func SetupMiddleware(installed bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.RequestURI
		method := c.Request.Method
		for _, excludedTuple := range excludedTuplesSetup {
			if strings.HasPrefix(path, excludedTuple[0]) && method == excludedTuple[1] {
				//core.LogDebug("Setup Not Required - " + method + " " + path)
				return
			}
		}
		if !installed {
			//core.LogDebug("Setup Required - " + method + " - " + path)
			c.Redirect(http.StatusTemporaryRedirect, "/setup")
			return
		}
	}
}
