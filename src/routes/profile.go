package routes

import (
	"YourPlace/src/core"
	blockchain2 "YourPlace/src/core/blockchain"
	"YourPlace/src/core/db"
	"YourPlace/src/core/middleware"
	"YourPlace/src/core/security"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func ProfileRoutes(router *gin.Engine, title string, database *db.Database, _blockchain *blockchain2.Blockchain, gateway bool) {
	router.GET("/p/*path", func(c *gin.Context) {
		path := strings.TrimPrefix(c.Param("path"), "/")
		if path == "" {
			value, exists1 := c.Get("blockchain")
			blockchainParam := value.(string)
			if !security.IsValidBlockchain(blockchainParam) {
				core.LogDebug("Invalid blockchain parameter in profile route")
				c.Redirect(http.StatusFound, "/login?redirect=/p/")
				return
			}
			value, exists2 := c.Get("accountAddress")
			addressParam := value.(string)
			if !security.IsValidAddress(addressParam, blockchainParam) {
				core.LogDebug("Invalid address parameter in profile route")
				c.Redirect(http.StatusFound, "/login?redirect=/p/")
				return
			}
			if exists1 && exists2 {
				core.LogDebug("Redirecting to profile with blockchain and address parameters")
				c.Redirect(http.StatusFound, "/p/"+blockchainParam+"/"+addressParam)
				return
			} else {
				core.LogDebug("No blockchain or address parameters found in profile route")
				c.Redirect(http.StatusFound, "/login?redirect=/p/")
				return
			}
		}
		segments := strings.Split(path, "/")
		var blockchainParam string
		var addressParam string

		if len(segments) == 1 {
			name := strings.ToLower(segments[0])
			var valid bool
			var err error
			valid, blockchainParam = security.IsValidENSName(name)
			addressParam, err = blockchain2.WalletGetAddress(blockchainParam, name, _blockchain)
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
		token := middleware.GetCSRFToken(c)
		ipfsGateway := getConfiguredIPFSGateway(database)
		// Check if the profile viewer is the profile owner
		isGuest := true
		userAddress := ""
		userBlockchain := ""
		viewerAddress, addressOk := c.Get("accountAddress")
		viewerBlockchain, blockchainOk := c.Get("blockchain")
		if addressOk && blockchainOk {
			userAddress = viewerAddress.(string)
			userBlockchain = viewerBlockchain.(string)
			if (viewerAddress == addressParam) && (viewerBlockchain == blockchainParam) {
				isGuest = false
			}
		}
		if !database.ProfileIsEnsDataFresh(addressParam, blockchainParam) {
			ensName, _ := blockchain2.WalletGetName(blockchainParam, addressParam, _blockchain)
			ensAvatar, _ := blockchain2.WalletGetAvatar(blockchainParam, addressParam, _blockchain)
			database.ProfileUpdateEnsData(addressParam, blockchainParam, ensName, ensAvatar)
		}
		profileName := database.ProfileGetName(addressParam, blockchainParam)
		profileDescription := database.ProfileGetDescription(addressParam, blockchainParam)
		profileAvatar := database.ProfileGetAvatar(addressParam, blockchainParam)
		if profileAvatar == "" {
			profileAvatar = database.ProfileGetEnsAvatar(addressParam, blockchainParam)
		}
		profileColorsJSON := database.ProfileGetColors(addressParam, blockchainParam)
		var profileColors map[string]string
		if profileColorsJSON != "" {
			json.Unmarshal([]byte(profileColorsJSON), &profileColors)
		}
		displayName := profileName
		if displayName == "" {
			displayName = database.ProfileGetEnsName(addressParam, blockchainParam)
		}
		if displayName == "" {
			displayName = addressParam[:6] + "..." + addressParam[len(addressParam)-4:]
		}
		gatewayMintEnabled := database.MetaGetValue("gatewayMintEnabled")
		pageTitle := displayName + " | " + title
		responseJson := gin.H{
			"title":                 pageTitle,
			"csrfToken":             token,
			"pageName":              "profile",
			"ipfsGateway":           ipfsGateway,
			"injectedAddress":       addressParam,
			"injectedBlockchain":    blockchainParam,
			"isCookieAuthenticated": true,
			"isGuest":               isGuest, // Guest mode distinguishes if the viewer is the guest or owner of the profile
			"gatewayMintEnabled":    gatewayMintEnabled == "true",
			"gatewayMode":           gateway,
			"userAddress":           userAddress,
			"userBlockchain":        userBlockchain,
			"ogTitle":               displayName,
			"ogDescription":         profileDescription,
			"ogImage":               profileAvatar,
			"ogType":                "profile",
			"profileColors":         profileColors,
		}
		c.HTML(http.StatusOK, "src/templates/pages/profile.tmpl", responseJson)
	})
	router.GET("/profile/data/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		avatarAddress := database.ProfileGetAvatar(address, blockchainParam)
		ensAvatar := database.ProfileGetEnsAvatar(address, blockchainParam)
		if avatarAddress == "" && ensAvatar != "" {
			avatarAddress = ensAvatar
		}
		if avatarAddress == "" {
			onChainAvatar, err := blockchain2.WalletGetAvatar(blockchainParam, address, _blockchain)
			if err == nil && onChainAvatar != "" {
				avatarAddress = onChainAvatar
			}
		}
		profileData := gin.H{
			"avatarAddress":  avatarAddress,
			"bannerAddress":  database.ProfileGetBanner(address, blockchainParam),
			"bot":            database.ProfileGetBot(address, blockchainParam),
			"description":    database.ProfileGetDescription(address, blockchainParam),
			"ensAvatar":      ensAvatar,
			"ensName":        database.ProfileGetEnsName(address, blockchainParam),
			"followerCount":  database.ProfileGetFollowerCount(address, blockchainParam),
			"followingCount": database.ProfileGetFollowingCount(address, blockchainParam),
			"joinedDate":     database.ProfileGetJoinedDate(address, blockchainParam),
			"location":       database.ProfileGetLocation(address, blockchainParam),
			"musicEmbed":     database.ProfileGetMusicEmbed(address, blockchainParam),
			"name":           database.ProfileGetName(address, blockchainParam),
			"nsfw":           database.ProfileGetNsfw(address, blockchainParam),
			"vertical":       database.ProfileGetVertical(address, blockchainParam),
			"website":        database.ProfileGetWebsite(address, blockchainParam),
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "profileData": profileData})
	})
	router.GET("/profile/details/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		balance, symbol := blockchain2.WalletGetBalanceFormatted(blockchainParam, address, _blockchain)
		priceUSD, _ := blockchain2.WalletGetPriceUSD(blockchainParam, _blockchain)
		balanceUSD := balance * priceUSD
		c.SecureJSON(http.StatusOK, gin.H{
			"balance":    balance,
			"balanceUSD": balanceUSD,
			"priceUSD":   priceUSD,
			"symbol":     symbol,
		})
	})
	router.GET("/profile/name/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		name := database.ProfileGetName(address, blockchainParam)
		if name == "" {
			name = database.ProfileGetEnsName(address, blockchainParam)
		}
		if name == "" {
			name, _ = blockchain2.WalletGetName(blockchainParam, address, _blockchain)
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "name": name})
	})
	router.GET("/profile/avatar/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		avatarAddress := database.ProfileGetAvatar(address, blockchainParam)
		if avatarAddress == "" {
			avatarAddress = database.ProfileGetEnsAvatar(address, blockchainParam)
		}
		if avatarAddress == "" {
			onChainAvatar, err := blockchain2.WalletGetAvatar(blockchainParam, address, _blockchain)
			if err == nil && onChainAvatar != "" {
				avatarAddress = onChainAvatar
			}
		}
		if avatarAddress == "" {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no avatar found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "avatarAddress": avatarAddress})
	})
	router.GET("/profile/banner/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		bannerAddress := database.ProfileGetBanner(address, blockchainParam)
		if bannerAddress == "" {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no banner found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "bannerAddress": bannerAddress})
	})
	router.GET("/profile/description/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		description := database.ProfileGetDescription(address, blockchainParam)
		if description == "" {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no description found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"description": description})
	})
	router.GET("/profile/location/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		location := database.ProfileGetLocation(address, blockchainParam)
		if location == "" {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no location found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"location": location})
	})
	router.GET("/profile/bot/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		bot := database.ProfileGetBot(address, blockchainParam)
		c.SecureJSON(http.StatusOK, gin.H{"bot": bot})
	})
	router.GET("/profile/nsfw/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		nsfw := database.ProfileGetNsfw(address, blockchainParam)
		c.SecureJSON(http.StatusOK, gin.H{"nsfw": nsfw})
	})
	router.GET("/profile/vertical/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		vertical := database.ProfileGetVertical(address, blockchainParam)
		if vertical == "" {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no vertical found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"vertical": vertical})
	})
	router.GET("/profile/joinedDate/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		joinedDate := database.ProfileGetJoinedDate(address, blockchainParam)
		if joinedDate == nil {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no joined date found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"joinedDate": joinedDate})
	})
	router.GET("/profile/website/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		website := database.ProfileGetWebsite(address, blockchainParam)
		if website == "" {
			c.SecureJSON(http.StatusOK, gin.H{"status": "no website found"})
			return
		}
		c.SecureJSON(http.StatusOK, gin.H{"website": website})
	})
	router.GET("/profile/followerCount/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		followerCount := database.ProfileGetFollowerCount(address, blockchainParam)
		c.SecureJSON(http.StatusOK, gin.H{"followerCount": followerCount})
	})
	router.GET("/profile/isFollower/:toBlockchain/:toAddress/:fromBlockchain/:fromAddress", func(c *gin.Context) {
		toBlockchain := c.Param("toBlockchain")
		if !security.IsValidBlockchain(toBlockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		toAddress := c.Param("toAddress")
		if !security.IsValidAddress(toAddress, toBlockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		fromBlockchain := c.Param("fromBlockchain")
		if !security.IsValidBlockchain(fromBlockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid follower blockchain"})
			return
		}
		fromAddress := c.Param("fromAddress")
		if !security.IsValidAddress(fromAddress, fromBlockchain) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid follower address"})
			return
		}
		isFollower := database.ProfileIsFollower(toAddress, toBlockchain, fromAddress, fromBlockchain)
		c.SecureJSON(http.StatusOK, gin.H{"isFollower": isFollower})
	})
	router.GET("/profile/colors/:blockchain/:address", func(c *gin.Context) {
		blockchainParam := c.Param("blockchain")
		if !security.IsValidBlockchain(blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid blockchain"})
			return
		}
		address := c.Param("address")
		if !security.IsValidAddress(address, blockchainParam) {
			c.SecureJSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
			return
		}
		colorsJSON := database.ProfileGetColors(address, blockchainParam)
		var colors map[string]string
		if colorsJSON != "" {
			json.Unmarshal([]byte(colorsJSON), &colors)
		}
		if colors == nil {
			colors = make(map[string]string)
		}
		c.SecureJSON(http.StatusOK, gin.H{"colors": colors})
	})
}
