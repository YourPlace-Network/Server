package middleware

import (
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
	"strings"
)

var blockedAgents = []string{
	"WWW:Mechanize",
	"WWW-Mechanize",
	"libwww-perl",
	"Acunetix",
	"sqlmap",
	"OWASP",
	"Shodan",
	"ZmEu",
	"PyCurl",
	"python-requests",
	"OpenVAS",
	"Nikto",
	"Masscan",
	"LWP:Simple",
	"WPScan",
	"curl",
	"wget",
	"Baidu",
	"Yandex",
	"Googlebot",
	"Bingbot",
	"Slurp",
	"DuckDuckBot",
	"Facebot",
	"urllib",
	"libcurl",
}

func IdsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userAgent := strings.ToLower(c.Request.Header.Get("User-Agent"))
		for _, blockedAgent := range blockedAgents {
			if strings.Contains(userAgent, strings.ToLower(blockedAgent)) {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		}
		c.Next()
	}
}

func DnsRebindingMiddleware(allowedHosts []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.Request.Host
		// Remove port number if present
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		// Check if the host is in the allowed list
		valid := false
		for _, allowedHost := range allowedHosts {
			if strings.EqualFold(host, allowedHost) {
				valid = true
				break
			}
		}
		if !valid {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

func HotPatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}

	/*return func(c *gin.Context) { // https://github.com/gin-gonic/gin/pull/3556
		disposition := c.Request.Header.Get("Content-Disposition")
		re := regexp.MustCompile(`filename="([^"]*)`)
		matches := re.FindStringSubmatch(disposition)
		if len(matches) > 1 {
			c.Writer.Header().Del("Content-Disposition")
			c.Writer.Header().Set("Content-Disposition",
				`attachment; filename="`+strings.Replace(matches[0], "\"", "\\\"", -1)+`"`)
		}
	}*/
}
