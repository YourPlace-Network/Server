package security

import "regexp"

func MaskToken(input string) string {
	runeSlice := []rune(input)
	length := len(runeSlice)
	if length <= 15 {
		return "**********"
	}
	firstSlice := string(runeSlice[:3])
	lastSlice := string(runeSlice[length-3:])
	return firstSlice + "**********" + lastSlice
}

func IsMaskedToken(input string) bool {
	pattern := "^.{3}?\\*{10}.{3}?$"
	match, _ := regexp.MatchString(pattern, input)
	return match
}
