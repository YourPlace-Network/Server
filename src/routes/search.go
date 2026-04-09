package routes

import (
	blockchain2 "YourPlace/src/core/blockchain"
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func SearchRoutes(router *gin.Engine, database *db.Database, _blockchain *blockchain2.Blockchain) {
	router.GET("/discover/random", func(c *gin.Context) {
		randomProfiles := database.DiscoverGetRandomProfiles(5)
		c.SecureJSON(http.StatusOK, gin.H{
			"random": randomProfiles,
		})
	})
	router.GET("/discover", func(c *gin.Context) {
		randomProfiles := database.DiscoverGetRandomProfiles(5)
		topByFollowers := database.DiscoverGetTopByFollowers(5)
		topByPosts := database.DiscoverGetTopByPosts(5)
		c.SecureJSON(http.StatusOK, gin.H{
			"random":      randomProfiles,
			"byFollowers": topByFollowers,
			"byPosts":     topByPosts,
		})
	})
	router.GET("/s", func(c *gin.Context) {
		query := c.Query("q")
		if len(query) < 3 {
			c.SecureJSON(http.StatusBadRequest, gin.H{"status": "query too short"})
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
		limitStr := c.DefaultQuery("limit", "26")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 26 {
			limit = 26
		}
		offsetStr := c.DefaultQuery("offset", "0")
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			offset = 0
		}
		tokens := strings.Fields(noWhitespace)
		var address string
		if tokens[0] == "profile:" {
			valid, chain := security.IsValidENSName(tokens[1])
			if valid {
				var err error
				address, err = blockchain2.WalletGetAddress(chain, tokens[1], _blockchain)
				if err != nil {
					c.SecureJSON(http.StatusOK, gin.H{"status": "can't get address"})
					return
				}
			}
			if address != "" {
				database.OnchainMN(chain, address, tokens[1], uint64(time.Now().Unix()))
			}
			var profile string
			if address != "" {
				profile = address
			} else {
				profile = tokens[1]
			}
			results := database.SearchGetProfiles(profile, 50, 0)
			c.SecureJSON(http.StatusOK, gin.H{"profiles": results, "posts": []map[string]interface{}{}, "hasMorePosts": false})
			return
		}
		valid, chain := security.IsValidENSName(printableQuery)
		if valid {
			address, _ = blockchain2.WalletGetAddress(chain, printableQuery, _blockchain)
			if address != "" {
				database.OnchainMN(chain, address, printableQuery, uint64(time.Now().Unix()))
			}
			posts := database.SearchGetPosts(printableQuery, limit, offset)
			hasMorePosts := len(posts) >= limit
			if hasMorePosts {
				posts = posts[:limit-1]
			}
			var profiles []map[string]interface{}
			if address != "" {
				profiles = database.SearchGetProfiles(address, 50, 0)
			} else {
				profiles = database.SearchGetProfiles(printableQuery, 50, 0)
			}
			c.SecureJSON(http.StatusOK, gin.H{
				"profiles":     profiles,
				"posts":        posts,
				"hasMorePosts": hasMorePosts,
			})
			return
		}
		posts := database.SearchGetPosts(printableQuery, limit, offset)
		hasMorePosts := len(posts) >= limit
		if hasMorePosts {
			posts = posts[:limit-1]
		}
		profiles := database.SearchGetProfiles(printableQuery, 50, 0)
		seen := make(map[string]bool)
		var dedupedProfiles []map[string]interface{}
		for _, p := range profiles {
			key := p["blockchain"].(string) + p["address"].(string)
			if !seen[key] {
				seen[key] = true
				dedupedProfiles = append(dedupedProfiles, p)
			}
		}
		ensSuffixes := []string{".algo", ".base.eth", ".eth"}
		if !strings.Contains(printableQuery, ".") {
			for _, suffix := range ensSuffixes {
				ensName := strings.ToLower(printableQuery) + suffix
				valid, chain := security.IsValidENSName(ensName)
				if valid {
					ensAddress, err := blockchain2.WalletGetAddress(chain, ensName, _blockchain)
					if err == nil && ensAddress != "" {
						database.OnchainMN(chain, ensAddress, ensName, uint64(time.Now().Unix()))
						ensProfiles := database.SearchGetProfiles(ensAddress, 50, 0)
						for _, ep := range ensProfiles {
							key := ep["blockchain"].(string) + ep["address"].(string)
							if !seen[key] {
								seen[key] = true
								dedupedProfiles = append(dedupedProfiles, ep)
							}
						}
					}
				}
			}
		}
		c.SecureJSON(http.StatusOK, gin.H{
			"profiles":     dedupedProfiles,
			"posts":        posts,
			"hasMorePosts": hasMorePosts,
		})
	})
}
