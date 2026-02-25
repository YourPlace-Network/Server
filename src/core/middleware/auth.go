package middleware

// Catches all requests and checks for a valid auth cookie. If the cookie is valid, the request is allowed to continue.
// Otherwise, redirect to the login page.

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// List of path/method pairs that do not require authentication. Everything else requires a yp_auth cookie created from /login
var excludedTuplesAuth = [][]string{ // exact match on path and method
	{"/", "GET"},                                                       // Home page
	{"/setup", "GET"}, {"/setup/installed", "GET"}, {"/setup", "POST"}, // Setup pages
	{"/discover", "GET"},                                    // Discover API
	{"/favicon.ico", "GET"},                                 // Favicon
	{"/ping", "GET"},                                        // Health check endpoint
	{"/mentalHealth", "GET"}, {"/mentalHealth/tips", "GET"}, // Mental health resources
	{"/faq", "GET"},                                  // FAQ page
	{"/notification", "GET"},                         // Notification page
	{"/robots.txt", "GET"},                           // Robots direction
	{"/rpc/base", "POST"}, {"/rpc/ethereum", "POST"}, // Public RPC proxy endpoints
	{"/settings/database/exportSnapshot", "POST"}, {"/settings/database/importSnapshot", "POST"}, // Database snapshot endpoints
	{"/settings/ipfs/port", "GET"},         // IPFS settings
	{"/settings/base", "GET"},              // Base settings
	{"/settings/services/algorand", "GET"}, // Algorand settings
	{"/s", "GET"},                          // Search endpoint
	{"/404", "GET"},                        // 404 not-found page
}

// Prefix match on path, exact match on method
var prefixGetExclusions = []string{ // Auth exclude all GET requests to these paths (public read-only APIs)
	"/static/", "/login", "/logout", // Necessary for login page and static assets
	"/profile/", "/posts/", "/post/", "/comments/", "/reactions/", // Public content browsing
	"/services/algorand/nfd/", "/services/twitter/oembed", // Public data-fetching proxies for post embeds and profile data
}
var prefixPostExclusions = []string{ // Exclude all POST requests to these paths (public, non-auth, stateful APIs)
	"/login", // Login endpoints
}

func AuthMiddleware(cryptoSeed []byte, database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		isExcluded := IsRequestExcluded(c)
		// Always try to set context variables if valid cookie exists
		authCookie, authCookieErr := c.Request.Cookie("yp_auth")
		// Cache cookie validation result to avoid multiple PBKDF2 operations
		var validAuthCookie bool
		var address, blockchain string
		var authDataValid bool

		if authCookieErr == nil {
			validAuthCookie = security.ValidateCookie(authCookie, cryptoSeed, database)
			if validAuthCookie {
				var err1, err2 error
				address, err1 = security.GetCookieValue(authCookie, cryptoSeed, "address", database)
				blockchain, err2 = security.GetCookieValue(authCookie, cryptoSeed, "blockchain", database)
				authDataValid = (err1 == nil && err2 == nil)

				if authDataValid {
					c.Set("accountAddress", address)
					c.Set("blockchain", blockchain)
					// Only increment cookie for non-excluded paths to avoid unnecessary operations
					if !isExcluded {
						err := security.IncrementCookie(c, cryptoSeed, database)
						if err != nil {
							core.LogDebug("Auth middleware - Failed to increment cookie: " + err.Error())
							security.InvalidateCookie(authCookie, cryptoSeed, database)
							c.Redirect(http.StatusFound, "/login")
							c.Abort()
							return
						}
					}
				}
			}
		}
		if isExcluded { // Early return for excluded paths (after setting context if possible)
			c.Next()
			return
		}
		// Build redirect path
		path := c.Request.URL.Path
		redirect := ""
		if len(path) > 0 && path != "/login" && path != "/ping" {
			redirect = "?redirect=" + path
		}
		// Don't redirect HEAD requests - just deny them
		if c.Request.Method == "HEAD" {
			c.AbortWithStatus(http.StatusMethodNotAllowed)
			return
		}
		// Use cached validation results to avoid redundant PBKDF2 operations
		if authCookieErr != nil || !validAuthCookie {
			if authCookieErr != nil { // The cookie doesn't exist, so redirect to /login
				c.Redirect(http.StatusFound, "/login"+redirect)
				c.Abort()
				return
			}
			// Cookie exists but is invalid
			security.InvalidateCookie(authCookie, cryptoSeed, database)
			c.Redirect(http.StatusFound, "/login"+redirect)
			c.Abort()
			return
		}
		// Use cached auth data if available, otherwise extract values
		if !authDataValid {
			var err error
			address, err = security.GetCookieValue(authCookie, cryptoSeed, "address", database)
			if err != nil { // If the cookie is valid, but can't get the address value, send back to /login
				core.LogDebug("Auth middleware - Failed to get address value from cookie: " + err.Error())
				security.InvalidateCookie(authCookie, cryptoSeed, database)
				c.Redirect(http.StatusFound, "/login"+redirect)
				c.Abort()
				return
			}
			blockchain, err = security.GetCookieValue(authCookie, cryptoSeed, "blockchain", database)
			if err != nil { // If the cookie is valid, but can't get the value, send back to /login
				core.LogDebug("Auth middleware - Failed to get blockchain value from cookie: " + err.Error())
				security.InvalidateCookie(authCookie, cryptoSeed, database)
				c.Redirect(http.StatusFound, "/login"+redirect)
				c.Abort()
				return
			}
			// Set context values
			c.Set("accountAddress", address)
			c.Set("blockchain", blockchain)
		}
		// Increment cookie to prevent against misuse (only if not already done)
		if authDataValid && isExcluded {
			// Cookie was already incremented in the first section for non-excluded paths
		} else if !authDataValid {
			err := security.IncrementCookie(c, cryptoSeed, database)
			if err != nil {
				core.LogDebug("Auth middleware - Failed to increment cookie: " + err.Error())
				security.InvalidateCookie(authCookie, cryptoSeed, database)
				c.Redirect(http.StatusFound, "/login"+redirect)
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

func IsRequestExcluded(c *gin.Context) bool {
	requestURI := c.Request.RequestURI
	parsedURL, _ := url.Parse(requestURI)
	requestPath := parsedURL.Path
	requestMethod := strings.TrimRight(c.Request.Method, "/\\")
	if requestPath == "/login" && requestMethod == "HEAD" {
		return true // Exclude all HEAD requests to /login to prevent redirect loops
	}
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
	// Special case for /p/ profile browsing paths
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
