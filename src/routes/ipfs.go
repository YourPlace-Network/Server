package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/network"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func IPFSRoutes(router *gin.Engine, database *db.Database, ipfs *network.IPFS, port int) {
	router.GET("/ipfs/url/:cid", func(c *gin.Context) {
		cid := c.Param("cid")
		if !security.IsValidCID(cid) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "invalid cid"})
			return
		}
		portStr := strconv.FormatInt(int64(port+1), 10)
		c.SecureJSON(http.StatusOK, gin.H{"url": "http://localhost:" + portStr + "/ipfs/" + cid})
	})

	router.POST("/ipfs/add/", func(c *gin.Context) {
		// Take a file path (generated from POST /files/upload) and add it to IPFS + pin it and return the CID
		type Payload struct {
			FilePath string `json:"filePath" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid file UUID JSON"})
			return
		}
		cid, err := ipfs.IPFSAddFile(payload.FilePath)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "IPFS Add Error: " + err.Error()})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "cid": cid})
		return
	})
}
