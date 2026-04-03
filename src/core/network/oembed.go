package network

import (
	"YourPlace/src/core"
	"YourPlace/src/core/security"
	"html"
	"regexp"
	"strings"
)

var oembedLinkRegex = regexp.MustCompile(`(?i)<link[^>]+type=["']application/json\+oembed["'][^>]+href=["']([^"']+)["'][^>]*/?>|<link[^>]+href=["']([^"']+)["'][^>]+type=["']application/json\+oembed["'][^>]*/?>`)
var tcoLinkRegex = regexp.MustCompile(`<a[^>]*href="(https://t\.co/[a-zA-Z0-9]+)"[^>]*>[^<]*</a>`)
var XcomMediaLinkRegex = regexp.MustCompile(`<a[^>]*href="(https://(?:www\.)?(?:twitter\.com|x\.com)/[a-zA-Z0-9_]+/status/\d+/(?:photo|video)/\d+)"[^>]*>[^<]*</a>`)
var XcomMediaUrlRegex = regexp.MustCompile(`/status/(\d+)/(?:photo|video)/`)
var XcomUrlRegex = regexp.MustCompile(`^https://(?:www\.)?(twitter\.com|x\.com)/([a-zA-Z0-9_]+)/status/(\d+)/?(?:[?#].*)?$`)

var oembedFilterKeys = []string{"author_name", "author_url", "description", "provider_name", "provider_url", "thumbnail_url", "title", "type"}

func FilterOEmbedResponse(oembedResponse map[string]interface{}) map[string]interface{} {
	filtered := map[string]interface{}{}
	for _, key := range oembedFilterKeys {
		if val, ok := oembedResponse[key]; ok {
			filtered[key] = val
		}
	}
	return filtered
}
func FindOEmbedEndpoint(pageHtml string) string {
	matches := oembedLinkRegex.FindStringSubmatch(pageHtml)
	endpoint := ""
	if len(matches) > 1 {
		for _, m := range matches[1:] {
			if m != "" {
				endpoint = m
				break
			}
		}
	}
	return html.UnescapeString(endpoint)
}
func ResolveAuthorAvatar(oembedResponse map[string]interface{}, tweetUrl string) {
	urlMatch := XcomUrlRegex.FindStringSubmatch(tweetUrl)
	if urlMatch == nil {
		return
	}
	statusId := urlMatch[3]
	syndicationUrl := "https://cdn.syndication.twimg.com/tweet-result?id=" + statusId + "&token=0"
	var syndicationData map[string]interface{}
	err := HttpGetJson(syndicationUrl, &syndicationData)
	if err != nil {
		core.LogDebug("Could not fetch syndication data for avatar: " + err.Error())
		return
	}
	user, ok := syndicationData["user"].(map[string]interface{})
	if !ok {
		return
	}
	avatarUrl, _ := user["profile_image_url_https"].(string)
	if avatarUrl != "" && security.IsValidURL(avatarUrl) {
		oembedResponse["author_avatar"] = avatarUrl
	}
}
func ResolveMediaLinks(oembedResponse map[string]interface{}) {
	htmlContent, ok := oembedResponse["html"].(string)
	if !ok {
		return
	}
	matches := XcomMediaLinkRegex.FindAllStringSubmatch(htmlContent, -1)
	if len(matches) == 0 {
		return
	}
	fetchedIds := make(map[string]bool)
	var mediaUrls []map[string]string
	for _, match := range matches {
		fullTag := match[0]
		mediaPageUrl := match[1]
		htmlContent = strings.Replace(htmlContent, fullTag, "", 1)
		idMatch := XcomMediaUrlRegex.FindStringSubmatch(mediaPageUrl)
		if idMatch == nil {
			continue
		}
		statusId := idMatch[1]
		if fetchedIds[statusId] {
			continue
		}
		fetchedIds[statusId] = true
		syndicationUrl := "https://cdn.syndication.twimg.com/tweet-result?id=" + statusId + "&token=0"
		var syndicationData map[string]interface{}
		err := HttpGetJson(syndicationUrl, &syndicationData)
		if err != nil {
			core.LogDebug("Could not fetch syndication data for status " + statusId + ": " + err.Error())
			continue
		}
		if mediaDetails, ok := syndicationData["mediaDetails"].([]interface{}); ok {
			for _, item := range mediaDetails {
				detail, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if mediaType, _ := detail["type"].(string); mediaType == "photo" {
					if imageUrl, _ := detail["media_url_https"].(string); imageUrl != "" && security.IsValidURL(imageUrl) {
						mediaUrls = append(mediaUrls, map[string]string{"type": "photo", "url": imageUrl})
					}
				}
			}
		}
		if video, ok := syndicationData["video"].(map[string]interface{}); ok {
			if poster, _ := video["poster"].(string); poster != "" && security.IsValidURL(poster) {
				mediaUrls = append(mediaUrls, map[string]string{"type": "video", "url": poster})
			}
		}
	}
	oembedResponse["html"] = htmlContent
	if len(mediaUrls) > 0 {
		oembedResponse["media_urls"] = mediaUrls
	}
}
func UnwrapTcoLinks(oembedResponse map[string]interface{}) {
	htmlContent, ok := oembedResponse["html"].(string)
	if !ok {
		return
	}
	matches := tcoLinkRegex.FindAllStringSubmatch(htmlContent, -1)
	for _, match := range matches {
		fullTag := match[0]
		tcoUrl := match[1]
		resolved := HttpResolveRedirect(tcoUrl)
		if resolved == tcoUrl {
			continue
		}
		escaped := html.EscapeString(resolved)
		newTag := `<a href="` + escaped + `">` + escaped + `</a>`
		idx := strings.Index(htmlContent, fullTag)
		if idx > 0 {
			prefix := strings.TrimRight(htmlContent[:idx], " \t\n\r")
			if len(prefix) > 0 && prefix[len(prefix)-1] != '>' {
				newTag = "<br>" + newTag
			}
		}
		htmlContent = strings.Replace(htmlContent, fullTag, newTag, 1)
	}
	oembedResponse["html"] = htmlContent
}
