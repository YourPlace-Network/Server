package routes

// https://ipfs.io/ipfs/<CID>
// https://cloudflare-ipfs.com/ipfs/<CID>

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
	"path/filepath"
	"strings"
)

func FilesRoutes(router *gin.Engine, database *db.Database) {
	router.GET("/files/download/:uuid", func(c *gin.Context) {
		uuid := c.Param("uuid")
		if !security.IsValidUUID(uuid) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "invalid uuid"})
			return
		}

		c.AbortWithStatus(http.StatusNotImplemented)
		return
	})

	router.POST("/files/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "no file uploaded"})
			return
		}
		uploadDirectory := security.SanitizePathTraversal(database.SettingsGetValue("uploadDirectory"))
		if !strings.HasSuffix(uploadDirectory, host.PathSeparator) {
			uploadDirectory = uploadDirectory + host.PathSeparator
		}
		if !host.DoesExist(uploadDirectory) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "upload directory does not exist"})
			return
		}
		fileUUID := uuid.New().String()
		extension := filepath.Ext(file.Filename)
		newFileName := fileUUID + extension
		newFilePath := uploadDirectory + newFileName
		if !security.IsInParentDirectory(host.GetDataDir(), newFilePath) { // Limit uploads to the upload directory
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "cannot upload file to unknown directory"})
			return
		}
		err = c.SaveUploadedFile(file, newFilePath)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "failed to save file"})
			return
		}
		fileHash, err := security.HashFile(newFilePath)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "failed to hash file"})
			return
		}
		encodedFileHash := security.Base64EncodeBytes(fileHash)
		finalFilePath := uploadDirectory + encodedFileHash + extension
		if !security.IsInParentDirectory(host.GetDataDir(), finalFilePath) { // Limit uploads to the upload directory
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		host.MoveFile(newFilePath, finalFilePath)
		if !host.DoesExist(finalFilePath) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "failed to move file"})
			return
		}
		encodedUnsafeName := security.Base64Encode(file.Filename)
		database.FileAdd(encodedFileHash, extension, finalFilePath, encodedUnsafeName, file.Size)
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "uuid": encodedFileHash, "path": finalFilePath, "extension": extension, "encodedUnsafeName": encodedUnsafeName, "size": file.Size})
		return
	})
}
