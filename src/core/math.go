package core

import (
	"regexp"
	"strconv"
)

func BytesToGigabytes(bytes uint64) uint64 {
	return bytes / 1024 / 1024 / 1024
}
func ParseVersionString(version string) (int, int, int) {
	// Use regex to extract version numbers, ignoring any extra characters
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(version)
	// If no matches found, return zeros
	if len(matches) != 4 {
		return 0, 0, 0
	}
	// Parse the captured groups
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, 0
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, 0, 0
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return 0, 0, 0
	}
	return major, minor, patch
}
func CompareVersionString(version1, version2 string) bool {
	// compare if version1 is greater than version2
	major1, minor1, patch1 := ParseVersionString(version1)
	major2, minor2, patch2 := ParseVersionString(version2)
	if major1 > major2 {
		return true
	} else if major1 == major2 {
		if minor1 > minor2 {
			return true
		} else if minor1 == minor2 {
			if patch1 > patch2 {
				return true
			}
		}
	}
	return false
}
