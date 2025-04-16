package routes

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/network"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
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
			FileUUID string `json:"fileUUID" required:"true"`
		}
		var payload Payload
		err := c.BindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid file UUID JSON"})
			return
		}
		fileHash := database.GetFileHashFromUUID(payload.FileUUID)
		core.LogInfo("file hash: " + fileHash)
		uploadDirectory := security.SanitizePathTraversal(database.SettingsGetValue("uploadDirectory"))
		if !strings.HasSuffix(uploadDirectory, host.PathSeparator) {
			uploadDirectory = uploadDirectory + host.PathSeparator
		}
		if !host.DoesExist(uploadDirectory) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Upload directory does not exist"})
			return
		}
		extension, err := host.GetFileExtenstion(uploadDirectory, fileHash)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Error getting file extension"})
		}
		ipfsPath := uploadDirectory + fileHash + extension
		cid, err := ipfs.IPFSAddFile(ipfsPath)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "IPFS Add Error: " + err.Error()})
			return
		}
		database.IPFSAdd(payload.FileUUID, cid)
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "cid": cid})
		return
	})
}
