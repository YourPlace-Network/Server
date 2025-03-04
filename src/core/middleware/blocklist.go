package middleware

import (
	"YourPlace/src/core/db"
	"github.com/gin-gonic/gin"
)

func BlockedContent(database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		/*userAgent := strings.ToLower(c.Request.Header.Get("User-Agent"))
		for _, blockedAgent := range blockedAgents {
			if strings.Contains(userAgent, strings.ToLower(blockedAgent)) {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		}*/
		c.Next()
	}
}
