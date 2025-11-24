package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/db/blockchain"
	"YourPlace/src/core/security"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func SearchRoutes(router *gin.Engine, database *db.Database, _blockchain *blockchain.Blockchain) {
	router.GET("/s", func(c *gin.Context) {
		query := c.Query("q")
		if len(query) == 0 {
			c.SecureJSON(http.StatusBadRequest, gin.H{"status": "no query provided"})
			return
		}
		if len(query) > 250 {
			c.SecureJSON(http.StatusBadRequest, gin.H{"status": "query too long"})
			return
		}
		printableQuery := security.SanitizeNonPrintable(query)
		printableQuery = security.StripXssChars(printableQuery)
		noWhitespace := strings.TrimSpace(printableQuery)
		if len(noWhitespace) == 0 {
			c.SecureJSON(http.StatusBadRequest, gin.H{"status": "query only contains invalid characters - cut it out, buster"})
			return
		}
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
			ensSuffixes := []string{".base.eth"}
			if !strings.Contains(printableQuery, ".") {
				for _, suffix := range ensSuffixes {
					ensName := strings.ToLower(printableQuery) + suffix
					valid, chain := security.IsValidENSName(ensName)
					if valid {
						ensAddress, err := blockchain.WalletGetAddress(chain, ensName, _blockchain)
						if err == nil && ensAddress != "" {
							ensProfiles := database.SearchGetProfiles(ensAddress)
							profiles = append(profiles, ensProfiles...)
						}
					}
				}
			}
			results := append(posts, profiles...)
			c.SecureJSON(http.StatusOK, gin.H{"results": results})
		}
	})
}
