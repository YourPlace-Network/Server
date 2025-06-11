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
		// Check content length first
		const maxUploadSize = 100 << 30 // 100 GB
		if c.Request.ContentLength > maxUploadSize {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"status": "File size exceeds limit"})
			return
		}
		form, err := c.MultipartForm()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "No file uploaded"})
			return
		}
		files := form.File["file"]
		if len(files) == 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "No file uploaded"})
			return
		}
		// Limit number of files
		if len(files) > 10 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Too many files uploaded"})
			return
		}
		fileDataArray := []map[string]interface{}{}
		uploadDirectory := security.SanitizePathTraversal(database.SettingsGetValue("uploadDirectory"))
		if !strings.HasSuffix(uploadDirectory, host.PathSeparator) {
			uploadDirectory = uploadDirectory + host.PathSeparator
		}
		if !host.DoesExist(uploadDirectory) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Upload directory does not exist"})
			return
		}
		for _, file := range files {
			fileUUID := uuid.New().String()
			extension := filepath.Ext(file.Filename)
			newFileName := fileUUID + extension
			newFilePath := uploadDirectory + newFileName
			if !security.IsInParentDirectory(host.GetDataDir(), newFilePath) { // Limit uploads to the upload directory
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Cannot upload file to unknown directory"})
				return
			}
			err = c.SaveUploadedFile(file, newFilePath)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to save file"})
				return
			}
			fileHash, err := security.HashFile(newFilePath)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to hash file"})
				return
			}
			finalFilePath := uploadDirectory + fileHash + extension
			if !security.IsInParentDirectory(host.GetDataDir(), finalFilePath) { // Limit uploads to the upload directory
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Cannot upload file to unknown directory"})
				return
			}
			host.MoveFile(newFilePath, finalFilePath)
			if !host.DoesExist(finalFilePath) {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to move file"})
				return
			}
			_, mimeType := security.GetFileType(finalFilePath)
			database.FileAdd(fileUUID, fileHash, mimeType, file.Filename, file.Size)
			fileData := map[string]interface{}{
				"uuid":       fileUUID,
				"pathOnDisk": finalFilePath,
				"mimeType":   mimeType,
				"fileName":   file.Filename,
				"size":       file.Size,
			}
			fileDataArray = append(fileDataArray, fileData)
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "data": fileDataArray})
		return
	})
}
