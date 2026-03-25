package db

// This file cannot create blockchain-specific types or packages or functions, to prevent upstream application code from becoming blockchain-dependent. This is the public database "API" wrapper to the rest of the application. Having a "blockchain" parameter to allow selection of the chain via database query logic is fine.

import (
	"YourPlace/src/core"
	"fmt"
	"os"
	"slices"
	"time"
)

type Database struct {
	mysql  MySQL
	sqlite SQLite
	Engine string
}
type Attachment struct { //we can move this as long as it isn't defined in a package that imports database
	FileURL  string
	MimeType string
	FileSize uint64
	FileName string
}

func (db *Database) Init(path string, engine string) {
	validEngines := []string{"sqlite", "mysql"}
	if !slices.Contains(validEngines, engine) {
		core.LogFatal("Invalid DB engine selected")
	}
	core.LogInfo("Initializing database with engine: " + engine)
	switch engine {
	case "sqlite":
		db.sqlite.Init(path)
	case "mysql":
		dsn := os.Getenv("YOURPLACE_MYSQL_DSN")
		if dsn == "" {
			core.LogFatal("MYSQL DSN not set in environment variable YOURPLACE_MYSQL_DSN")
		}
		db.mysql.Init(dsn)
	}
	db.Engine = engine
	// Wait for DB to be ready
	for i := 0; i < 5; i++ {
		if db.Ping() {
			break
		}
		time.Sleep(time.Second)
		if i == 4 {
			core.LogFatal("Database failed to initialize after 5 attempts")
		}
	}
}
func (db *Database) SetDefaultSettings() {
	// Pre-fills some safe default settings if they do not already exist
	settingsDefaults := map[string]string{
		"historyDays":      "90",
		"indexerOnBattery": "false",
		"indexerRunning":   "true",
		"badbitsEnabled":   "true",
	}
	for key, defaultValue := range settingsDefaults {
		existingValue := db.SettingsGetValue(key)
		if existingValue == "" {
			db.SettingsUpdateValue(key, defaultValue)
		}
	}
}
func (db *Database) SetGatewayDefaultSettings(uploadDirectory string) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	metaDefaults := map[string]string{
		"accountAddress": "0x0000000000000000000000000000000000000000",
		"accountNetwork": "base",
		"installedDate":  timestamp,
	}
	settingsDefaults := map[string]string{
		"uploadDirectory": uploadDirectory,
	}
	for key, defaultValue := range metaDefaults {
		existingValue := db.MetaGetValue(key)
		if existingValue == "" {
			db.MetaUpdateValue(key, defaultValue)
		}
	}
	for key, defaultValue := range settingsDefaults {
		existingValue := db.SettingsGetValue(key)
		if existingValue == "" {
			db.SettingsUpdateValue(key, defaultValue)
		}
	}
}
func (db *Database) ExportSnapshot(exportPath string) error {
	switch db.Engine {
	case "mysql":
		return db.mysql.ExportSnapshot(exportPath)
	case "sqlite":
		return db.sqlite.ExportSnapshot(exportPath)
	default:
		return core.LogErrorReturn("Invalid DB engine selected")
	}
}
func (db *Database) ImportSnapshot(importPath string) error {
	switch db.Engine {
	case "mysql":
		return db.mysql.ImportSnapshot(importPath)
	case "sqlite":
		return db.sqlite.ImportSnapshot(importPath)
	default:
		return core.LogErrorReturn("Invalid DB engine selected")
	}
}
func (db *Database) ImportSnapshotNoMetadata(importPath string) error {
	switch db.Engine {
	case "mysql":
		return db.mysql.ImportSnapshotNoMetadata(importPath)
	case "sqlite":
		return db.sqlite.ImportSnapshotNoMetadata(importPath)
	default:
		return core.LogErrorReturn("Invalid DB engine selected")
	}
}

// --- Metadata & Settings Functions --- //
func (db *Database) MetaUpdateValue(key string, value string) {
	switch db.Engine {
	case "mysql":
		db.mysql.MetaUpdateValue(key, value)
	case "sqlite":
		db.sqlite.MetaUpdateValue(key, value)
	}
}
func (db *Database) MetaGetValue(key string) string {
	switch db.Engine {
	case "mysql":
		return db.mysql.MetaGetValue(key)
	case "sqlite":
		return db.sqlite.MetaGetValue(key)
	}
	return ""
}
func (db *Database) SettingsGetValue(key string) string {
	switch db.Engine {
	case "mysql":
		return db.mysql.SettingsGetValue(key)
	case "sqlite":
		return db.sqlite.SettingsGetValue(key)
	}
	return ""
}
func (db *Database) SettingsUpdateValue(key string, value string) {
	switch db.Engine {
	case "mysql":
		db.mysql.SettingsUpdateValue(key, value)
	case "sqlite":
		db.sqlite.SettingsUpdateValue(key, value)
	}
}
func (db *Database) SettingsDeleteValue(key string) error {
	switch db.Engine {
	case "mysql":
		return db.mysql.SettingsDeleteValue(key)
	case "sqlite":
		return db.sqlite.SettingsDeleteValue(key)
	}
	return nil
}
func (db *Database) Ping() bool {
	switch db.Engine {
	case "mysql":
		return db.mysql.Ping()
	case "sqlite":
		return db.sqlite.Ping()
	}
	return false
}
func (db *Database) Close() error {
	switch db.Engine {
	case "mysql":
		return db.mysql.Close()
	case "sqlite":
		return db.sqlite.Close()
	}
	return nil
}

// --- Auth Functions --- //
func (db *Database) AuthGetNonceStatus(nonce string) string {
	switch db.Engine {
	case "mysql":
		return db.mysql.AuthGetNonceStatus(nonce)
	case "sqlite":
		return db.sqlite.AuthGetNonceStatus(nonce)
	}
	return ""
}
func (db *Database) AuthUpdateNonce(nonce string, status string) {
	switch db.Engine {
	case "mysql":
		db.mysql.AuthUpdateNonce(nonce, status)
	case "sqlite":
		db.sqlite.AuthUpdateNonce(nonce, status)
	}
}
func (db *Database) AuthDeleteNonce(nonce string) {
	switch db.Engine {
	case "mysql":
		db.mysql.AuthDeleteNonce(nonce)
	case "sqlite":
		db.sqlite.AuthDeleteNonce(nonce)
	}
}
func (db *Database) AuthExpireCookie(uuid string) {
	switch db.Engine {
	case "mysql":
		db.mysql.AuthExpireCookie(uuid)
	case "sqlite":
		db.sqlite.AuthExpireCookie(uuid)
	}
}
func (db *Database) AuthGetCookieStatus(uuid string) string {
	switch db.Engine {
	case "mysql":
		return db.mysql.AuthGetCookieStatus(uuid)
	case "sqlite":
		return db.sqlite.AuthGetCookieStatus(uuid)
	}
	return ""
}
func (db *Database) AuthUpdateLoginNonce(nonce string, domain string, expiration uint64, nonceHash string) {
	switch db.Engine {
	case "mysql":
		db.mysql.AuthUpdateLoginNonce(nonce, domain, expiration, nonceHash)
	case "sqlite":
		db.sqlite.AuthUpdateLoginNonce(nonce, domain, expiration, nonceHash)
	}
}
func (db *Database) AuthGetLoginNonceByHash(nonceHash string) string {
	switch db.Engine {
	case "mysql":
		return db.mysql.AuthGetLoginNonceByHash(nonceHash)
	case "sqlite":
		return db.sqlite.AuthGetLoginNonceByHash(nonceHash)
	}
	return ""
}
func (db *Database) AuthDeleteLoginNonce(nonce string) {
	switch db.Engine {
	case "mysql":
		db.mysql.AuthDeleteLoginNonce(nonce)
	case "sqlite":
		db.sqlite.AuthDeleteLoginNonce(nonce)
	}
}
func (db *Database) AuthExpireLoginNonce() {
	switch db.Engine {
	case "mysql":
		db.mysql.AuthExpireLoginNonce()
	case "sqlite":
		db.sqlite.AuthExpireLoginNonce()
	}
}
func (db *Database) AuthGetServerOwnerAddress() string {
	switch db.Engine {
	case "mysql":
		return db.mysql.AuthGetServerOwnerAddress()
	case "sqlite":
		return db.sqlite.AuthGetServerOwnerAddress()
	}
	return ""
}
func (db *Database) AuthGetServerOwnerNetwork() string {
	switch db.Engine {
	case "mysql":
		return db.mysql.AuthGetServerOwnerNetwork()
	case "sqlite":
		return db.sqlite.AuthGetServerOwnerNetwork()
	}
	return ""
}

// --- Files Functions --- //
func (db *Database) FileAdd(fileUUID string, fileHash string, mimeType string, fileName string, size int64) {
	switch db.Engine {
	case "mysql":
		db.mysql.FileAdd(fileUUID, fileHash, mimeType, fileName, size)
	case "sqlite":
		db.sqlite.FileAdd(fileUUID, fileHash, mimeType, fileName, size)
	}
}
func (db *Database) IPFSAdd(fileUUID string, cid string) {
	switch db.Engine {
	case "mysql":
		db.mysql.IPFSAdd(fileUUID, cid)
	case "sqlite":
		db.sqlite.IPFSAdd(fileUUID, cid)
	}
}
func (db *Database) GetFileHashFromUUID(fileUUID string) string {
	switch db.Engine {
	case "mysql":
		return db.mysql.GetFileHashFromUUID(fileUUID)
	case "sqlite":
		return db.sqlite.GetFileHashFromUUID(fileUUID)
	}
	return ""
}

// --- Indexer Functions --- //
func (db *Database) IndexerCreateJob(uuid string, blockchain string) {
	switch db.Engine {
	case "mysql":
		db.mysql.IndexerCreateJob(uuid, blockchain)
	case "sqlite":
		db.sqlite.IndexerCreateJob(uuid, blockchain)
	}
}
func (db *Database) IndexerGetJobUUID(blockchain string) string {
	switch db.Engine {
	case "mysql":
		return db.mysql.IndexerGetJobUUID(blockchain)
	case "sqlite":
		return db.sqlite.IndexerGetJobUUID(blockchain)
	}
	return ""
}
func (db *Database) IndexerGetJobStatus(uuid string) string {
	switch db.Engine {
	case "mysql":
		return db.mysql.IndexerGetJobStatus(uuid)
	case "sqlite":
		return db.sqlite.IndexerGetJobStatus(uuid)
	}
	return ""
}
func (db *Database) IndexerGetHeadBlock(uuid string) uint64 {
	switch db.Engine {
	case "mysql":
		return db.mysql.IndexerGetHeadBlock(uuid)
	case "sqlite":
		return db.sqlite.IndexerGetHeadBlock(uuid)
	}
	return 0
}
func (db *Database) IndexerGetTailBlock(uuid string) uint64 {
	switch db.Engine {
	case "mysql":
		return db.mysql.IndexerGetTailBlock(uuid)
	case "sqlite":
		return db.sqlite.IndexerGetTailBlock(uuid)
	}
	return 0
}
func (db *Database) IndexerGetRunningJobsUUIDs() []string {
	switch db.Engine {
	case "mysql":
		return db.mysql.IndexerGetRunningJobsUUIDs()
	case "sqlite":
		return db.sqlite.IndexerGetRunningJobsUUIDs()
	}
	return []string{}
}
func (db *Database) IndexerUpdateJobStatus(uuid string, status string) {
	switch db.Engine {
	case "mysql":
		db.mysql.IndexerUpdateJobStatus(uuid, status)
	case "sqlite":
		db.sqlite.IndexerUpdateJobStatus(uuid, status)
	}
}
func (db *Database) IndexerUpdateHeadBlock(uuid string, headBlock uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.IndexerUpdateHeadBlock(uuid, headBlock)
	case "sqlite":
		db.sqlite.IndexerUpdateHeadBlock(uuid, headBlock)
	}
}
func (db *Database) IndexerUpdateTailBlock(uuid string, tailBlock uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.IndexerUpdateTailBlock(uuid, tailBlock)
	case "sqlite":
		db.sqlite.IndexerUpdateTailBlock(uuid, tailBlock)
	}
}
func (db *Database) IndexerUpdateJobSpeed(uuid string, rps uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.IndexerUpdateJobSpeed(uuid, rps)
	case "sqlite":
		db.sqlite.IndexerUpdateJobSpeed(uuid, rps)
	}
}
func (db *Database) IndexerAddPost(txHash string, blockchain string, fromAddr string, toAddr string, parentTxHash string, amount uint64, timestamp uint64, data string, blockNumber uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.IndexerAddPost(txHash, blockchain, fromAddr, toAddr, parentTxHash, amount, timestamp, data, blockNumber)
	case "sqlite":
		db.sqlite.IndexerAddPost(txHash, blockchain, fromAddr, toAddr, parentTxHash, amount, timestamp, data, blockNumber)
	}
}
func (db *Database) IndexerResetJobs(blockchain string) {
	switch db.Engine {
	case "mysql":
		db.mysql.IndexerResetJobs(blockchain)
	case "sqlite":
		db.sqlite.IndexerResetJobs(blockchain)
	}
}

// --- Indexer Functions to Tokenize Onchain Posts --- //
func (db *Database) OnchainP(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainP(txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data)
	case "sqlite":
		db.sqlite.OnchainP(txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data)
	}
}
func (db *Database) OnchainPA(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string, attachments []Attachment) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainPA(txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data, attachments)
	case "sqlite":
		db.sqlite.OnchainPA(txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data, attachments)
	}
}
func (db *Database) OnchainMN(blockchain string, address string, name string, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainMN(blockchain, address, name, timestamp)
	case "sqlite":
		db.sqlite.OnchainMN(blockchain, address, name, timestamp)
	}
}
func (db *Database) OnchainMA(blockchain string, address string, avatar string, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainMA(blockchain, address, avatar, timestamp)
	case "sqlite":
		db.sqlite.OnchainMA(blockchain, address, avatar, timestamp)
	}
}
func (db *Database) OnchainMBot(blockchain string, address string, bot bool, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainMBot(blockchain, address, bot, timestamp)
	case "sqlite":
		db.sqlite.OnchainMBot(blockchain, address, bot, timestamp)
	}
}
func (db *Database) OnchainMB(blockchain string, address string, banner string, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainMB(blockchain, address, banner, timestamp)
	case "sqlite":
		db.sqlite.OnchainMB(blockchain, address, banner, timestamp)
	}
}
func (db *Database) OnchainMNsfw(blockchain string, address string, nsfw bool, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainMNsfw(blockchain, address, nsfw, timestamp)
	case "sqlite":
		db.sqlite.OnchainMNsfw(blockchain, address, nsfw, timestamp)
	}
}
func (db *Database) OnchainMV(blockchain string, address string, vertical string, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainMV(blockchain, address, vertical, timestamp)
	case "sqlite":
		db.sqlite.OnchainMV(blockchain, address, vertical, timestamp)
	}
}
func (db *Database) OnchainML(blockchain string, address string, location string, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainML(blockchain, address, location, timestamp)
	case "sqlite":
		db.sqlite.OnchainML(blockchain, address, location, timestamp)
	}
}
func (db *Database) OnchainMW(blockchain string, address string, website string, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainMW(blockchain, address, website, timestamp)
	case "sqlite":
		db.sqlite.OnchainMW(blockchain, address, website, timestamp)
	}
}
func (db *Database) OnchainMC(blockchain string, address string, colors string, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainMC(blockchain, address, colors, timestamp)
	case "sqlite":
		db.sqlite.OnchainMC(blockchain, address, colors, timestamp)
	}
}
func (db *Database) OnchainMD(blockchain string, address string, description string, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainMD(blockchain, address, description, timestamp)
	case "sqlite":
		db.sqlite.OnchainMD(blockchain, address, description, timestamp)
	}
}
func (db *Database) OnchainF(txHash string, blockchain string, followerAddress string, followerBlockchain string, followeeAddress string, followeeBlockchain string, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainF(txHash, blockchain, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain, timestamp)
	case "sqlite":
		db.sqlite.OnchainF(txHash, blockchain, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain, timestamp)
	}
}
func (db *Database) OnchainFU(txHash string, blockchain string, followerAddress string, followerBlockchain string, followeeAddress string, followeeBlockchain string, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainFU(txHash, blockchain, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain, timestamp)
	case "sqlite":
		db.sqlite.OnchainFU(txHash, blockchain, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain, timestamp)
	}
}
func (db *Database) OnchainDeleteExpired(blockchain string, cutoffTimestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainDeleteExpired(blockchain, cutoffTimestamp)
	case "sqlite":
		db.sqlite.OnchainDeleteExpired(blockchain, cutoffTimestamp)
	}
}

// --- Post Functions --- //
func (db *Database) GetPost(txHash string, blockchain string) map[string]interface{} {
	switch db.Engine {
	case "mysql":
		return db.mysql.GetPost(txHash, blockchain)
	case "sqlite":
		return db.sqlite.GetPost(txHash, blockchain)
	}
	return nil
}

// --- Search Functions --- //
func (db *Database) SearchGetPosts(query string, limit int, offset int) []map[string]interface{} {
	var posts []map[string]interface{}
	switch db.Engine {
	case "mysql":
		posts = db.mysql.SearchGetPosts(query, limit, offset)
		return posts
	case "sqlite":
		posts = db.sqlite.SearchGetPosts(query, limit, offset)
		return posts
	}
	return nil
}
func (db *Database) SearchGetProfiles(query string, limit int, offset int) []map[string]interface{} {
	var profiles []map[string]interface{}
	switch db.Engine {
	case "mysql":
		profiles = db.mysql.SearchGetProfiles(query, limit, offset)
		return profiles
	case "sqlite":
		profiles = db.sqlite.SearchGetProfiles(query, limit, offset)
		return profiles
	}
	return nil
}
func (db *Database) DiscoverGetRandomProfiles(limit int) []map[string]interface{} {
	switch db.Engine {
	case "mysql":
		return db.mysql.DiscoverGetRandomProfiles(limit)
	case "sqlite":
		return db.sqlite.DiscoverGetRandomProfiles(limit)
	}
	return nil
}
func (db *Database) DiscoverGetTopByFollowers(limit int) []map[string]interface{} {
	switch db.Engine {
	case "mysql":
		return db.mysql.DiscoverGetTopByFollowers(limit)
	case "sqlite":
		return db.sqlite.DiscoverGetTopByFollowers(limit)
	}
	return nil
}
func (db *Database) DiscoverGetTopByPosts(limit int) []map[string]interface{} {
	switch db.Engine {
	case "mysql":
		return db.mysql.DiscoverGetTopByPosts(limit)
	case "sqlite":
		return db.sqlite.DiscoverGetTopByPosts(limit)
	}
	return nil
}

// --- Profile Functions --- //
func (db *Database) ProfileGetName(address string, blockchain string) string {
	var name string
	switch db.Engine {
	case "mysql":
		name = db.mysql.ProfileGetName(address, blockchain)
	case "sqlite":
		name = db.sqlite.ProfileGetName(address, blockchain)
	}
	return name
}
func (db *Database) ProfileGetCommentCount(address string, blockchain string) int64 {
	switch db.Engine {
	case "mysql":
		return db.mysql.ProfileGetCommentCount(address, blockchain)
	case "sqlite":
		return db.sqlite.ProfileGetCommentCount(address, blockchain)
	}
	return 0
}
func (db *Database) ProfileGetComments(address string, blockchain string, limit int, offset int) []map[string]interface{} {
	var comments []map[string]interface{}
	switch db.Engine {
	case "mysql":
		comments = db.mysql.ProfileGetComments(address, blockchain, limit, offset)
		return comments
	case "sqlite":
		comments = db.sqlite.ProfileGetComments(address, blockchain, limit, offset)
		return comments
	}
	return nil
}
func (db *Database) ProfileGetPostCount(address string, blockchain string) int64 {
	switch db.Engine {
	case "mysql":
		return db.mysql.ProfileGetPostCount(address, blockchain)
	case "sqlite":
		return db.sqlite.ProfileGetPostCount(address, blockchain)
	}
	return 0
}
func (db *Database) ProfileGetPosts(address string, blockchain string, limit int, offset int) []map[string]interface{} {
	var posts []map[string]interface{}
	switch db.Engine {
	case "mysql":
		posts = db.mysql.ProfileGetPosts(address, blockchain, limit, offset)
		return posts
	case "sqlite":
		posts = db.sqlite.ProfileGetPosts(address, blockchain, limit, offset)
		return posts
	}
	return nil
}
func (db *Database) ProfileGetAvatar(address string, blockchain string) string {
	var avatar string
	switch db.Engine {
	case "mysql":
		avatar = db.mysql.ProfileGetAvatar(address, blockchain)
	case "sqlite":
		avatar = db.sqlite.ProfileGetAvatar(address, blockchain)
	}
	return avatar
}
func (db *Database) ProfileGetBanner(address string, blockchain string) string {
	var banner string
	switch db.Engine {
	case "mysql":
		banner = db.mysql.ProfileGetBanner(address, blockchain)
	case "sqlite":
		banner = db.sqlite.ProfileGetBanner(address, blockchain)
	}
	return banner
}
func (db *Database) ProfileGetColors(address string, blockchain string) string {
	var colors string
	switch db.Engine {
	case "mysql":
		colors = db.mysql.ProfileGetColors(address, blockchain)
	case "sqlite":
		colors = db.sqlite.ProfileGetColors(address, blockchain)
	}
	return colors
}
func (db *Database) ProfileGetDescription(address string, blockchain string) string {
	var description string
	switch db.Engine {
	case "mysql":
		description = db.mysql.ProfileGetDescription(address, blockchain)
	case "sqlite":
		description = db.sqlite.ProfileGetDescription(address, blockchain)
	}
	return description
}
func (db *Database) ProfileGetLocation(address string, blockchain string) string {
	var location string
	switch db.Engine {
	case "mysql":
		location = db.mysql.ProfileGetLocation(address, blockchain)
	case "sqlite":
		location = db.sqlite.ProfileGetLocation(address, blockchain)
	}
	return location
}
func (db *Database) ProfileGetWebsite(address string, blockchain string) string {
	var website string
	switch db.Engine {
	case "mysql":
		website = db.mysql.ProfileGetWebsite(address, blockchain)
	case "sqlite":
		website = db.sqlite.ProfileGetWebsite(address, blockchain)
	}
	return website
}
func (db *Database) ProfileGetBot(address string, blockchain string) bool {
	switch db.Engine {
	case "mysql":
		return db.mysql.ProfileGetBot(address, blockchain)
	case "sqlite":
		return db.sqlite.ProfileGetBot(address, blockchain)
	}
	return false
}
func (db *Database) ProfileGetNsfw(address string, blockchain string) bool {
	switch db.Engine {
	case "mysql":
		return db.mysql.ProfileGetNsfw(address, blockchain)
	case "sqlite":
		return db.sqlite.ProfileGetNsfw(address, blockchain)
	}
	return false
}
func (db *Database) ProfileGetVertical(address string, blockchain string) string {
	var vertical string
	switch db.Engine {
	case "mysql":
		vertical = db.mysql.ProfileGetVertical(address, blockchain)
	case "sqlite":
		vertical = db.sqlite.ProfileGetVertical(address, blockchain)
	}
	return vertical
}
func (db *Database) ProfileGetJoinedDate(address string, blockchain string) *int64 {
	var joineddate *int64
	switch db.Engine {
	case "mysql":
		joineddate = db.mysql.ProfileGetJoinedDate(address, blockchain)
	case "sqlite":
		joineddate = db.sqlite.ProfileGetJoinedDate(address, blockchain)
	}
	return joineddate
}
func (db *Database) ProfileGetFollowerCount(address string, blockchain string) *int64 {
	var followerCount *int64
	switch db.Engine {
	case "mysql":
		followerCount = db.mysql.ProfileGetFollowerCount(address, blockchain)
	case "sqlite":
		followerCount = db.sqlite.ProfileGetFollowerCount(address, blockchain)
	}
	return followerCount
}
func (db *Database) ProfileGetFollowingCount(address string, blockchain string) *int64 {
	var followingCount *int64
	switch db.Engine {
	case "mysql":
		followingCount = db.mysql.ProfileGetFollowingCount(address, blockchain)
	case "sqlite":
		followingCount = db.sqlite.ProfileGetFollowingCount(address, blockchain)
	}
	return followingCount
}
func (db *Database) ProfileIsFollower(followeeAddress string, followeeBlockchain string, followerAddress string, followerBlockchain string) bool {
	switch db.Engine {
	case "mysql":
		return db.mysql.ProfileIsFollower(followeeAddress, followeeBlockchain, followerAddress, followerBlockchain)
	case "sqlite":
		return db.sqlite.ProfileIsFollower(followeeAddress, followeeBlockchain, followerAddress, followerBlockchain)
	}
	return false
}
func (db *Database) GetFollowersFeed(followerAddress string, followerBlockchain string, limit int, offset int) []map[string]interface{} {
	switch db.Engine {
	case "mysql":
		return db.mysql.GetFollowersFeed(followerAddress, followerBlockchain, limit, offset)
	case "sqlite":
		return db.sqlite.GetFollowersFeed(followerAddress, followerBlockchain, limit, offset)
	}
	return nil
}
func (db *Database) ProfileGetAddressesWithMissingEnsData(blockchain string) []string {
	switch db.Engine {
	case "mysql":
		return db.mysql.ProfileGetAddressesWithMissingEnsData(blockchain)
	case "sqlite":
		return db.sqlite.ProfileGetAddressesWithMissingEnsData(blockchain)
	}
	return nil
}
func (db *Database) ProfileUpdateEnsData(address string, blockchain string, name string, avatar string) {
	switch db.Engine {
	case "mysql":
		db.mysql.ProfileUpdateEnsData(address, blockchain, name, avatar)
	case "sqlite":
		db.sqlite.ProfileUpdateEnsData(address, blockchain, name, avatar)
	}
}
func (db *Database) ProfileIsEnsDataFresh(address string, blockchain string) bool {
	switch db.Engine {
	case "mysql":
		return db.mysql.ProfileIsEnsDataFresh(address, blockchain)
	case "sqlite":
		return db.sqlite.ProfileIsEnsDataFresh(address, blockchain)
	}
	return false
}
func (db *Database) ProfileGetEnsName(address string, blockchain string) string {
	switch db.Engine {
	case "mysql":
		return db.mysql.ProfileGetEnsName(address, blockchain)
	case "sqlite":
		return db.sqlite.ProfileGetEnsName(address, blockchain)
	}
	return ""
}
func (db *Database) ProfileGetEnsAvatar(address string, blockchain string) string {
	switch db.Engine {
	case "mysql":
		return db.mysql.ProfileGetEnsAvatar(address, blockchain)
	case "sqlite":
		return db.sqlite.ProfileGetEnsAvatar(address, blockchain)
	}
	return ""
}

// --- Notifications --- //
func (db *Database) NotificationInsert(uid string, message string) {
	switch db.Engine {
	case "mysql":
		db.mysql.NotificationInsert(uid, message)
	case "sqlite":
		db.sqlite.NotificationInsert(uid, message)
	}
}
func (db *Database) NotificationDismiss(uid string) {
	switch db.Engine {
	case "mysql":
		db.mysql.NotificationDismiss(uid)
	case "sqlite":
		db.sqlite.NotificationDismiss(uid)
	}
}
func (db *Database) NotificationGetActive() []map[string]string {
	switch db.Engine {
	case "mysql":
		return db.mysql.NotificationGetActive()
	case "sqlite":
		return db.sqlite.NotificationGetActive()
	}
	return nil
}

// --- User Notifications --- //
func (db *Database) UserNotificationClearAll(userAddress string, userBlockchain string) {
	switch db.Engine {
	case "mysql":
		db.mysql.UserNotificationClearAll(userAddress, userBlockchain)
	case "sqlite":
		db.sqlite.UserNotificationClearAll(userAddress, userBlockchain)
	}
}
func (db *Database) UserNotificationCleanup() {
	switch db.Engine {
	case "mysql":
		db.mysql.UserNotificationCleanup()
	case "sqlite":
		db.sqlite.UserNotificationCleanup()
	}
}
func (db *Database) UserNotificationDismiss(id string) {
	switch db.Engine {
	case "mysql":
		db.mysql.UserNotificationDismiss(id)
	case "sqlite":
		db.sqlite.UserNotificationDismiss(id)
	}
}
func (db *Database) UserNotificationGet(userAddress string, userBlockchain string, limit int, offset int) []map[string]string {
	switch db.Engine {
	case "mysql":
		return db.mysql.UserNotificationGet(userAddress, userBlockchain, limit, offset)
	case "sqlite":
		return db.sqlite.UserNotificationGet(userAddress, userBlockchain, limit, offset)
	}
	return nil
}
func (db *Database) UserNotificationGetCount(userAddress string, userBlockchain string, since uint64) int64 {
	switch db.Engine {
	case "mysql":
		return db.mysql.UserNotificationGetCount(userAddress, userBlockchain, since)
	case "sqlite":
		return db.sqlite.UserNotificationGetCount(userAddress, userBlockchain, since)
	}
	return 0
}
func (db *Database) UserNotificationGetSeen(userAddress string, userBlockchain string) uint64 {
	switch db.Engine {
	case "mysql":
		return db.mysql.UserNotificationGetSeen(userAddress, userBlockchain)
	case "sqlite":
		return db.sqlite.UserNotificationGetSeen(userAddress, userBlockchain)
	}
	return 0
}
func (db *Database) UserNotificationUpdateSeen(userAddress string, userBlockchain string, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.UserNotificationUpdateSeen(userAddress, userBlockchain, timestamp)
	case "sqlite":
		db.sqlite.UserNotificationUpdateSeen(userAddress, userBlockchain, timestamp)
	}
}

// --- oEmbed Cache Functions --- //
func (db *Database) OEmbedCacheGet(url string) (string, int64) {
	switch db.Engine {
	case "mysql":
		return db.mysql.OEmbedCacheGet(url)
	case "sqlite":
		return db.sqlite.OEmbedCacheGet(url)
	}
	return "", 0
}
func (db *Database) OEmbedCacheSet(url string, data string) {
	switch db.Engine {
	case "mysql":
		db.mysql.OEmbedCacheSet(url, data)
	case "sqlite":
		db.sqlite.OEmbedCacheSet(url, data)
	}
}

// --- Snapshot Service Functions --- //
func (db *Database) SnapshotSetDefaults() {
	defaults := map[string]string{
		"historyDays":      "-1",
		"indexerOnBattery": "true",
		"indexerRunning":   "true",
	}
	for key, defaultValue := range defaults {
		existingValue := db.SettingsGetValue(key)
		if existingValue == "" {
			db.SettingsUpdateValue(key, defaultValue)
		}
	}
}
func (db *Database) ExportSnapshots(exportPath string, blockchain string, headBlock uint64, tailBlock uint64) error {
	switch db.Engine {
	case "mysql":
		return db.mysql.exportSnapshots(exportPath, blockchain, headBlock, tailBlock)
	case "sqlite":
		return db.sqlite.exportSnapshots(exportPath, blockchain, headBlock, tailBlock)
	default:
		return core.LogErrorReturn("Invalid DB engine selected")
	}
}

// --- Wallet Functions --- //
func (db *Database) WalletStore(publicKey string, blockchain string, address string, encryptedPrivateKey string, isDefault bool) error {
	switch db.Engine {
	case "mysql":
		return db.mysql.WalletStore(publicKey, blockchain, address, encryptedPrivateKey, isDefault)
	case "sqlite":
		return db.sqlite.WalletStore(publicKey, blockchain, address, encryptedPrivateKey, isDefault)
	}
	return core.LogErrorReturn("Invalid DB engine selected")
}
func (db *Database) WalletGet(publicKey string, blockchain string) (map[string]interface{}, error) {
	switch db.Engine {
	case "mysql":
		return db.mysql.WalletGet(publicKey, blockchain)
	case "sqlite":
		return db.sqlite.WalletGet(publicKey, blockchain)
	}
	return nil, core.LogErrorReturn("Invalid DB engine selected")
}
func (db *Database) WalletGetDefault(blockchain string) (map[string]interface{}, error) {
	switch db.Engine {
	case "mysql":
		return db.mysql.WalletGetDefault(blockchain)
	case "sqlite":
		return db.sqlite.WalletGetDefault(blockchain)
	}
	return nil, core.LogErrorReturn("Invalid DB engine selected")
}
func (db *Database) WalletGetPrivateKey(publicKey string, blockchain string) (string, error) {
	switch db.Engine {
	case "mysql":
		return db.mysql.WalletGetPrivateKey(publicKey, blockchain)
	case "sqlite":
		return db.sqlite.WalletGetPrivateKey(publicKey, blockchain)
	}
	return "", core.LogErrorReturn("Invalid DB engine selected")
}
func (db *Database) WalletSetDefault(publicKey string, blockchain string) error {
	switch db.Engine {
	case "mysql":
		return db.mysql.WalletSetDefault(publicKey, blockchain)
	case "sqlite":
		return db.sqlite.WalletSetDefault(publicKey, blockchain)
	}
	return core.LogErrorReturn("Invalid DB engine selected")
}
func (db *Database) WalletGetAll() ([]map[string]interface{}, error) {
	switch db.Engine {
	case "mysql":
		return db.mysql.WalletGetAll()
	case "sqlite":
		return db.sqlite.WalletGetAll()
	}
	return nil, core.LogErrorReturn("Invalid DB engine selected")
}

// --- Comment Functions --- //
func (db *Database) OnchainC(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainC(txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data)
	case "sqlite":
		db.sqlite.OnchainC(txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data)
	}
}
func (db *Database) OnchainCA(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string, attachments []Attachment) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainCA(txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data, attachments)
	case "sqlite":
		db.sqlite.OnchainCA(txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data, attachments)
	}
}
func (db *Database) GetComments(parentTxHash string, blockchain string, limit int, offset int) []map[string]interface{} {
	switch db.Engine {
	case "mysql":
		return db.mysql.GetComments(parentTxHash, blockchain, limit, offset)
	case "sqlite":
		return db.sqlite.GetComments(parentTxHash, blockchain, limit, offset)
	}
	return nil
}
func (db *Database) GetCommentCount(targetTxHash string, blockchain string) int64 {
	switch db.Engine {
	case "mysql":
		return db.mysql.GetCommentCount(targetTxHash, blockchain)
	case "sqlite":
		return db.sqlite.GetCommentCount(targetTxHash, blockchain)
	}
	return 0
}
func (db *Database) HasUserCommented(parentTxHash string, blockchain string, address string) bool {
	switch db.Engine {
	case "mysql":
		return db.mysql.HasUserCommented(parentTxHash, blockchain, address)
	case "sqlite":
		return db.sqlite.HasUserCommented(parentTxHash, blockchain, address)
	}
	return false
}

// --- Reaction Functions --- //
func (db *Database) OnchainR(txHash string, blockchain string, fromAddr string, targetTxHash string, targetType string, reactionType string, timestamp uint64) {
	switch db.Engine {
	case "mysql":
		db.mysql.OnchainR(txHash, blockchain, fromAddr, targetTxHash, targetType, reactionType, timestamp)
	case "sqlite":
		db.sqlite.OnchainR(txHash, blockchain, fromAddr, targetTxHash, targetType, reactionType, timestamp)
	}
}
func (db *Database) GetReactionCounts(targetTxHash string, blockchain string) map[string]interface{} {
	switch db.Engine {
	case "mysql":
		return db.mysql.GetReactionCounts(targetTxHash, blockchain)
	case "sqlite":
		return db.sqlite.GetReactionCounts(targetTxHash, blockchain)
	}
	return nil
}
func (db *Database) GetUserReaction(targetTxHash string, blockchain string, fromAddress string) string {
	switch db.Engine {
	case "mysql":
		return db.mysql.GetUserReaction(targetTxHash, blockchain, fromAddress)
	case "sqlite":
		return db.sqlite.GetUserReaction(targetTxHash, blockchain, fromAddress)
	}
	return ""
}
func (db *Database) GetUserReactions(targetTxHash string, blockchain string, fromAddress string) map[string]string {
	switch db.Engine {
	case "mysql":
		return db.mysql.GetUserReactions(targetTxHash, blockchain, fromAddress)
	case "sqlite":
		return db.sqlite.GetUserReactions(targetTxHash, blockchain, fromAddress)
	}
	return map[string]string{"likeDislike": "", "emoji": ""}
}
