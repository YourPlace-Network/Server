package security

import (
	"html"
	"strings"
)

func HtmlEscape(payload string) string {
	return html.EscapeString(payload)
}

func StripXssChars(payload string) string {
	metaCharacters := [2]string{">", "<"}
	var result string
	for i := 0; i < len(metaCharacters); i++ {
		result = strings.ReplaceAll(payload, metaCharacters[i], "")
	}
	return result
}
