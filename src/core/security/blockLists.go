package security

/*func GetBlockLists(database *db.Bolt) []string {
	blockedAddresses := database.GetBucketValues("blockedAddresses")
	blockedPosts := database.GetBucketValues("blockedPosts")
	blockedRegex := database.GetBucketValues("blockedRegex")
	var blockLists []string
	for _, address := range blockedAddresses {
		blockLists = append(blockLists, address.Key)
	}
	for _, post := range blockedPosts {
		blockLists = append(blockLists, post.Key)
	}
	for _, regex := range blockedRegex {
		blockLists = append(blockLists, regex.Key)
	}
	return blockLists
}
func UpdateBlockLists(database *db.Bolt) {
	blockLists := GetBlockLists(database)
	for _, list := range blockLists {

	}
}
func IsAddressBlocked(database *db.Bolt, address string) bool {
	return false
}
func IsPostHashBlocked(database *db.Bolt, hash string) bool {
	return false
}
func IsPostContainsBlockedRegex(database *db.Bolt, blockedRegex []string) bool {
	return false
}
func IsStringBlocked(database *db.Bolt, input string) bool {
	unleetStrings := SanitizeLeet(input)
	for _, str := range unleetStrings {
		_ = str // todo
	}

	return false
}*/
