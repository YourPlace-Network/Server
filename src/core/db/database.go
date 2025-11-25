package db

import (
	"YourPlace/src/core"
	"database/sql"
	"fmt"
	"slices"
	"time"
)

type Database struct {
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
	validEngines := []string{"sqlite"}
	if !slices.Contains(validEngines, engine) {
		core.LogFatal("Invalid DB engine selected")
	}
	db.sqlite.Init(path)
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
func (db *Database) SetDefaults() {
	defaults := map[string]string{
		"historyDays":      "90",
		"indexerOnBattery": "false",
		"indexerRunning":   "true",
		"badbitsEnabled":   "true",
	}
	err := db.sqlite.withTransaction(func(tx *sql.Tx) error {
		for key, defaultValue := range defaults {
			var value string
			err := tx.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
			if err == sql.ErrNoRows || len(value) == 0 { // If the setting already exists, skip
				_, err = tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", key, defaultValue)
				if err != nil {
					return core.LogErrorReturn(fmt.Sprintf("Failed to insert default %s: %w", key, err))
				}
			}
		}
		return nil
	})
	if err != nil {
		core.LogError("Failed to set defaults: " + err.Error())
	}
}
func (db *Database) SetGatewayDefaults(uploadDirectory string) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	metaDefaults := map[string]string{
		"accountAddress": "0x0000000000000000000000000000000000000000",
		"accountNetwork": "Base",
		"installedDate":  timestamp,
	}
	settingsDefaults := map[string]string{
		"uploadDirectory": uploadDirectory,
	}
	err := db.sqlite.withTransaction(func(tx *sql.Tx) error {
		for key, defaultValue := range metaDefaults {
			var value string
			err := tx.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&value)
			if err == sql.ErrNoRows || len(value) == 0 {
				_, err = tx.Exec("INSERT INTO meta (key, value) VALUES (?, ?)", key, defaultValue)
				if err != nil {
					return core.LogErrorReturn(fmt.Sprintf("Failed to insert meta default %s: %w", key, err))
				}
			}
		}
		for key, defaultValue := range settingsDefaults {
			var value string
			err := tx.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
			if err == sql.ErrNoRows || len(value) == 0 {
				_, err = tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", key, defaultValue)
				if err != nil {
					return core.LogErrorReturn(fmt.Sprintf("Failed to insert settings default %s: %w", key, err))
				}
			}
		}
		return nil
	})
	if err != nil {
		core.LogError("Failed to set gateway defaults: " + err.Error())
	}
}
func (db *Database) ExportSnapshot(exportPath string) error {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.ExportSnapshot(exportPath)
	default:
		return core.LogErrorReturn("Invalid DB engine selected")
	}
}
func (db *Database) ImportSnapshot(importPath string) error {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.ImportSnapshot(importPath)
	default:
		return core.LogErrorReturn("Invalid DB engine selected")
	}
}
func (db *Database) ImportSnapshotNoMetadata(importPath string) error {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.ImportSnapshotNoMetadata(importPath)
	default:
		return core.LogErrorReturn("Invalid DB engine selected")
	}
}

// --- Metadata & Settings Functions --- //
func (db *Database) MetaUpdateValue(key string, value string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.MetaUpdateValue(key, value)
	}
}
func (db *Database) MetaGetValue(key string) string {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.MetaGetValue(key)
	}
	return ""
}
func (db *Database) SettingsGetValue(key string) string {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.SettingsGetValue(key)
	}
	return ""
}
func (db *Database) SettingsUpdateValue(key string, value string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.SettingsUpdateValue(key, value)
	}
}
func (db *Database) SettingsDeleteValue(key string) error {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.SettingsDeleteValue(key)
	}
	return nil
}
func (db *Database) Ping() bool {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.Ping()
	}
	return false
}
func SanitizeDatabase(path string) error {
	return SanitizeSQLiteDatabase(path)
}

// --- Auth Functions --- //
func (db *Database) AuthGetNonceStatus(nonce string) string {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.AuthGetNonceStatus(nonce)
	}
	return ""
}
func (db *Database) AuthUpdateNonce(nonce string, status string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.AuthUpdateNonce(nonce, status)
	}
}
func (db *Database) AuthDeleteNonce(nonce string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.AuthDeleteNonce(nonce)
	}
}
func (db *Database) AuthExpireCookie(uuid string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.AuthExpireCookie(uuid)
	}
}
func (db *Database) AuthGetCookieStatus(uuid string) string {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.AuthGetCookieStatus(uuid)
	}
	return ""
}
func (db *Database) AuthUpdateLoginNonce(nonce string, domain string, expiration uint64, nonceHash string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.AuthUpdateLoginNonce(nonce, domain, expiration, nonceHash)
	}
}
func (db *Database) AuthGetLoginNonceByHash(nonceHash string) string {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.AuthGetLoginNonceByHash(nonceHash)
	}
	return ""
}
func (db *Database) AuthDeleteLoginNonce(nonce string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.AuthDeleteLoginNonce(nonce)
	}
}
func (db *Database) AuthExpireLoginNonce() {
	switch db.Engine {
	case "sqlite":
		db.sqlite.AuthExpireLoginNonce()
	}
}
func (db *Database) AuthGetServerOwnerAddress() string {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.AuthGetServerOwnerAddress()
	}
	return ""
}
func (db *Database) AuthGetServerOwnerNetwork() string {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.AuthGetServerOwnerNetwork()
	}
	return ""
}

// --- Files Functions --- //
func (db *Database) FileAdd(fileUUID string, fileHash string, mimeType string, fileName string, size int64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.FileAdd(fileUUID, fileHash, mimeType, fileName, size)
	}
}
func (db *Database) IPFSAdd(fileUUID string, cid string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.IPFSAdd(fileUUID, cid)
	}
}
func (db *Database) GetFileHashFromUUID(fileUUID string) string {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.GetFileHashFromUUID(fileUUID)
	}
	return ""
}

// --- Indexer Functions --- //
func (db *Database) IndexerCreateJob(uuid string, blockchain string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.IndexerCreateJob(uuid, blockchain)
	}
}
func (db *Database) IndexerGetJobUUID(blockchain string) string {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.IndexerGetJobUUID(blockchain)
	}
	return ""
}
func (db *Database) IndexerGetJobStatus(uuid string) string {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.IndexerGetJobStatus(uuid)
	}
	return ""
}
func (db *Database) IndexerGetHeadBlock(uuid string) uint64 {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.IndexerGetHeadBlock(uuid)
	}
	return 0
}
func (db *Database) IndexerGetTailBlock(uuid string) uint64 {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.IndexerGetTailBlock(uuid)
	}
	return 0
}
func (db *Database) IndexerGetRunningJobsUUIDs() []string {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.IndexerGetRunningJobsUUIDs()
	}
	return []string{}
}
func (db *Database) IndexerUpdateJobStatus(uuid string, status string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.IndexerUpdateJobStatus(uuid, status)
	}
}
func (db *Database) IndexerUpdateHeadBlock(uuid string, headBlock uint64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.IndexerUpdateHeadBlock(uuid, headBlock)
	}
}
func (db *Database) IndexerUpdateTailBlock(uuid string, tailBlock uint64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.IndexerUpdateTailBlock(uuid, tailBlock)
	}
}
func (db *Database) IndexerUpdateJobSpeed(uuid string, rps uint64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.IndexerUpdateJobSpeed(uuid, rps)
	}
}
func (db *Database) IndexerAddPost(txHash string, blockchain string, fromAddr string, toAddr string, parentTxHash string, amount uint64, timestamp uint64, data string, blockNumber uint64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.IndexerAddPost(txHash, blockchain, fromAddr, toAddr, parentTxHash, amount, timestamp, data, blockNumber)
	}
}
func (db *Database) IndexerResetJobs(blockchain string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.IndexerResetJobs(blockchain)
	}
}

// --- Indexer Functions to Tokenize Onchain Posts --- //
func (db *Database) OnchainP(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.OnchainP(txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data)
	}
}
func (db *Database) OnchainPA(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string, attachments []Attachment) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.OnchainPA(txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data, attachments)
	}
}
func (db *Database) OnchainMN(blockchain string, address string, name string, timestamp uint64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.OnchainMN(blockchain, address, name, timestamp)
	}
}
func (db *Database) OnchainMA(blockchain string, address string, avatar string, timestamp uint64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.OnchainMA(blockchain, address, avatar, timestamp)
	}
}
func (db *Database) OnchainMB(blockchain string, address string, avatar string, timestamp uint64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.OnchainMB(blockchain, address, avatar, timestamp)
	}
}
func (db *Database) OnchainMBD(blockchain string, address string, birthdate uint64, timestamp uint64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.OnchainMBD(blockchain, address, birthdate, timestamp)
	}
}
func (db *Database) OnchainML(blockchain string, address string, location string, timestamp uint64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.OnchainML(blockchain, address, location, timestamp)
	}
}
func (db *Database) OnchainMW(blockchain string, address string, website string, timestamp uint64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.OnchainMW(blockchain, address, website, timestamp)
	}
}
func (db *Database) OnchainMD(blockchain string, address string, description string, timestamp uint64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.OnchainMD(blockchain, address, description, timestamp)
	}
}
func (db *Database) OnchainF(txHash string, blockchain string, followerAddress string, followerBlockchain string, followeeAddress string, followeeBlockchain string, timestamp uint64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.OnchainF(txHash, blockchain, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain, timestamp)
	}
}
func (db *Database) OnchainFU(txHash string, blockchain string, followerAddress string, followerBlockchain string, followeeAddress string, followeeBlockchain string, timestamp uint64) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.OnchainFU(txHash, blockchain, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain, timestamp)
	}
}

// --- Search Functions --- //
func (db *Database) SearchGetPosts(query string) []map[string]interface{} {
	var posts []map[string]interface{}
	switch db.Engine {
	case "sqlite":
		posts = db.sqlite.SearchGetPosts(query)
		return posts
	}
	return nil
}
func (db *Database) SearchGetProfiles(query string) []map[string]interface{} {
	var profiles []map[string]interface{}
	switch db.Engine {
	case "sqlite":
		profiles = db.sqlite.SearchGetProfiles(query)
		return profiles
	}
	return nil
}

// --- Profile Functions --- //
func (db *Database) ProfileGetName(address string, blockchain string) string {
	var name string
	switch db.Engine {
	case "sqlite":
		name = db.sqlite.ProfileGetName(address, blockchain)
	}
	return name
}
func (db *Database) ProfileGetPosts(address string, blockchain string) []map[string]interface{} {
	var posts []map[string]interface{}
	switch db.Engine {
	case "sqlite":
		posts = db.sqlite.ProfileGetPosts(address, blockchain)
		return posts
	}
	return nil
}
func (db *Database) ProfileGetAvatar(address string, blockchain string) string {
	var avatar string
	switch db.Engine {
	case "sqlite":
		avatar = db.sqlite.ProfileGetAvatar(address, blockchain)
	}
	return avatar
}
func (db *Database) ProfileGetBanner(address string, blockchain string) string {
	var banner string
	switch db.Engine {
	case "sqlite":
		banner = db.sqlite.ProfileGetBanner(address, blockchain)
	}
	return banner
}
func (db *Database) ProfileGetDescription(address string, blockchain string) string {
	var description string
	switch db.Engine {
	case "sqlite":
		description = db.sqlite.ProfileGetDescription(address, blockchain)
	}
	return description
}
func (db *Database) ProfileGetLocation(address string, blockchain string) string {
	var location string
	switch db.Engine {
	case "sqlite":
		location = db.sqlite.ProfileGetLocation(address, blockchain)
	}
	return location
}
func (db *Database) ProfileGetWebsite(address string, blockchain string) string {
	var website string
	switch db.Engine {
	case "sqlite":
		website = db.sqlite.ProfileGetWebsite(address, blockchain)
	}
	return website
}
func (db *Database) ProfileGetBirthDate(address string, blockchain string) *int64 {
	var birthday *int64
	switch db.Engine {
	case "sqlite":
		birthday = db.sqlite.ProfileGetBirthDate(address, blockchain)
	}
	return birthday
}
func (db *Database) ProfileGetJoinedDate(address string, blockchain string) *int64 {
	var joineddate *int64
	switch db.Engine {
	case "sqlite":
		joineddate = db.sqlite.ProfileGetJoinedDate(address, blockchain)
	}
	return joineddate
}
func (db *Database) ProfileGetFollowerCount(address string, blockchain string) *int64 {
	var followerCount *int64
	switch db.Engine {
	case "sqlite":
		followerCount = db.sqlite.ProfileGetFollowerCount(address, blockchain)
	}
	return followerCount
}
func (db *Database) ProfileGetFollowingCount(address string, blockchain string) *int64 {
	var followingCount *int64
	switch db.Engine {
	case "sqlite":
		followingCount = db.sqlite.ProfileGetFollowingCount(address, blockchain)
	}
	return followingCount
}
func (db *Database) ProfileIsFollower(address string, blockchain string, followerAddress string, followerBlockchain string) bool {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.ProfileIsFollower(address, blockchain, followerAddress, followerBlockchain)
	}
	return false
}
func (db *Database) GetFollowersFeed(followerAddress string, followerBlockchain string, limit int) []map[string]interface{} {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.GetFollowersFeed(followerAddress, followerBlockchain, limit)
	}
	return nil
}

// --- Notifications --- //
func (db *Database) NotificationInsert(uid string, message string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.NotificationInsert(uid, message)
	}
}
func (db *Database) NotificationDismiss(uid string) {
	switch db.Engine {
	case "sqlite":
		db.sqlite.NotificationDismiss(uid)
	}
}
func (db *Database) NotificationGetActive() []map[string]string {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.NotificationGetActive()
	}
	return nil
}

// --- Snapshot Service Functions --- //
func (db *Database) SnapshotSetDefaults() {
	defaults := map[string]string{
		"historyDays":      "-1",
		"indexerOnBattery": "true",
		"indexerRunning":   "true",
	}
	err := db.sqlite.withTransaction(func(tx *sql.Tx) error {
		for key, defaultValue := range defaults {
			var value string
			err := tx.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
			if err == sql.ErrNoRows || len(value) == 0 { // If the setting already exists, skip
				_, err = tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", key, defaultValue)
				if err != nil {
					return core.LogErrorReturn(fmt.Sprintf("Failed to insert default %s: %w", key, err))
				}
			}
		}
		return nil
	})
	if err != nil {
		core.LogError("Failed to set defaults: " + err.Error())
	}
}
func (db *Database) ExportSnapshots(exportPath string, blockchain string, headBlock uint64, tailBlock uint64) error {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.exportSnapshots(exportPath, blockchain, headBlock, tailBlock)
	default:
		return core.LogErrorReturn("Invalid DB engine selected")
	}
}

// --- Wallet Functions --- //
func (db *Database) WalletStore(publicKey string, blockchain string, address string, encryptedPrivateKey string, isDefault bool) error {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.WalletStore(publicKey, blockchain, address, encryptedPrivateKey, isDefault)
	}
	return core.LogErrorReturn("Invalid DB engine selected")
}
func (db *Database) WalletGet(publicKey string, blockchain string) (map[string]interface{}, error) {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.WalletGet(publicKey, blockchain)
	}
	return nil, core.LogErrorReturn("Invalid DB engine selected")
}
func (db *Database) WalletGetDefault(blockchain string) (map[string]interface{}, error) {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.WalletGetDefault(blockchain)
	}
	return nil, core.LogErrorReturn("Invalid DB engine selected")
}
func (db *Database) WalletGetPrivateKey(publicKey string, blockchain string) (string, error) {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.WalletGetPrivateKey(publicKey, blockchain)
	}
	return "", core.LogErrorReturn("Invalid DB engine selected")
}
func (db *Database) WalletSetDefault(publicKey string, blockchain string) error {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.WalletSetDefault(publicKey, blockchain)
	}
	return core.LogErrorReturn("Invalid DB engine selected")
}
func (db *Database) WalletGetAll() ([]map[string]interface{}, error) {
	switch db.Engine {
	case "sqlite":
		return db.sqlite.WalletGetAll()
	}
	return nil, core.LogErrorReturn("Invalid DB engine selected")
}
