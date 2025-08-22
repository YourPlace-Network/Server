package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"net/http"

	"github.com/gin-gonic/gin"
)

type MCPCapability struct {
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	Description string `json:"description"`
	Method      string `json:"method"`
}

var mcpCapabilities []MCPCapability

func registerMCPCapability(name, endpoint, description, method string) {
	capability := MCPCapability{
		Name:        name,
		Endpoint:    endpoint,
		Description: description,
		Method:      method,
	}
	mcpCapabilities = append(mcpCapabilities, capability)
}
func MCPRoutes(router *gin.Engine, database *db.Database) {
	registerMCPCapability("serverNetwork", "/mcp/serverNetwork", "Get the blockchain network this server operates on", "GET")
	router.GET("/mcp/serverNetwork", func(c *gin.Context) {
		serverNetwork := database.AuthGetServerOwnerNetwork()
		if !security.IsValidNetwork(serverNetwork) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Server network not found",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"serverNetwork": serverNetwork,
		})
	})

	registerMCPCapability("serverOwner", "/mcp/serverOwner", "Get the wallet address of the server owner with network validation", "GET")
	router.GET("/mcp/serverOwner", func(c *gin.Context) {
		serverOwner := database.AuthGetServerOwnerAddress()
		serverNetwork := database.AuthGetServerOwnerNetwork()
		if !security.IsValidNetwork(serverNetwork) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Server network not found",
			})
			return
		}
		if serverOwner == "" || !security.IsValidAddress(serverOwner, serverNetwork) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Server owner not found",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"serverOwner": serverOwner,
		})
	})

	registerMCPCapability("capabilities", "/mcp/capabilities", "Discover available MCP capabilities and endpoints", "GET")
	router.GET("/mcp/capabilities", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"capabilities": mcpCapabilities,
		})
	})
}
