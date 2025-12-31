package security

import "strings"

func MaskDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	atIndex := strings.Index(dsn, "@")
	if atIndex == -1 {
		return dsn
	}
	userPassPart := dsn[:atIndex]
	hostPart := dsn[atIndex:]
	colonIndex := strings.Index(userPassPart, ":")
	if colonIndex == -1 {
		return dsn
	}
	user := userPassPart[:colonIndex]
	return user + ":********" + hostPart
}
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
