package routes

import (
	"YourPlace/src/core/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
)

func TestRoutes(router *gin.Engine, title string, gateway bool) {
	router.GET("/test", func(c *gin.Context) {
		token := middleware.GetCSRFToken(c)
		c.HTML(http.StatusOK, "src/templates/pages/test.tmpl", gin.H{
			"title":       title,
			"pageName":    "test",
			"csrfToken":   token,
			"gatewayMode": gateway,
		})
	})
}
