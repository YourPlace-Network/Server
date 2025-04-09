package routes

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func NotFoundRoutes(router *gin.Engine, title string, gateway bool) {
	router.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "src/templates/pages/notFound.tmpl", gin.H{
			"title":       title,
			"pageName":    "notFound",
			"gatewayMode": gateway,
		})
	})
	router.GET("/404", func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "src/templates/pages/notFound.tmpl", gin.H{
			"title":       title,
			"pageName":    "notFound",
			"gatewayMode": gateway,
		})
	})
}
