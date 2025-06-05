package security

import (
	"html"
	"strings"
)

func HtmlEscape(payload string) string {
	return html.EscapeString(payload)
}

func StripXssChars(payload string) string {
	// More comprehensive XSS character filtering
	metaCharacters := []string{
		"<", ">", "\"", "'", "&",
		"javascript:", "data:", "vbscript:", "onload=", "onerror=",
		"onclick=", "onmouseover=", "onfocus=", "onblur=", "onchange=",
		"onsubmit=", "onreset=", "onselect=", "onunload=", "onbeforeunload=",
		"eval(", "alert(", "confirm(", "prompt(", "document.cookie",
		"document.write", "innerHTML", "outerHTML", "insertAdjacentHTML",
		"setTimeout(", "setInterval(", "Function(", "constructor",
		"__proto__", "prototype",
	}

	result := payload
	for _, char := range metaCharacters {
		result = strings.ReplaceAll(result, char, "")
		result = strings.ReplaceAll(result, strings.ToUpper(char), "")
		result = strings.ReplaceAll(result, strings.ToLower(char), "")
	}

	// Remove null bytes and control characters
	result = strings.ReplaceAll(result, "\x00", "")
	result = strings.Map(func(r rune) rune {
		if r < 32 && r != 9 && r != 10 && r != 13 { // Keep tab, newline, carriage return
			return -1
		}
		return r
	}, result)

	return result
}
