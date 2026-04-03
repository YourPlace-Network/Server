package network

import (
	"YourPlace/src/core/security"
	"html"
	"regexp"
	"strings"
)

var ogMetaRegex = regexp.MustCompile(`(?i)<meta[^>]+(?:property|name)=["'](og:[^"']+|article:author)["'][^>]+content=["']([^"']*)["'][^>]*/?>|<meta[^>]+content=["']([^"']*)["'][^>]+(?:property|name)=["'](og:[^"']+|article:author)["'][^>]*/?>`)

func ParseOpenGraphTags(pageHtml string) map[string]interface{} {
	matches := ogMetaRegex.FindAllStringSubmatch(pageHtml, -1)
	if len(matches) == 0 {
		return nil
	}
	ogTags := map[string]string{}
	for _, match := range matches {
		var property, content string
		if match[1] != "" {
			property = strings.ToLower(match[1])
			content = match[2]
		} else if match[4] != "" {
			property = strings.ToLower(match[4])
			content = match[3]
		}
		if property != "" && content != "" {
			ogTags[property] = html.UnescapeString(content)
		}
	}
	if ogTags["og:title"] == "" && ogTags["og:image"] == "" {
		return nil
	}
	filtered := map[string]interface{}{}
	if v := ogTags["article:author"]; v != "" {
		filtered["author_name"] = v
	}
	if v := ogTags["og:description"]; v != "" {
		filtered["description"] = v
	}
	if v := ogTags["og:site_name"]; v != "" {
		filtered["provider_name"] = v
	}
	if v := ogTags["og:image"]; v != "" && security.IsValidURL(v) {
		filtered["thumbnail_url"] = v
	}
	if v := ogTags["og:title"]; v != "" {
		filtered["title"] = v
	}
	if v := ogTags["og:type"]; v != "" {
		filtered["type"] = v
	}
	return filtered
}
