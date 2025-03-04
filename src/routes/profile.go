package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/db/blockchain"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"net/http"
	"strings"
)

func ProfileRoutes(router *gin.Engine, title string, database *db.Database, _blockchain *blockchain.Blockchain, cryptoSeed []byte) {
	/* Need to think through the security implications of middleware path detection and user-controllable top-level path routing
	router.GET("/:address", func(c *gin.Context) {
		address := c.Param("address")
		core.LogDebug("ProfileRoutes(): " + address)

		if strings.HasSuffix(address, ".base.eth") { // Check for Base ENS name
			core.LogDebug("ProfileRoutes(): Base ENS name")
			resolvedAddresses, err := blockchain.WalletGetAddress("base", address, _blockchain) // Check for Base ENS name
			core.LogDebug("ProfileRoutes(): resolvedAddresses: " + resolvedAddresses)
			if err == nil && security.IsValidAddress(resolvedAddresses, "base") {
				c.Redirect(http.StatusFound, "/p/base/"+resolvedAddresses) // Base ENS name exists
				return
			}
		}

		if strings.HasSuffix(address, ".eth") { // Check for ENS name
			resolvedAddresses, err := blockchain.WalletGetAddress("eth", address, _blockchain) // Check for ENS name
			if err == nil && security.IsValidAddress(resolvedAddresses, "eth") {
				c.Redirect(http.StatusFound, "/p/eth/"+resolvedAddresses) // ENS name exists
				return
			}
		}

		if security.IsValidAddress(address, "eth") { // Check for Ethereum address
			c.Redirect(http.StatusFound, "/p/eth/"+address)
			return
		} else {
			c.Redirect(http.StatusFound, "/p/")
			return
		}
	})*/
	router.GET("/p/*path", func(c *gin.Context) {
		path := strings.TrimPrefix(c.Param("path"), "/")
		if path == "" {
			blockchainparam, exists1 := c.Get("blockchain")
			address, exists2 := c.Get("accountAddress")
			if exists1 && exists2 {
				c.Redirect(http.StatusFound, "/p/"+blockchainparam.(string)+"/"+address.(string))
				return
			} else {
				c.Redirect(http.StatusFound, "/login?redirect=/p/")
				return
			}
		}
		segments := strings.Split(path, "/")
		var blockchainParam string
		var addressParam string

		if len(segments) == 1 {
			name := segments[0]
			var valid bool
			var err error
			valid, blockchainParam = security.IsValidENSName(name)
			addressParam, err = blockchain.WalletGetAddress(blockchainParam, name, _blockchain)
			if !valid || err != nil {
				c.Redirect(http.StatusSeeOther, "/404")
				return
			}
		} else if len(segments) == 2 {
			blockchainParam = segments[0]
			addressParam = segments[1]
		} else {
			c.Redirect(http.StatusSeeOther, "/404")
			return
		}
		if !security.IsValidBlockchain(blockchainParam) {
			c.Redirect(http.StatusSeeOther, "/404")
			return
		}
		if !security.IsValidAddress(addressParam, blockchainParam) {
			c.Redirect(http.StatusSeeOther, "/404")
			return
		}
		token := csrf.Token(c.Request)

		// Check if the profile viewer is the profile owner
		isGuest := true
		authCookie, err := c.Request.Cookie("yp_auth")
		if err == nil && security.ValidateCookie(authCookie, cryptoSeed, database) {
			blockchainValue, _err := security.GetCookieValue(authCookie, cryptoSeed, "blockchain", database)
			if _err == nil && security.IsValidBlockchain(blockchainValue) {
				addressValue, err2 := security.GetCookieValue(authCookie, cryptoSeed, "address", database)
				if err2 == nil && security.IsValidAddress(addressValue, blockchainValue) {
					if addressValue == addressParam && blockchainValue == blockchainParam {
						isGuest = false
					}
				}
			}
		}
		responseJson := gin.H{
			"title":                 title,
			"csrfToken":             token,
			"pageName":              "profile",
			"injectedAddress":       addressParam,
			"injectedBlockchain":    blockchainParam,
			"isCookieAuthenticated": true,
			"isGuest":               isGuest, // Guest mode distinguishes if the viewer is the guest or owner of the profile
		}
		c.HTML(http.StatusOK, "src/templates/pages/profile.tmpl", responseJson)
	})

	router.GET("/profile/name/:blockchain/:address", func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		name := database.ProfileGetName(address, blockchain)
		if name == "" {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no name found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "name": name})
	})
	router.GET("/profile/avatar/:blockchain/:address", func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		avatarAddress := database.ProfileGetAvatar(address, blockchain)
		if avatarAddress == "" {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no avatar found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "avatarAddress": avatarAddress})
	})
	router.GET("/profile/banner/:blockchain/:address", func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		bannerAddress := database.ProfileGetBanner(address, blockchain)
		if bannerAddress == "" {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no banner found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "bannerAddress": bannerAddress})
	})
	router.GET("/profile/description/:blockchain/:address", func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		description := database.ProfileGetDescription(address, blockchain)
		if description == "" {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no description found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"description": description})
	})
	router.GET("/profile/location/:blockchain/:address", func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchain) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchain) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		location := database.ProfileGetLocation(address, blockchain)
		if location == "" {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no location found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"location": location})
	})
	router.GET("/profile/birthdate/:blockchain/:address", func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		birthdate := database.ProfileGetBirthDate(address, blockchain)
		if birthdate == nil {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no birthdate found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"birthdate": birthdate})
	})
	router.GET("/profile/joineddate/:blockchain/:address", func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		joinedDate := database.ProfileGetJoinedDate(address, blockchain)
		if joinedDate == nil {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no joined date found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"joinedDate": joinedDate})
	})
	router.GET("/profile/website/:blockchain/:address", func(c *gin.Context) {
		blockchain := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		website := database.ProfileGetWebsite(address, blockchain)
		if website == "" {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no website found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"website": website})
	})
}
