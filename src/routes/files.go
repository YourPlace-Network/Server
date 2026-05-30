package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/middleware"
	"YourPlace/src/core/network"
	"YourPlace/src/core/security"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func getViewerIdentity(c *gin.Context) (string, string, bool) {
	viewerAddressValue, hasAddress := c.Get("accountAddress")
	viewerBlockchainValue, hasBlockchain := c.Get("blockchain")
	if !hasAddress || !hasBlockchain {
		return "", "", false
	}
	viewerAddress, okAddress := viewerAddressValue.(string)
	viewerBlockchain, okBlockchain := viewerBlockchainValue.(string)
	if !okAddress || !okBlockchain {
		return "", "", false
	}
	return viewerAddress, viewerBlockchain, true
}

func getUploadDirectory(database *db.Database) string {
	uploadDirectory := security.SanitizePathTraversal(database.SettingsGetValue("uploadDirectory"))
	if !strings.HasSuffix(uploadDirectory, host.PathSeparator) {
		uploadDirectory += host.PathSeparator
	}
	return uploadDirectory
}

func resolveLocalFilePath(database *db.Database, localFile map[string]interface{}) (string, string, bool) {
	if localFile == nil {
		return "", "", false
	}
	fileHash, okHash := localFile["fileHash"].(string)
	fileName, okName := localFile["fileName"].(string)
	if !okHash || !okName || fileHash == "" || !security.IsValidHex(fileHash) {
		return "", "", false
	}
	uploadDirectory := getUploadDirectory(database)
	if uploadDirectory == "" || !host.DoesExist(uploadDirectory) {
		return "", "", false
	}
	fileExtension, err := host.GetFileExtenstion(uploadDirectory, fileHash)
	if err != nil || fileExtension == "" {
		fileExtension = filepath.Ext(fileName)
	}
	if fileExtension == "" {
		return "", "", false
	}
	filePath := filepath.Join(uploadDirectory, fileHash+fileExtension)
	if !security.IsInParentDirectory(uploadDirectory, filePath) {
		return "", "", false
	}
	safeFileName := filepath.Base(security.SanitizePathTraversal(security.SanitizeNonPrintable(fileName)))
	if safeFileName == "" || safeFileName == "." || safeFileName == string(filepath.Separator) {
		safeFileName = fileHash + fileExtension
	}
	return filePath, safeFileName, true
}

func finalizeLocalFileVisibility(database *db.Database, ownerAddress string, ownerBlockchain string, cid string, source string, state string) {
	database.LocalFileUpdate(ownerAddress, ownerBlockchain, cid, source, state)
}
func removeLocalFileMFSPaths(database *db.Database, ipfs *network.IPFS, localFile map[string]interface{}) {
	if ipfs == nil || localFile == nil {
		return
	}
	filePath, safeFileName, ok := resolveLocalFilePath(database, localFile)
	if !ok {
		return
	}
	fileBaseName := filepath.Base(filePath)
	if fileBaseName != "" {
		_ = ipfs.IPFSRemoveFromMFS("/uploads/" + fileBaseName)
	}
	if safeFileName != "" && safeFileName != fileBaseName {
		_ = ipfs.IPFSRemoveFromMFS("/uploads/" + safeFileName)
	}
}
func buildRenamedFileName(currentFileName string, requestedBase string) string {
	currentSafeName := filepath.Base(security.SanitizePathTraversal(security.SanitizeNonPrintable(currentFileName)))
	if currentSafeName == "" {
		return ""
	}
	safeBase := security.SanitizeNonPrintable(strings.TrimSpace(requestedBase))
	if safeBase == "" {
		return ""
	}
	fileExtension := filepath.Ext(currentSafeName)
	renamedFileName := safeBase
	if fileExtension != "" {
		renamedFileName += fileExtension
	}
	renamedFileName = filepath.Base(security.SanitizePathTraversal(renamedFileName))
	if !security.IsValidIndexedFilename(renamedFileName) {
		return ""
	}
	return renamedFileName
}
func createRenamedTempCopy(database *db.Database, localFile map[string]interface{}, renamedFileName string) (string, func(), error) {
	filePath, _, ok := resolveLocalFilePath(database, localFile)
	if !ok || !host.DoesExist(filePath) {
		return "", func() {}, os.ErrNotExist
	}
	uploadDirectory := getUploadDirectory(database)
	tempDirectory, err := os.MkdirTemp(uploadDirectory, "rename-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() {
		_ = os.RemoveAll(tempDirectory)
	}
	tempFilePath := filepath.Join(tempDirectory, renamedFileName)
	if !security.IsInParentDirectory(tempDirectory, tempFilePath) {
		cleanup()
		return "", func() {}, os.ErrPermission
	}
	if !host.CopyFile(filePath, tempFilePath) {
		cleanup()
		return "", func() {}, os.ErrInvalid
	}
	return tempFilePath, cleanup, nil
}
func upsertLocalFileRecord(database *db.Database, localFile map[string]interface{}, cid string, fileName string) {
	ownerAddress, _ := localFile["ownerAddress"].(string)
	ownerBlockchain, _ := localFile["ownerBlockchain"].(string)
	fileHash, _ := localFile["fileHash"].(string)
	mimeType, _ := localFile["mimeType"].(string)
	source, _ := localFile["source"].(string)
	state, _ := localFile["state"].(string)
	size, _ := localFile["size"].(int64)
	database.LocalFileUpsert(ownerAddress, ownerBlockchain, fileHash, cid, mimeType, fileName, size, source, state)
}
func canRenamePublishedLocalFile(localFile map[string]interface{}) bool {
	if localFile == nil {
		return false
	}
	source, _ := localFile["source"].(string)
	return source == "" || source == "direct_upload"
}
func getViewerLocalFile(database *db.Database, viewerAddress string, viewerBlockchain string, cid string) map[string]interface{} {
	localFile := database.LocalFileGet(viewerAddress, viewerBlockchain, cid)
	if localFile != nil {
		return localFile
	}
	return database.LocalFileGetByOwnerAddress(viewerAddress, cid)
}
func streamLocalFileInline(c *gin.Context, database *db.Database, localFile map[string]interface{}) {
	filePath, _, ok := resolveLocalFilePath(database, localFile)
	if !ok || !host.DoesExist(filePath) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"status": "File not found"})
		return
	}
	fileHandle, err := os.Open(filePath)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"status": "File not found"})
		return
	}
	defer fileHandle.Close()
	fileInfo, err := fileHandle.Stat()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"status": "File not found"})
		return
	}
	mimeType, _ := localFile["mimeType"].(string)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Header("Cache-Control", "private, no-store, must-revalidate")
	c.Header("Content-Type", mimeType)
	http.ServeContent(c.Writer, c.Request, fileInfo.Name(), fileInfo.ModTime(), fileHandle)
}
func deleteLocalFileResources(database *db.Database, ipfs *network.IPFS, localFile map[string]interface{}) {
	if localFile == nil {
		return
	}
	cid, _ := localFile["cid"].(string)
	ownerAddress, _ := localFile["ownerAddress"].(string)
	ownerBlockchain, _ := localFile["ownerBlockchain"].(string)
	filePath, _, hasFilePath := resolveLocalFilePath(database, localFile)
	removeLocalFileMFSPaths(database, ipfs, localFile)
	if ipfs != nil && cid != "" {
		_ = ipfs.IPFSUnpinFile(cid)
		_ = ipfs.IPFSGarbageCollect()
	}
	if hasFilePath {
		host.DeleteIfExists(filePath)
	}
	database.LocalFileDelete(ownerAddress, ownerBlockchain, cid)
}

func FilesRoutes(router *gin.Engine, database *db.Database, ipfs *network.IPFS, port int, gateway bool, pinningService *network.PinningService) {
	_ = port

	router.GET("/files/download/:cid", func(c *gin.Context) {
		viewerAddress, viewerBlockchain, authenticated := getViewerIdentity(c)
		if !authenticated {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "Authentication required"})
			return
		}
		cid := strings.TrimSpace(c.Param("cid"))
		if !security.IsValidCID(cid) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid CID"})
			return
		}
		localFile := getViewerLocalFile(database, viewerAddress, viewerBlockchain, cid)
		filePath, fileName, ok := resolveLocalFilePath(database, localFile)
		if !ok || !host.DoesExist(filePath) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"status": "File not found"})
			return
		}
		c.FileAttachment(filePath, fileName)
	})
	router.GET("/files/preview/:cid", func(c *gin.Context) {
		viewerAddress, viewerBlockchain, authenticated := getViewerIdentity(c)
		if !authenticated {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "Authentication required"})
			return
		}
		cid := strings.TrimSpace(c.Param("cid"))
		if !security.IsValidCID(cid) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid CID"})
			return
		}
		localFile := getViewerLocalFile(database, viewerAddress, viewerBlockchain, cid)
		if localFile == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"status": "File not found"})
			return
		}
		streamLocalFileInline(c, database, localFile)
	})
	router.GET("/files/data/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid blockchain"})
			return
		}
		addressParam := c.Param("address")
		if !security.IsValidAddress(addressParam, blockchainParam) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid address"})
			return
		}
		viewerAddress, viewerBlockchain, authenticated := getViewerIdentity(c)
		isGuest := true
		if authenticated && viewerAddress == addressParam && viewerBlockchain == blockchainParam {
			isGuest = false
		}
		files := database.ProfileGetFilesForViewer(addressParam, blockchainParam, viewerAddress, viewerBlockchain)
		c.SecureJSON(http.StatusOK, gin.H{
			"status":  "success",
			"files":   files,
			"isGuest": isGuest,
		})
	})
	router.GET("/files/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.Redirect(http.StatusSeeOther, "/404")
			return
		}
		addressParam := c.Param("address")
		if !security.IsValidAddress(addressParam, blockchainParam) {
			c.Redirect(http.StatusSeeOther, "/404")
			return
		}
		viewerAddress, viewerBlockchain, authenticated := getViewerIdentity(c)
		isGuest := true
		if authenticated && viewerAddress == addressParam && viewerBlockchain == blockchainParam {
			isGuest = false
		}
		displayName := database.ProfileGetName(addressParam, blockchainParam)
		if displayName == "" {
			displayName = database.ProfileGetEnsName(addressParam, blockchainParam)
		}
		if displayName == "" {
			displayName = addressParam[:6] + "..." + addressParam[len(addressParam)-4:]
		}
		c.HTML(http.StatusOK, "src/templates/pages/files.tmpl", gin.H{
			"title":                 displayName + " Files",
			"pageName":              "files",
			"csrfToken":             middleware.GetCSRFToken(c),
			"injectedAddress":       addressParam,
			"injectedBlockchain":    blockchainParam,
			"isCookieAuthenticated": authenticated,
			"isGuest":               isGuest,
			"gatewayMode":           gateway,
			"userAddress":           viewerAddress,
			"userBlockchain":        viewerBlockchain,
		})
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
		if gateway {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Not available in gateway mode"})
			return
		}
		viewerAddress, viewerBlockchain, authenticated := getViewerIdentity(c)
		if !authenticated {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "Authentication required"})
			return
		}
		if ipfs == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"status": "IPFS not available"})
			return
		}
		const maxUploadSize = 100 << 30
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
		uploadDirectory := getUploadDirectory(database)
		if uploadDirectory == "" || !host.DoesExist(uploadDirectory) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Upload directory does not exist"})
			return
		}
		fileDataArray := []map[string]interface{}{}
		for _, file := range files {
			tempFileName := uuid.New().String() + filepath.Ext(file.Filename)
			tempFilePath := filepath.Join(uploadDirectory, tempFileName)
			if !security.IsInParentDirectory(uploadDirectory, tempFilePath) {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Cannot upload file to unknown directory"})
				return
			}
			if err = c.SaveUploadedFile(file, tempFilePath); err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to save file"})
				return
			}
			fileHash, err := security.HashFile(tempFilePath)
			if err != nil {
				host.DeleteIfExists(tempFilePath)
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to hash file"})
				return
			}
			finalFilePath := filepath.Join(uploadDirectory, fileHash+filepath.Ext(file.Filename))
			if !security.IsInParentDirectory(uploadDirectory, finalFilePath) {
				host.DeleteIfExists(tempFilePath)
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Cannot upload file to unknown directory"})
				return
			}
			if host.DoesExist(finalFilePath) {
				host.DeleteIfExists(tempFilePath)
			} else {
				host.MoveFile(tempFilePath, finalFilePath)
			}
			if !host.DoesExist(finalFilePath) {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to store file"})
				return
			}
			_, mimeType := security.GetFileType(finalFilePath)
			cid, err := ipfs.IPFSHashFile(finalFilePath)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to generate CID"})
				return
			}
			database.LocalFileUpsert(viewerAddress, viewerBlockchain, fileHash, cid, mimeType, file.Filename, file.Size, "direct_upload", "staged")
			fileDataArray = append(fileDataArray, map[string]interface{}{
				"cid":      cid,
				"mimeType": mimeType,
				"fileName": file.Filename,
				"size":     file.Size,
				"hash":     fileHash,
			})
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "data": fileDataArray})
	})
	router.POST("/files/fetchExternal", func(c *gin.Context) {
		if gateway {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Not available in gateway mode"})
			return
		}
		viewerAddress, viewerBlockchain, authenticated := getViewerIdentity(c)
		if !authenticated {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "Authentication required"})
			return
		}
		if ipfs == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"status": "IPFS not available"})
			return
		}
		type payload struct {
			URL string `json:"url" binding:"required"`
		}
		var requestData payload
		if err := c.BindJSON(&requestData); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
			return
		}
		requestData.URL = security.SanitizeNonPrintable(strings.TrimSpace(requestData.URL))
		parsedURL, err := url.Parse(requestData.URL)
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
		req, err := http.NewRequest(http.MethodGet, requestData.URL, nil)
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
		const maxSize = 50 << 20
		if resp.ContentLength > maxSize {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"status": "External file too large"})
			return
		}
		contentType := resp.Header.Get("Content-Type")
		if mediaType, _, parseErr := mime.ParseMediaType(contentType); parseErr == nil {
			contentType = mediaType
		}
		fileName := filepath.Base(resp.Request.URL.Path)
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
		uploadDirectory := getUploadDirectory(database)
		if uploadDirectory == "" || !host.DoesExist(uploadDirectory) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Upload directory does not exist"})
			return
		}
		tempFilePath := filepath.Join(uploadDirectory, uuid.New().String()+ext)
		if !security.IsInParentDirectory(uploadDirectory, tempFilePath) {
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
		finalFilePath := filepath.Join(uploadDirectory, fileHash+ext)
		if !security.IsInParentDirectory(uploadDirectory, finalFilePath) {
			host.DeleteIfExists(tempFilePath)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid file path"})
			return
		}
		if host.DoesExist(finalFilePath) {
			host.DeleteIfExists(tempFilePath)
		} else {
			host.MoveFile(tempFilePath, finalFilePath)
		}
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
		cid, err := ipfs.IPFSHashFile(finalFilePath)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Failed to generate CID"})
			return
		}
		fileSize := int64(len(bodyBytes))
		database.LocalFileUpsert(viewerAddress, viewerBlockchain, fileHash, cid, mimeType, fileName, fileSize, "direct_upload", "staged")
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "data": []map[string]interface{}{{
			"cid":      cid,
			"mimeType": mimeType,
			"fileName": fileName,
			"size":     fileSize,
			"hash":     fileHash,
		}}})
	})
	router.POST("/files/ipfs/add", func(c *gin.Context) {
		if gateway {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Not available in gateway mode"})
			return
		}
		viewerAddress, viewerBlockchain, authenticated := getViewerIdentity(c)
		if !authenticated {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "Authentication required"})
			return
		}
		type payload struct {
			CID string `json:"cid" binding:"required"`
		}
		var requestData payload
		if err := c.BindJSON(&requestData); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid CID JSON"})
			return
		}
		requestData.CID = strings.TrimSpace(requestData.CID)
		if !security.IsValidCID(requestData.CID) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid CID"})
			return
		}
		localFile := getViewerLocalFile(database, viewerAddress, viewerBlockchain, requestData.CID)
		filePath, _, ok := resolveLocalFilePath(database, localFile)
		if !ok || !host.DoesExist(filePath) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"status": "File not found"})
			return
		}
		cid, err := ipfs.IPFSAddFile(filePath)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "IPFS Add Error: " + err.Error()})
			return
		}
		source, _ := localFile["source"].(string)
		ownerBlockchain, _ := localFile["ownerBlockchain"].(string)
		if ownerBlockchain == "" {
			ownerBlockchain = viewerBlockchain
		}
		if source == "" {
			source = "direct_upload"
		}
		finalizeLocalFileVisibility(database, viewerAddress, ownerBlockchain, requestData.CID, source, "publishedLocalCopy")
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "cid": cid})
	})
	router.POST("/files/finalize", func(c *gin.Context) {
		if gateway {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Not available in gateway mode"})
			return
		}
		viewerAddress, viewerBlockchain, authenticated := getViewerIdentity(c)
		if !authenticated {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "Authentication required"})
			return
		}
		type payload struct {
			CIDs       []string `json:"cids" binding:"required"`
			Visibility string   `json:"visibility" binding:"required"`
			Source     string   `json:"source" binding:"required"`
			TxHash     string   `json:"txHash"`
			Blockchain string   `json:"blockchain"`
		}
		var requestData payload
		if err := c.BindJSON(&requestData); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
			return
		}
		if len(requestData.CIDs) == 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "No files provided"})
			return
		}
		validSource := requestData.Source == "direct_upload" || requestData.Source == "post_attachment" || requestData.Source == "comment_attachment"
		if !validSource {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid source"})
			return
		}
		if requestData.Visibility != "public" && requestData.Visibility != "private" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid visibility"})
			return
		}
		attachments := []db.Attachment{}
		localFileOwnerChains := map[string]string{}
		for _, cid := range requestData.CIDs {
			cid = strings.TrimSpace(cid)
			if !security.IsValidCID(cid) {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid CID"})
				return
			}
			localFile := getViewerLocalFile(database, viewerAddress, viewerBlockchain, cid)
			if localFile == nil {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"status": "File not found"})
				return
			}
			ownerBlockchain, _ := localFile["ownerBlockchain"].(string)
			if ownerBlockchain == "" {
				ownerBlockchain = viewerBlockchain
			}
			localFileOwnerChains[cid] = ownerBlockchain
			mimeType, _ := localFile["mimeType"].(string)
			fileName, _ := localFile["fileName"].(string)
			sizeInt64, _ := localFile["size"].(int64)
			attachments = append(attachments, db.Attachment{
				CID:      cid,
				MimeType: mimeType,
				FileSize: uint64(sizeInt64),
				FileName: fileName,
			})
		}
		if requestData.Visibility == "private" {
			for _, cid := range requestData.CIDs {
				finalizeLocalFileVisibility(database, viewerAddress, localFileOwnerChains[cid], cid, requestData.Source, "private")
			}
			c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
			return
		}
		if !security.IsValidBlockchain(requestData.Blockchain) || !security.IsValidTxHash(requestData.TxHash, requestData.Blockchain) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid blockchain or transaction hash"})
			return
		}
		if requestData.Blockchain != viewerBlockchain {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Authenticated chain does not match requested chain"})
			return
		}
		database.OnchainFilesUpsert(requestData.TxHash, requestData.Blockchain, viewerAddress, requestData.Source, uint64(time.Now().Unix()), attachments)
		for _, cid := range requestData.CIDs {
			finalizeLocalFileVisibility(database, viewerAddress, localFileOwnerChains[cid], cid, requestData.Source, "publishedLocalCopy")
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
	})
	router.POST("/files/rename/prepare", func(c *gin.Context) {
		if gateway {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Not available in gateway mode"})
			return
		}
		viewerAddress, viewerBlockchain, authenticated := getViewerIdentity(c)
		if !authenticated {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "Authentication required"})
			return
		}
		type payload struct {
			CID          string `json:"cid" binding:"required"`
			FileNameBase string `json:"fileNameBase" binding:"required"`
		}
		var requestData payload
		if err := c.BindJSON(&requestData); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
			return
		}
		requestData.CID = strings.TrimSpace(requestData.CID)
		if !security.IsValidCID(requestData.CID) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid CID"})
			return
		}
		localFile := getViewerLocalFile(database, viewerAddress, viewerBlockchain, requestData.CID)
		if localFile == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"status": "File not found"})
			return
		}
		currentFileName, _ := localFile["fileName"].(string)
		renamedFileName := buildRenamedFileName(currentFileName, requestData.FileNameBase)
		if renamedFileName == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid file name"})
			return
		}
		if renamedFileName == currentFileName {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "File name is unchanged"})
			return
		}
		state, _ := localFile["state"].(string)
		if state != "private" && state != "publishedLocalCopy" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Only private and published files can be renamed"})
			return
		}
		if state == "publishedLocalCopy" && !canRenamePublishedLocalFile(localFile) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Published attachments cannot be renamed"})
			return
		}
		publishCID := requestData.CID
		if state == "publishedLocalCopy" {
			if ipfs == nil {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"status": "IPFS not available"})
				return
			}
			tempFilePath, cleanup, err := createRenamedTempCopy(database, localFile, renamedFileName)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Failed to prepare renamed file"})
				return
			}
			defer cleanup()
			publishCID, err = ipfs.IPFSHashFile(tempFilePath)
			if err != nil || !security.IsValidCID(publishCID) {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Failed to prepare renamed file"})
				return
			}
		}
		mimeType, _ := localFile["mimeType"].(string)
		size, _ := localFile["size"].(int64)
		c.SecureJSON(http.StatusOK, gin.H{
			"status":   "success",
			"cid":      publishCID,
			"fileName": renamedFileName,
			"mimeType": mimeType,
			"size":     size,
		})
	})
	router.POST("/files/rename", func(c *gin.Context) {
		if gateway {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Not available in gateway mode"})
			return
		}
		viewerAddress, viewerBlockchain, authenticated := getViewerIdentity(c)
		if !authenticated {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "Authentication required"})
			return
		}
		type payload struct {
			CID           string `json:"cid" binding:"required"`
			FileNameBase  string `json:"fileNameBase" binding:"required"`
			PublishCID    string `json:"publishCid"`
			DeleteTxHash  string `json:"deleteTxHash"`
			PublishTxHash string `json:"publishTxHash"`
			Blockchain    string `json:"blockchain"`
		}
		var requestData payload
		if err := c.BindJSON(&requestData); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
			return
		}
		requestData.CID = strings.TrimSpace(requestData.CID)
		if !security.IsValidCID(requestData.CID) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid CID"})
			return
		}
		localFile := getViewerLocalFile(database, viewerAddress, viewerBlockchain, requestData.CID)
		if localFile == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"status": "File not found"})
			return
		}
		currentFileName, _ := localFile["fileName"].(string)
		renamedFileName := buildRenamedFileName(currentFileName, requestData.FileNameBase)
		if renamedFileName == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid file name"})
			return
		}
		if renamedFileName == currentFileName {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "File name is unchanged"})
			return
		}
		state, _ := localFile["state"].(string)
		if state == "private" {
			upsertLocalFileRecord(database, localFile, requestData.CID, renamedFileName)
			c.SecureJSON(http.StatusOK, gin.H{"status": "success", "cid": requestData.CID, "fileName": renamedFileName})
			return
		}
		if state != "publishedLocalCopy" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Only private and published files can be renamed"})
			return
		}
		if !canRenamePublishedLocalFile(localFile) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Published attachments cannot be renamed"})
			return
		}
		if ipfs == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"status": "IPFS not available"})
			return
		}
		requestData.PublishCID = strings.TrimSpace(requestData.PublishCID)
		requestData.DeleteTxHash = strings.TrimSpace(requestData.DeleteTxHash)
		requestData.PublishTxHash = strings.TrimSpace(requestData.PublishTxHash)
		requestData.Blockchain = strings.TrimSpace(requestData.Blockchain)
		if !security.IsValidCID(requestData.PublishCID) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid publish CID"})
			return
		}
		if !security.IsValidBlockchain(requestData.Blockchain) ||
			!security.IsValidTxHash(requestData.DeleteTxHash, requestData.Blockchain) ||
			!security.IsValidTxHash(requestData.PublishTxHash, requestData.Blockchain) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Public file rename requires valid blockchain transactions"})
			return
		}
		if requestData.Blockchain != viewerBlockchain {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Authenticated chain does not match requested chain"})
			return
		}
		tempFilePath, cleanup, err := createRenamedTempCopy(database, localFile, renamedFileName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Failed to rename file"})
			return
		}
		defer cleanup()
		reseededCID, err := ipfs.IPFSAddFile(tempFilePath)
		if err != nil || !security.IsValidCID(reseededCID) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Failed to reseed renamed file"})
			return
		}
		if reseededCID != requestData.PublishCID {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"status": "Renamed file CID changed unexpectedly"})
			return
		}
		removeLocalFileMFSPaths(database, ipfs, localFile)
		if requestData.CID != reseededCID {
			_ = ipfs.IPFSUnpinFile(requestData.CID)
			_ = ipfs.IPFSGarbageCollect()
		}
		upsertLocalFileRecord(database, localFile, reseededCID, renamedFileName)
		if reseededCID != requestData.CID {
			ownerAddress, _ := localFile["ownerAddress"].(string)
			ownerBlockchain, _ := localFile["ownerBlockchain"].(string)
			database.LocalFileDelete(ownerAddress, ownerBlockchain, requestData.CID)
		}
		mimeType, _ := localFile["mimeType"].(string)
		source, _ := localFile["source"].(string)
		size, _ := localFile["size"].(int64)
		if source == "" {
			source = "direct_upload"
		}
		database.OnchainPFD(requestData.DeleteTxHash, requestData.Blockchain, viewerAddress, uint64(time.Now().Unix()), []string{requestData.CID})
		database.OnchainFilesUpsert(requestData.PublishTxHash, requestData.Blockchain, viewerAddress, source, uint64(time.Now().Unix()), []db.Attachment{{
			CID:      reseededCID,
			MimeType: mimeType,
			FileSize: uint64(size),
			FileName: renamedFileName,
		}})
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "cid": reseededCID, "fileName": renamedFileName})
	})
	router.POST("/files/delete", func(c *gin.Context) {
		if gateway {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Not available in gateway mode"})
			return
		}
		viewerAddress, viewerBlockchain, authenticated := getViewerIdentity(c)
		if !authenticated {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "Authentication required"})
			return
		}
		type payload struct {
			CID        string `json:"cid" binding:"required"`
			TxHash     string `json:"txHash"`
			Blockchain string `json:"blockchain"`
		}
		var requestData payload
		if err := c.BindJSON(&requestData); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
			return
		}
		requestData.CID = strings.TrimSpace(requestData.CID)
		if !security.IsValidCID(requestData.CID) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid CID"})
			return
		}
		localFile := getViewerLocalFile(database, viewerAddress, viewerBlockchain, requestData.CID)
		if localFile == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"status": "File not found"})
			return
		}
		localFileState, _ := localFile["state"].(string)
		if localFileState == "publishedLocalCopy" {
			requestData.TxHash = strings.TrimSpace(requestData.TxHash)
			requestData.Blockchain = strings.TrimSpace(requestData.Blockchain)
			if !security.IsValidBlockchain(requestData.Blockchain) || !security.IsValidTxHash(requestData.TxHash, requestData.Blockchain) {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Public file deletion requires a valid blockchain transaction"})
				return
			}
			if requestData.Blockchain != viewerBlockchain {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Authenticated chain does not match requested chain"})
				return
			}
			database.OnchainPFD(requestData.TxHash, requestData.Blockchain, viewerAddress, uint64(time.Now().Unix()), []string{requestData.CID})
		}
		deleteLocalFileResources(database, ipfs, localFile)
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "cid": requestData.CID})
	})
	router.POST("/posts/local", func(c *gin.Context) {
		if gateway {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "Not available in gateway mode"})
			return
		}
		viewerAddress, viewerBlockchain, authenticated := getViewerIdentity(c)
		if !authenticated {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "Authentication required"})
			return
		}
		type payload struct {
			Payload     string   `json:"payload" binding:"required"`
			Attachments []string `json:"attachments"`
		}
		var requestData payload
		if err := c.BindJSON(&requestData); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
			return
		}
		validAttachments := make([]string, 0, len(requestData.Attachments))
		for _, cid := range requestData.Attachments {
			cid = strings.TrimSpace(cid)
			if !security.IsValidCID(cid) {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": "Invalid CID"})
				return
			}
			if getViewerLocalFile(database, viewerAddress, viewerBlockchain, cid) == nil {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"status": "File not found"})
				return
			}
			validAttachments = append(validAttachments, cid)
		}
		localPostUUID := database.LocalPostCreate(viewerAddress, viewerBlockchain, security.SanitizeNonPrintable(requestData.Payload), validAttachments)
		if localPostUUID == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "Failed to create local post"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "localPostUUID": localPostUUID})
	})
}
