package routes

// https://ipfs.io/ipfs/<CID>
// https://cloudflare-ipfs.com/ipfs/<CID>

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/network"
	"YourPlace/src/core/security"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func FilesRoutes(router *gin.Engine, database *db.Database, ipfs *network.IPFS, port int, gateway bool, pinningService *network.PinningService) {
	router.GET("/files/download/:uuid", func(c *gin.Context) {
		uuid := c.Param("uuid")
		if !security.IsValidUUID(uuid) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "invalid uuid"})
			return
		}
		c.AbortWithStatus(http.StatusNotImplemented)
		return
	})

	router.POST("/files/avatar/sign", func(c *gin.Context) {
		if pinningService == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"status": "Avatar upload not available"})
			return
		}
		auth, err := network.PinningServiceGenerateUploadAuth(pinningService, "/files/ipfs/avatar/add")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Failed to generate upload credentials"})
			return
		}
		c.SecureJSON(http.StatusOK, auth)
	})
	router.POST("/files/nft/sign", func(c *gin.Context) {
		if pinningService == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"status": "NFT minting not available"})
			return
		}
		auth, err := network.PinningServiceGenerateUploadAuth(pinningService, "/files/ipfs/nft/add")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Failed to generate upload credentials"})
			return
		}
		c.SecureJSON(http.StatusOK, auth)
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
	router.POST("/files/fetchExternal", func(c *gin.Context) {
		// Fetch an external image URL and save it locally, returning file data for upload.
		// Disabled in gateway mode to prevent abuse.
		if gateway {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Not available in gateway mode"})
			return
		}
		type Payload struct {
			URL string `json:"url" binding:"required"`
		}
		var payload Payload
		if err := c.BindJSON(&payload); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
			return
		}
		payload.URL = security.SanitizeNonPrintable(strings.TrimSpace(payload.URL))
		parsedURL, err := url.Parse(payload.URL)
		if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid URL"})
			return
		}
		if security.IsPrivateHost(parsedURL.Hostname()) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid URL"})
			return
		}
		client := &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return http.ErrUseLastResponse
				}
				if req.URL == nil || req.URL.Host == "" || security.IsPrivateHost(req.URL.Hostname()) {
					return http.ErrUseLastResponse
				}
				return nil
			},
		}
		req, err := http.NewRequest(http.MethodGet, payload.URL, nil)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid URL"})
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"status": "Failed to fetch external resource"})
			return
		}
		defer resp.Body.Close()
		if resp.Request == nil || resp.Request.URL == nil || resp.Request.URL.Host == "" || security.IsPrivateHost(resp.Request.URL.Hostname()) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid URL"})
			return
		}
		if resp.StatusCode != http.StatusOK {
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"status": "External resource returned error"})
			return
		}
		const maxSize = 50 << 20 // 50 MB
		if resp.ContentLength > maxSize {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"status": "External file too large"})
			return
		}
		contentType := resp.Header.Get("Content-Type")
		if mediaType, _, parseErr := mime.ParseMediaType(contentType); parseErr == nil {
			contentType = mediaType
		}
		urlPath := resp.Request.URL.Path
		fileName := filepath.Base(urlPath)
		if fileName == "" || fileName == "." || fileName == "/" {
			fileName = "external-" + uuid.New().String()
		}
		ext := filepath.Ext(fileName)
		if ext == "" {
			switch contentType {
			case "image/png":
				ext = ".png"
			case "image/jpeg":
				ext = ".jpg"
			case "image/gif":
				ext = ".gif"
			case "image/webp":
				ext = ".webp"
			case "video/mp4":
				ext = ".mp4"
			case "video/webm":
				ext = ".webm"
			default:
				ext = ".bin"
			}
			fileName += ext
		}
		uploadDirectory := security.SanitizePathTraversal(database.SettingsGetValue("uploadDirectory"))
		if !strings.HasSuffix(uploadDirectory, host.PathSeparator) {
			uploadDirectory = uploadDirectory + host.PathSeparator
		}
		if !host.DoesExist(uploadDirectory) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Upload directory does not exist"})
			return
		}
		fileUUID := uuid.New().String()
		tempFilePath := uploadDirectory + fileUUID + ext
		if !security.IsInParentDirectory(host.GetDataDir(), tempFilePath) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid file path"})
			return
		}
		limitedReader := io.LimitReader(resp.Body, maxSize+1)
		bodyBytes, err := io.ReadAll(limitedReader)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"status": "Failed to read external resource"})
			return
		}
		if int64(len(bodyBytes)) > maxSize {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"status": "External file too large"})
			return
		}
		host.WriteFile(tempFilePath, bodyBytes)
		if !host.DoesExist(tempFilePath) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Failed to save file"})
			return
		}
		fileHash, err := security.HashFile(tempFilePath)
		if err != nil {
			host.DeleteIfExists(tempFilePath)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Failed to hash file"})
			return
		}
		finalFilePath := uploadDirectory + fileHash + ext
		if !security.IsInParentDirectory(host.GetDataDir(), finalFilePath) {
			host.DeleteIfExists(tempFilePath)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid file path"})
			return
		}
		host.MoveFile(tempFilePath, finalFilePath)
		if !host.DoesExist(finalFilePath) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Failed to move file"})
			return
		}
		_, mimeType := security.GetFileType(finalFilePath)
		if !strings.HasPrefix(mimeType, "image/") && !strings.HasPrefix(mimeType, "video/") {
			host.DeleteIfExists(finalFilePath)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "URL must point to an image or video"})
			return
		}
		if ext == ".bin" {
			switch mimeType {
			case "image/png":
				fileName = strings.TrimSuffix(fileName, ext) + ".png"
			case "image/jpeg":
				fileName = strings.TrimSuffix(fileName, ext) + ".jpg"
			case "image/gif":
				fileName = strings.TrimSuffix(fileName, ext) + ".gif"
			case "image/webp":
				fileName = strings.TrimSuffix(fileName, ext) + ".webp"
			case "video/mp4":
				fileName = strings.TrimSuffix(fileName, ext) + ".mp4"
			case "video/webm":
				fileName = strings.TrimSuffix(fileName, ext) + ".webm"
			}
		}
		fileSize := int64(len(bodyBytes))
		database.FileAdd(fileUUID, fileHash, mimeType, fileName, fileSize)
		fileData := map[string]interface{}{
			"uuid":       fileUUID,
			"pathOnDisk": finalFilePath,
			"mimeType":   mimeType,
			"fileName":   fileName,
			"size":       fileSize,
			"hash":       fileHash,
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "data": []map[string]interface{}{fileData}})
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
