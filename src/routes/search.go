package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/db/blockchain"
	"YourPlace/src/core/security"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func SearchRoutes(router *gin.Engine, database *db.Database, _blockchain *blockchain.Blockchain) {
	router.GET("/s/", func(c *gin.Context) {
		query := c.Query("q")
		printableQuery := security.SanitizeNonPrintable(query)
		noWhitespace := strings.TrimSpace(printableQuery)
		tokens := strings.Fields(noWhitespace)
		var address string
		if tokens[0] == "profile:" { // profile specific search to demonstrate tokenization
			valid, chain := security.IsValidENSName(tokens[1])
			if valid {
				var err error
				address, err = blockchain.WalletGetAddress(chain, tokens[1], _blockchain)
				if err != nil {
					c.SecureJSON(http.StatusOK, gin.H{"status": "can't get address"})
					return
				}
			}
			var profile string
			if address != "" {
				profile = address
			} else {
				profile = tokens[1]
			}
			results := database.SearchGetProfiles(profile)
			c.SecureJSON(http.StatusOK, gin.H{"results": results})
			return
		} else {
			valid, chain := security.IsValidENSName(printableQuery)
			if valid {
				address, _ = blockchain.WalletGetAddress(chain, printableQuery, _blockchain)
			}
			profileQuery := printableQuery
			if address != "" {
				profileQuery = address
			}
			posts := database.SearchGetPosts(printableQuery)
			profiles := database.SearchGetProfiles(profileQuery)
			results := append(posts, profiles...)
			c.SecureJSON(http.StatusOK, gin.H{"results": results})

		}

	})
}
