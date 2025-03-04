package middleware

// Catches all requests and checks for a valid auth cookie. If the cookie is valid, the request is allowed to continue.
// Otherwise, redirect to the login page.

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// List of path/method pairs that do not require authentication. Everything else requires a yp_auth cookie created from /login
var excludedTuplesAuth = [][]string{ // exact match on path and method
	{"/", "GET"},
	{"/setup", "GET"}, {"/setup/installed", "GET"}, {"/setup", "POST"},
	{"/favicon.ico", "GET"},
	{"/ping", "GET"},
	{"/mentalHealth", "GET"},
	{"/faq", "GET"},
	{"/robots.txt", "GET"},
	{"/settings/base", "GET"}, {"/settings/ipfs/port", "GET"}, {"/settings/base/url", "GET"},
	{"/s/", "GET"},
	{"/404", "GET"},
}

// Prefix match on path, exact match on method
var prefixGetExclusions = []string{"/static/", "/login", "/logout", "/profile/", "/posts"} // This exclusion should be used rarely, as it will exclude all GET requests to that path
var prefixPostExclusions = []string{"/login"}                                              // This exclusion should be used rarely, as it will exclude all POST requests to that path

func AuthMiddleware(cryptoSeed []byte, database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Early return for excluded paths
		if IsRequestExcluded(c) {
			//core.LogDebug("Auth - Path Excluded: " + c.Request.URL.Path + " Method: " + c.Request.Method)
			c.Next()
			return
		}
		//core.LogDebug("Auth - Path Enforced: " + c.Request.URL.Path + " Method: " + c.Request.Method)
		// Build redirect path
		path := c.Request.URL.Path
		redirect := ""
		if len(path) > 0 && path != "/login" && path != "/ping" {
			redirect = "?redirect=" + path
		}
		//core.LogDebug("Auth middleware - Redirect: " + redirect)
		// Check for auth cookie
		authCookie, err := c.Request.Cookie("yp_auth")
		if err != nil { // If there is no yp_auth cookie in the request
			core.LogDebug("No auth cookie. Redirecting to login")
			time.Sleep(30)                                  // debug
			c.Redirect(http.StatusFound, "/login"+redirect) // Redirect to the login page
			c.Abort()                                       // You're on the /login page, and no cookie is expected
			return
		}
		// Validate cookie
		authenticated := security.ValidateCookie(authCookie, cryptoSeed, database) // Check if the cookie is valid
		if !authenticated {
			core.LogDebug("Auth middleware - Cookie is invalid")
			security.InvalidateCookie(authCookie, cryptoSeed, database)
			time.Sleep(30) // debug
			c.Redirect(http.StatusFound, "/login"+redirect)
			c.Abort()
			return
		}
		// Extract values from cookie
		address, err := security.GetCookieValue(authCookie, cryptoSeed, "address", database)
		if err != nil { // If the cookie is valid, but can't get the address value, send back to /login
			core.LogDebug("Auth middleware - Failed to get address value from cookie: " + err.Error())
			security.InvalidateCookie(authCookie, cryptoSeed, database)
			time.Sleep(30) // debug
			c.Redirect(http.StatusFound, "/login"+redirect)
			c.Abort()
			return
		}
		blockchain, err := security.GetCookieValue(authCookie, cryptoSeed, "blockchain", database)
		if err != nil { // If the cookie is valid, but can't get the value, send back to /login
			core.LogDebug("Auth middleware - Failed to get blockchain value from cookie: " + err.Error())
			security.InvalidateCookie(authCookie, cryptoSeed, database)
			time.Sleep(30) // debug
			c.Redirect(http.StatusFound, "/login"+redirect)
			c.Abort()
			return
		}
		// Increment cookie to prevent against misuse
		err = security.IncrementCookie(c, cryptoSeed, database)
		if err != nil {
			core.LogDebug("Auth middleware - Failed to increment cookie: " + err.Error())
			security.InvalidateCookie(authCookie, cryptoSeed, database)
			time.Sleep(30) // debug
			c.Redirect(http.StatusFound, "/login"+redirect)
			c.Abort()
			return
		}
		// Set context values and continue
		c.Set("accountAddress", address) // set values in request context
		c.Set("blockchain", blockchain)
		c.Next()
	}
}

func IsRequestExcluded(c *gin.Context) bool {
	requestURI := c.Request.RequestURI
	parsedURL, _ := url.Parse(requestURI)
	requestPath := parsedURL.Path
	requestMethod := strings.TrimRight(c.Request.Method, "/\\")

	// Prefix exclusion checks
	for _, prefix := range prefixGetExclusions { // GET prefix exclusions
		if strings.HasPrefix(requestPath, prefix) && requestMethod == "GET" {
			return true
		}
	}
	for _, prefix := range prefixPostExclusions { // POST prefix exclusions
		if strings.HasPrefix(requestPath, prefix) && requestMethod == "POST" {
			return true
		}
	}
	if strings.HasPrefix(requestPath, "/p/") && requestPath != "/p/" && requestMethod == "GET" {
		// authenticated users need to be able to provide auth context to /p/ to view their own profile, but unauthenticated users must be able to access paths below /p/ to view other profiles
		return true
	}

	// Excluded tuples checks (exact match)
	for _, excludedTuple := range excludedTuplesAuth {
		if requestPath == excludedTuple[0] && requestMethod == excludedTuple[1] { // check if it matches an excluded tuple
			return true
		}
	}
	return false
}
func ensureTrailingSlash(payload string) string {
	if len(payload) == 0 || payload[len(payload)-1] != '/' {
		return payload + "/"
	}
	return payload
}
