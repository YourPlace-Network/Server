package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"net/http"
)

func TestRoutes(router *gin.Engine, title string, gateway bool) {
	router.GET("/test", func(c *gin.Context) {
		token := csrf.Token(c.Request)
		c.HTML(http.StatusOK, "src/templates/pages/test.tmpl", gin.H{
			"title":       title,
			"pageName":    "test",
			"csrfToken":   token,
			"gatewayMode": gateway,
		})
	})
}
