package routes

// https://ipfs.io/ipfs/<CID>
// https://cloudflare-ipfs.com/ipfs/<CID>

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/network"
	"YourPlace/src/core/security"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func FilesRoutes(router *gin.Engine, database *db.Database, ipfs *network.IPFS, port int) {
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
			// Check if IPFS pinning is configured and automatically add to IPFS
			pinningURL := database.SettingsGetValue("ipfsPinningURL")
			pinningKey := host.GetSecret("ipfsPinningKey")
			cid := ""
			if pinningURL != "" && pinningKey != "" {
				ipfsCid, err := ipfs.IPFSAddFile(finalFilePath)
				if err == nil {
					cid = ipfsCid
					database.IPFSAdd(fileUUID, cid) // Pin the file locally and remotely (handled by IPFSAddFile which automatically pins)
				}
			}
			fileData := map[string]interface{}{
				"uuid":       fileUUID,
				"pathOnDisk": finalFilePath,
				"mimeType":   mimeType,
				"fileName":   file.Filename,
				"size":       file.Size,
				"cid":        cid,
				"hash":       fileHash,
			}
			fileDataArray = append(fileDataArray, fileData)
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "data": fileDataArray})
		return
	})
	router.POST("/files/ipfs/add/", func(c *gin.Context) {
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
		if fileHash == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "File not found"})
			return
		}
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
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Error getting file extension"})
			return
		}
		ipfsPath := uploadDirectory + fileHash + extension
		if !host.DoesExist(ipfsPath) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "File does not exist on disk"})
			return
		}
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
