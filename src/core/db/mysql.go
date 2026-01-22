package db

import (
	"YourPlace/src/core"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

type MySQL struct {
	database *sql.DB
	dsn      string
}

func (db *MySQL) Init(dsn string) {
	// DSN == <db-username>:<URL-encoded-password>@tcp(<db-host>:<db-port>)/<db-name>
	startupCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	db.dsn = dsn
	dsnArgs := dsn + "?charset=utf8mb4&parseTime=true&multiStatements=true"
	initDB, err := sql.Open("mysql", dsnArgs)
	if err != nil || initDB == nil {
		core.LogFatal("Could not open MySQL db: " + err.Error())
		return
	}
	_, err = initDB.ExecContext(startupCtx,
		"CREATE DATABASE "+ // String break to stop IntelliJ static analysis from complaining about SQL dialects
			"IF NOT EXISTS `yourplace`")
	if err != nil {
		core.LogDebug("Could not create database: " + err.Error())
	}
	initDB.Close()
	database, err := sql.Open("mysql", dsnArgs)
	if err != nil || database == nil {
		core.LogFatal("Could not open MySQL db: " + err.Error())
		return
	}
	database.SetMaxOpenConns(50)
	database.SetMaxIdleConns(20)
	database.SetConnMaxLifetime(15 * time.Minute)
	database.SetConnMaxIdleTime(3 * time.Minute)
	db.database = database
	err = db.createTables(startupCtx)
	if err != nil {
		core.LogDebug("Could not create tables: " + err.Error())
	}
	if err := db.RunMigrations(); err != nil {
		core.LogFatal("Schema migration failed: " + err.Error())
		return
	}
}

func (db *MySQL) runSQL(query string) {
	_, err := db.database.Exec(query)
	if err != nil {
		core.LogDebug("Could not run MySQL query: " + query + " - " + err.Error())
	}
}
func (db *MySQL) runParamSQLSelect(query string, params ...interface{}) (*sql.Rows, error) {
	queryUpper := strings.ToUpper(strings.TrimSpace(query))
	timeout := 30 * time.Second
	if strings.Contains(query, "settings") || strings.Contains(query, "meta") {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if strings.HasPrefix(queryUpper, "SELECT") || strings.HasPrefix(queryUpper, "EXPLAIN") || strings.HasPrefix(queryUpper, "WITH") {
		statement, err := db.database.PrepareContext(ctx, query)
		if err != nil {
			if ctx.Err() == context.Canceled {
				return nil, core.LogDebugReturn("Query preparation canceled: " + err.Error())
			}
			return nil, core.LogDebugReturn("Could not prepare MySQL query: " + query + " - " + err.Error())
		}
		defer statement.Close()
		queryCtx := context.Background()
		rows, err := statement.QueryContext(queryCtx, params...)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil, core.LogDebugReturn("Query canceled: " + err.Error())
			}
			return nil, core.LogDebugReturn("Could not run MySQL query: " + err.Error())
		}
		return rows, nil
	}
	return nil, core.LogDebugReturn("Invalid sql method")
}
func (db *MySQL) runParamSQLUpdate(query string, params ...interface{}) (sql.Result, error) {
	queryUpper := strings.ToUpper(strings.TrimSpace(query))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if strings.HasPrefix(queryUpper, "SELECT") || strings.HasPrefix(queryUpper, "EXPLAIN") {
		return nil, core.LogDebugReturn("Invalid method for SQL update")
	}
	statement, err := db.database.PrepareContext(ctx, query)
	if err != nil {
		return nil, core.LogDebugReturn("Could not prepare MySQL query: " + query + " - " + err.Error())
	}
	defer statement.Close()
	result, err := statement.ExecContext(ctx, params...)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, core.LogDebugReturn("Query timed out after 15s: " + err.Error())
		}
		return nil, core.LogDebugReturn("Could not run MySQL query: " + query + " - " + err.Error())
	}
	return result, nil
}
func (db *MySQL) getRows(query string) (*sql.Rows, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stmt, err := db.database.PrepareContext(ctx, query)
	if err != nil {
		return nil, core.LogDebugReturn("prepare failed: " + err.Error())
	}
	defer stmt.Close()
	return stmt.QueryContext(ctx)
}
func (db *MySQL) execWithRetry(ctx context.Context, query string, maxRetries int) error {
	if db.database == nil {
		return core.LogDebugReturn("Database connection not initialized")
	}
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			_, err := db.database.ExecContext(ctx, query)
			if err == nil {
				return nil
			}
			lastErr = err
			time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
		}
	}
	return lastErr
}
func (db *MySQL) withTransaction(fn func(*sql.Tx) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := db.database.BeginTx(ctx, nil)
	if err != nil {
		return core.LogDebugReturn("Begin transaction failed: " + err.Error())
	}
	if err = fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return core.LogDebugReturn(fmt.Sprintf("Rollback failed: %v (original error: %w)", rbErr, err))
		}
		return err
	}
	if err = tx.Commit(); err != nil {
		return core.LogDebugReturn("Commit failed: " + err.Error())
	}
	return nil
}
func (db *MySQL) createTables(ctx context.Context) error {
	tables := map[string]string{
		"auth_expired":  "CREATE TABLE IF NOT EXISTS auth_expired (uuid VARCHAR(255) PRIMARY KEY, status VARCHAR(255))",
		"auth_nonce":    "CREATE TABLE IF NOT EXISTS auth_nonce (nonce VARCHAR(255) PRIMARY KEY, status VARCHAR(255), timestamp BIGINT)",
		"csrf_tokens":   "CREATE TABLE IF NOT EXISTS csrf_tokens (token VARCHAR(255) PRIMARY KEY, expiration BIGINT)",
		"file_txn_hash": "CREATE TABLE IF NOT EXISTS file_txn_hash (fileUUID VARCHAR(255), txHash VARCHAR(255), blockchain VARCHAR(255), PRIMARY KEY (fileUUID, txHash, blockchain))",
		"files":         "CREATE TABLE IF NOT EXISTS files (fileUUID VARCHAR(255) PRIMARY KEY, fileHash VARCHAR(255), mimeType VARCHAR(255), fileName VARCHAR(255), size BIGINT, addedDate BIGINT, cid VARCHAR(255), fileURL TEXT, source VARCHAR(255))",
		"login_nonce":   "CREATE TABLE IF NOT EXISTS login_nonce (nonce VARCHAR(512) PRIMARY KEY, domain VARCHAR(255), expiration BIGINT, nonceHash VARCHAR(255))",
		"meta":          "CREATE TABLE IF NOT EXISTS meta (`key` VARCHAR(255) PRIMARY KEY, value BLOB)",
		"notifications": "CREATE TABLE IF NOT EXISTS notifications (uid VARCHAR(255) PRIMARY KEY, message TEXT, timestamp BIGINT DEFAULT 0)",
		"oembed_cache":  "CREATE TABLE IF NOT EXISTS oembed_cache (url VARCHAR(512) PRIMARY KEY, data TEXT, fetchedAt BIGINT DEFAULT 0)",
		"settings":      "CREATE TABLE IF NOT EXISTS settings (`key` VARCHAR(255) PRIMARY KEY, value BLOB)",
		"wallets":       "CREATE TABLE IF NOT EXISTS wallets (publicKey VARCHAR(255), blockchain VARCHAR(255), address VARCHAR(255), encryptedPrivateKey BLOB, isDefault TINYINT DEFAULT 0, PRIMARY KEY (publicKey, blockchain))",
		// Base-specific tables
		"base_indexer_jobs":     "CREATE TABLE IF NOT EXISTS base_indexer_jobs (uuid VARCHAR(255) PRIMARY KEY, blockchain VARCHAR(255), headBlock BIGINT, status VARCHAR(255), tailBlock BIGINT, timestamp BIGINT, rps BIGINT DEFAULT 0)",
		"onchain_base_post":     "CREATE TABLE IF NOT EXISTS onchain_base_post (txHash VARCHAR(255), blockchain VARCHAR(255), fromAddress VARCHAR(255) DEFAULT '', parentTxHash VARCHAR(255) DEFAULT '', amount DOUBLE DEFAULT 0, timestamp BIGINT DEFAULT 0, data TEXT, PRIMARY KEY(txHash, blockchain))",
		"onchain_base_meta":     "CREATE TABLE IF NOT EXISTS onchain_base_meta (blockchain VARCHAR(255), address VARCHAR(255), name VARCHAR(255) DEFAULT '', avatar TEXT DEFAULT '', description TEXT DEFAULT '', location VARCHAR(255) DEFAULT '', banner TEXT DEFAULT '', website VARCHAR(255) DEFAULT '', vertical VARCHAR(255) DEFAULT '', server VARCHAR(255) DEFAULT '', colors TEXT DEFAULT '', blockchainTimestamp BIGINT DEFAULT 0, addressTimestamp BIGINT DEFAULT 0, nameTimestamp BIGINT DEFAULT 0, avatarTimestamp BIGINT DEFAULT 0, descriptionTimestamp BIGINT DEFAULT 0, locationTimestamp BIGINT DEFAULT 0, bannerTimestamp BIGINT DEFAULT 0, websiteTimestamp BIGINT DEFAULT 0, verticalTimestamp BIGINT DEFAULT 0, serverTimestamp BIGINT DEFAULT 0, colorsTimestamp BIGINT DEFAULT 0, PRIMARY KEY(blockchain, address))",
		"onchain_base_block":    "CREATE TABLE IF NOT EXISTS onchain_base_block (txHash VARCHAR(255), blockchain VARCHAR(255), blockerAddress VARCHAR(255), blockerBlockchain VARCHAR(255), blockeeAddress VARCHAR(255), blockeeBlockchain VARCHAR(255), `key` VARCHAR(255), value TEXT, timestamp BIGINT DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"onchain_base_follow":   "CREATE TABLE IF NOT EXISTS onchain_base_follow (txHash VARCHAR(255), blockchain VARCHAR(255), followerAddress VARCHAR(255), followerBlockchain VARCHAR(255), followeeAddress VARCHAR(255), followeeBlockchain VARCHAR(255), timestamp BIGINT DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"onchain_base_comment":  "CREATE TABLE IF NOT EXISTS onchain_base_comment (txHash VARCHAR(255), blockchain VARCHAR(255), fromAddress VARCHAR(255) DEFAULT '', parentTxHash VARCHAR(255) DEFAULT '', amount DOUBLE DEFAULT 0, timestamp BIGINT DEFAULT 0, data TEXT, PRIMARY KEY(txHash, blockchain))",
		"onchain_base_reaction": "CREATE TABLE IF NOT EXISTS onchain_base_reaction (txHash VARCHAR(255), blockchain VARCHAR(255), fromAddress VARCHAR(255) DEFAULT '', targetTxHash VARCHAR(255) DEFAULT '', targetType VARCHAR(255) DEFAULT 'post', reactionType VARCHAR(255) DEFAULT '', timestamp BIGINT DEFAULT 0, PRIMARY KEY(txHash, blockchain))",
		// Algorand-specific tables
		"algorand_indexer_jobs":     "CREATE TABLE IF NOT EXISTS algorand_indexer_jobs (uuid VARCHAR(255) PRIMARY KEY, blockchain VARCHAR(255), headBlock BIGINT, status VARCHAR(255), tailBlock BIGINT, timestamp BIGINT, rps BIGINT DEFAULT 0)",
		"onchain_algorand_post":     "CREATE TABLE IF NOT EXISTS onchain_algorand_post (txHash VARCHAR(255), blockchain VARCHAR(255), fromAddress VARCHAR(255) DEFAULT '', parentTxHash VARCHAR(255) DEFAULT '', amount DOUBLE DEFAULT 0, timestamp BIGINT DEFAULT 0, data TEXT, PRIMARY KEY(txHash, blockchain))",
		"onchain_algorand_meta":     "CREATE TABLE IF NOT EXISTS onchain_algorand_meta (blockchain VARCHAR(255), address VARCHAR(255), name VARCHAR(255) DEFAULT '', avatar TEXT DEFAULT '', description TEXT DEFAULT '', location VARCHAR(255) DEFAULT '', banner TEXT DEFAULT '', website VARCHAR(255) DEFAULT '', vertical VARCHAR(255) DEFAULT '', server VARCHAR(255) DEFAULT '', colors TEXT DEFAULT '', blockchainTimestamp BIGINT DEFAULT 0, addressTimestamp BIGINT DEFAULT 0, nameTimestamp BIGINT DEFAULT 0, avatarTimestamp BIGINT DEFAULT 0, descriptionTimestamp BIGINT DEFAULT 0, locationTimestamp BIGINT DEFAULT 0, bannerTimestamp BIGINT DEFAULT 0, websiteTimestamp BIGINT DEFAULT 0, verticalTimestamp BIGINT DEFAULT 0, serverTimestamp BIGINT DEFAULT 0, colorsTimestamp BIGINT DEFAULT 0, PRIMARY KEY(blockchain, address))",
		"onchain_algorand_block":    "CREATE TABLE IF NOT EXISTS onchain_algorand_block (txHash VARCHAR(255), blockchain VARCHAR(255), blockerAddress VARCHAR(255), blockerBlockchain VARCHAR(255), blockeeAddress VARCHAR(255), blockeeBlockchain VARCHAR(255), `key` VARCHAR(255), value TEXT, timestamp BIGINT DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"onchain_algorand_follow":   "CREATE TABLE IF NOT EXISTS onchain_algorand_follow (txHash VARCHAR(255), blockchain VARCHAR(255), followerAddress VARCHAR(255), followerBlockchain VARCHAR(255), followeeAddress VARCHAR(255), followeeBlockchain VARCHAR(255), timestamp BIGINT DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"onchain_algorand_comment":  "CREATE TABLE IF NOT EXISTS onchain_algorand_comment (txHash VARCHAR(255), blockchain VARCHAR(255), fromAddress VARCHAR(255) DEFAULT '', parentTxHash VARCHAR(255) DEFAULT '', amount DOUBLE DEFAULT 0, timestamp BIGINT DEFAULT 0, data TEXT, PRIMARY KEY(txHash, blockchain))",
		"onchain_algorand_reaction": "CREATE TABLE IF NOT EXISTS onchain_algorand_reaction (txHash VARCHAR(255), blockchain VARCHAR(255), fromAddress VARCHAR(255) DEFAULT '', targetTxHash VARCHAR(255) DEFAULT '', targetType VARCHAR(255) DEFAULT 'post', reactionType VARCHAR(255) DEFAULT '', timestamp BIGINT DEFAULT 0, PRIMARY KEY(txHash, blockchain))",
	}
	for _, createStatement := range tables {
		err := db.execWithRetry(ctx, createStatement, 3)
		if err != nil {
			return core.LogDebugReturn("Table creation failed: " + err.Error())
		}
	}
	return nil
}
func (db *MySQL) getSchemaVersion() int {
	rows, err := db.runParamSQLSelect("SELECT value FROM meta WHERE `key` = ?", "schema_version")
	if err != nil {
		return 0
	}
	defer rows.Close()
	if rows.Next() {
		var versionBytes []byte
		err = rows.Scan(&versionBytes)
		if err != nil {
			return 0
		}
		var version int
		_, err = fmt.Sscanf(string(versionBytes), "%d", &version)
		if err != nil {
			return 0
		}
		return version
	}
	return 0
}
func (db *MySQL) setSchemaVersion(version int) {
	query := "INSERT INTO meta (`key`, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value)"
	_, err := db.runParamSQLUpdate(query, "schema_version", fmt.Sprintf("%d", version))
	if err != nil {
		core.LogDebug("Could not set schema version: " + err.Error())
	}
}
func (db *MySQL) RunMigrations() error {
	currentVersion := db.getSchemaVersion()
	targetVersion := SchemaVersion
	if currentVersion == 0 {
		db.setSchemaVersion(targetVersion)
		core.LogDebug(fmt.Sprintf("MySQL: Fresh database initialized at schema version %d", targetVersion))
		return nil
	}
	if currentVersion > targetVersion {
		return core.LogDebugReturn(fmt.Sprintf("MySQL: database schema version (%d) is ahead of binary schema version (%d)", currentVersion, targetVersion))
	}
	if currentVersion == targetVersion {
		core.LogDebug(fmt.Sprintf("MySQL: Database schema is up to date (version %d)", currentVersion))
		return nil
	}
	core.LogDebug(fmt.Sprintf("MySQL: Upgrading database schema from version %d to %d", currentVersion, targetVersion))
	if currentVersion < 2 {
		if err := db.migrateV2MySQL(); err != nil {
			return core.LogDebugReturn("MySQL migration v2 failed: " + err.Error())
		}
		db.setSchemaVersion(2)
	}
	if currentVersion < 3 {
		if err := db.migrateV3MySQL(); err != nil {
			return core.LogDebugReturn("MySQL migration v3 failed: " + err.Error())
		}
		db.setSchemaVersion(3)
	}
	core.LogDebug(fmt.Sprintf("MySQL: Database schema upgrade completed (now at version %d)", targetVersion))
	return nil
}
func (db *MySQL) migrateV2MySQL() error {
	tables := []string{
		"CREATE TABLE IF NOT EXISTS onchain_comment (txHash VARCHAR(255), blockchain VARCHAR(50), fromAddress VARCHAR(255) DEFAULT '', parentTxHash VARCHAR(255) DEFAULT '', amount DOUBLE DEFAULT 0, timestamp BIGINT DEFAULT 0, data TEXT, PRIMARY KEY(txHash, blockchain))",
		"CREATE TABLE IF NOT EXISTS onchain_algorand_comment (txHash VARCHAR(255), blockchain VARCHAR(50), fromAddress VARCHAR(255) DEFAULT '', parentTxHash VARCHAR(255) DEFAULT '', amount DOUBLE DEFAULT 0, timestamp BIGINT DEFAULT 0, data TEXT, PRIMARY KEY(txHash, blockchain))",
		"CREATE TABLE IF NOT EXISTS onchain_reaction (txHash VARCHAR(255), blockchain VARCHAR(50), fromAddress VARCHAR(255) DEFAULT '', targetTxHash VARCHAR(255) DEFAULT '', targetType VARCHAR(50) DEFAULT 'post', reactionType VARCHAR(50) DEFAULT '', timestamp BIGINT DEFAULT 0, PRIMARY KEY(txHash, blockchain))",
		"CREATE TABLE IF NOT EXISTS onchain_algorand_reaction (txHash VARCHAR(255), blockchain VARCHAR(50), fromAddress VARCHAR(255) DEFAULT '', targetTxHash VARCHAR(255) DEFAULT '', targetType VARCHAR(50) DEFAULT 'post', reactionType VARCHAR(50) DEFAULT '', timestamp BIGINT DEFAULT 0, PRIMARY KEY(txHash, blockchain))",
	}
	for _, createStatement := range tables {
		if _, err := db.database.Exec(createStatement); err != nil {
			return err
		}
	}
	return nil
}
func (db *MySQL) migrateV3MySQL() error {
	if err := db.migrateAddColumn("onchain_base_meta", "colors", "TEXT"); err != nil {
		return err
	}
	if err := db.migrateAddColumn("onchain_base_meta", "colorsTimestamp", "BIGINT DEFAULT 0"); err != nil {
		return err
	}
	if err := db.migrateAddColumn("onchain_algorand_meta", "colors", "TEXT"); err != nil {
		return err
	}
	if err := db.migrateAddColumn("onchain_algorand_meta", "colorsTimestamp", "BIGINT DEFAULT 0"); err != nil {
		return err
	}
	return nil
}
func (db *MySQL) migrateAddColumn(table, column, definition string) error {
	query := fmt.Sprintf("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '%s' AND COLUMN_NAME = '%s'", table, column)
	rows, err := db.database.Query(query)
	if err != nil {
		return core.LogDebugReturn("MySQL: failed to check column existence: " + err.Error())
	}
	defer rows.Close()
	var count int
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return core.LogDebugReturn("MySQL: failed to scan column count: " + err.Error())
		}
	}
	if count > 0 {
		core.LogDebug(fmt.Sprintf("MySQL: Column %s.%s already exists, skipping", table, column))
		return nil
	}
	alterQuery := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)
	if _, err := db.database.Exec(alterQuery); err != nil {
		return core.LogDebugReturn(fmt.Sprintf("MySQL: failed to add column %s to %s: %s", column, table, err.Error()))
	}
	core.LogDebug(fmt.Sprintf("MySQL: Added column %s.%s", table, column))
	return nil
}
func (db *MySQL) ExportSnapshot(exportPath string) error {
	return core.LogDebugReturn("MySQL snapshot export not implemented - use mysqldump instead")
}
func (db *MySQL) ImportSnapshotNoMetadata(importPath string) error {
	return core.LogDebugReturn("MySQL snapshot import not implemented - use mysql import instead")
}
func (db *MySQL) ImportSnapshot(importPath string) error {
	return core.LogDebugReturn("MySQL snapshot import not implemented - use mysql import instead")
}
func (db *MySQL) Ping() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := db.database.PingContext(ctx)
	if err != nil {
		return false
	}
	rows, err := db.runParamSQLSelect("SELECT 1")
	if err != nil {
		return false
	}
	defer rows.Close()
	return true
}
func (db *MySQL) Close() error {
	return db.database.Close()
}

// --- Metadata & Settings --- //
func (db *MySQL) MetaUpdateValue(key string, value string) {
	query := "INSERT INTO meta (`key`, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value)"
	_, err := db.runParamSQLUpdate(query, key, value)
	if err != nil {
		core.LogDebug("Meta update failed: " + err.Error())
	}
}
func (db *MySQL) MetaGetValue(key string) string {
	rows, err := db.runParamSQLSelect("SELECT value FROM meta WHERE `key` = ?", key)
	if err != nil {
		core.LogDebug("Could not get meta value, query failed: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var value []byte
		err = rows.Scan(&value)
		if err != nil {
			return ""
		}
		return string(value)
	}
	return ""
}
func (db *MySQL) SettingsGetValue(key string) string {
	startupCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		select {
		case <-startupCtx.Done():
			return ""
		default:
			rows, err := db.runParamSQLSelect("SELECT value FROM settings WHERE `key` = ?", key)
			if err != nil {
				if strings.Contains(err.Error(), "context canceled") {
					backoff := time.Duration(1<<uint(i)) * time.Second
					core.LogWarn("Settings query failed, retrying...")
					time.Sleep(backoff)
					continue
				}
				core.LogDebug("Could not get setting value for key: " + key + " - query failed: " + err.Error())
				return ""
			}
			defer rows.Close()
			for rows.Next() {
				var value []byte
				err = rows.Scan(&value)
				if err != nil {
					return ""
				}
				return string(value)
			}
		}
	}
	return ""
}
func (db *MySQL) SettingsUpdateValue(key string, value string) {
	query := "INSERT INTO settings (`key`, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value)"
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		_, err := db.runParamSQLUpdate(query, key, value)
		if err == nil {
			return
		}
		if strings.Contains(err.Error(), "Lock wait timeout") || strings.Contains(err.Error(), "Deadlock") {
			backoff := time.Duration(100*(1<<uint(i))) * time.Millisecond
			core.LogWarn("Settings update locked, retrying after " + backoff.String() + "...")
			time.Sleep(backoff)
			continue
		}
		core.LogDebug("Settings update failed: " + err.Error())
		return
	}
	core.LogDebug("Settings update failed after " + fmt.Sprint(maxRetries) + " retries")
}
func (db *MySQL) SettingsDeleteValue(key string) error {
	_, err := db.runParamSQLUpdate("DELETE FROM settings WHERE `key` = ?", key)
	if err != nil {
		return core.LogDebugReturn("Could not delete setting: " + err.Error())
	}
	return nil
}

// --- Profile --- //
func (db *MySQL) ProfileGetName(address string, blockchain string) string {
	query := fmt.Sprintf("SELECT name FROM onchain_%s_meta WHERE address = LOWER(?) AND blockchain = ?", blockchain)
	rows, err := db.runParamSQLSelect(query, address, blockchain)
	if err != nil {
		core.LogDebug("Could not get profile name from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			core.LogDebug("Could not get profile name from database: " + err.Error())
		}
		return name
	}
	return ""
}
func (db *MySQL) ProfileGetAvatar(address string, blockchain string) string {
	query := fmt.Sprintf("SELECT avatar FROM onchain_%s_meta WHERE address = LOWER(?) AND blockchain = ?", blockchain)
	rows, err := db.runParamSQLSelect(query, address, blockchain)
	if err != nil {
		core.LogDebug("could not get profile avatar from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var avatar string
		err = rows.Scan(&avatar)
		if err != nil {
			core.LogDebug("could not parse database rows for profile avatar: " + err.Error())
			return ""
		}
		return avatar
	}
	return ""
}
func (db *MySQL) ProfileGetBanner(address string, blockchain string) string {
	query := fmt.Sprintf("SELECT banner FROM onchain_%s_meta WHERE address = LOWER(?) AND blockchain = ?", blockchain)
	rows, err := db.runParamSQLSelect(query, address, blockchain)
	if err != nil {
		core.LogDebug("Could not get profile banner from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var banner string
		err = rows.Scan(&banner)
		if err != nil {
			core.LogDebug("Could not parse database rows for profile banner")
			return ""
		}
		return banner
	}
	return ""
}
func (db *MySQL) ProfileGetColors(address string, blockchain string) string {
	query := fmt.Sprintf("SELECT COALESCE(colors, '') FROM onchain_%s_meta WHERE address = ? AND blockchain = ?", blockchain)
	rows, err := db.runParamSQLSelect(query, address, blockchain)
	if err != nil {
		core.LogDebug("Could not get profile colors from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var colors string
		err = rows.Scan(&colors)
		if err != nil {
			core.LogDebug("Could not parse database rows for profile colors: " + err.Error())
			return ""
		}
		return colors
	}
	return ""
}
func (db *MySQL) ProfileGetDescription(address string, blockchain string) string {
	query := fmt.Sprintf("SELECT description FROM onchain_%s_meta WHERE address = LOWER(?) AND blockchain = ?", blockchain)
	rows, err := db.runParamSQLSelect(query, address, blockchain)
	if err != nil {
		core.LogDebug("Could not get profile description from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var description string
		err = rows.Scan(&description)
		if err != nil {
			core.LogDebug("Could not parse database rows for profile description: " + err.Error())
			return ""
		}
		return description
	}
	return ""
}
func (db *MySQL) ProfileGetLocation(address string, blockchain string) string {
	query := fmt.Sprintf("SELECT location FROM onchain_%s_meta WHERE address = LOWER(?) AND blockchain = ?", blockchain)
	rows, err := db.runParamSQLSelect(query, address, blockchain)
	if err != nil {
		core.LogDebug("Could not get profile location from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var location string
		err = rows.Scan(&location)
		if err != nil {
			core.LogDebug("Could not parse database rows for profile location: " + err.Error())
			return ""
		}
		return location
	}
	return ""
}
func (db *MySQL) ProfileGetWebsite(address string, blockchain string) string {
	query := fmt.Sprintf("SELECT website FROM onchain_%s_meta WHERE address = LOWER(?) AND blockchain = ?", blockchain)
	rows, err := db.runParamSQLSelect(query, address, blockchain)
	if err != nil {
		core.LogDebug("Could not get profile website from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var website string
		err = rows.Scan(&website)
		if err != nil {
			core.LogDebug("Could not parse database rows for profile website: " + err.Error())
			return ""
		}
		return website
	}
	return ""
}
func (db *MySQL) ProfileGetVertical(address string, blockchain string) string {
	query := fmt.Sprintf("SELECT vertical FROM onchain_%s_meta WHERE address = LOWER(?) AND blockchain = ?", blockchain)
	rows, err := db.runParamSQLSelect(query, address, blockchain)
	if err != nil {
		core.LogDebug("Could not get profile vertical from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var vertical string
		err = rows.Scan(&vertical)
		if err != nil {
			core.LogDebug("Could not parse database rows for profile vertical: " + err.Error())
			return ""
		}
		return vertical
	}
	return ""
}
func (db *MySQL) ProfileGetJoinedDate(address string, blockchain string) *int64 {
	var metaAge int64 = 0
	var postAge int64 = 0
	var joinedDate int64 = 0
	query := fmt.Sprintf("SELECT COALESCE(MIN(CASE WHEN blockchainTimestamp > 0 THEN blockchainTimestamp WHEN addressTimestamp > 0 THEN addressTimestamp WHEN nameTimestamp > 0 THEN nameTimestamp WHEN avatarTimestamp > 0 THEN avatarTimestamp WHEN descriptionTimestamp > 0 THEN descriptionTimestamp WHEN locationTimestamp > 0 THEN locationTimestamp WHEN bannerTimestamp > 0 THEN bannerTimestamp WHEN websiteTimestamp > 0 THEN websiteTimestamp WHEN verticalTimestamp > 0 THEN verticalTimestamp WHEN serverTimestamp > 0 THEN serverTimestamp ELSE 0 END), 0) AS min_timestamp FROM onchain_%s_meta WHERE blockchain = ? AND address = LOWER(?)", blockchain)
	rowsmeta, err := db.runParamSQLSelect(query, blockchain, address)
	if err == nil {
		if rowsmeta != nil {
			defer rowsmeta.Close()
			for rowsmeta.Next() {
				err = rowsmeta.Scan(&metaAge)
				if err != nil {
					core.LogDebug("Could not parse database rows for profile joined date: " + err.Error())
					return nil
				}
			}
		}
	} else {
		core.LogDebug("Could not parse database rows for profile joined date: " + err.Error())
		return nil
	}
	query2 := fmt.Sprintf("SELECT timestamp FROM onchain_%s_post WHERE fromAddress = LOWER(?) AND blockchain = ?", blockchain)
	rowsposts, err := db.runParamSQLSelect(query2, address, blockchain)
	if err == nil {
		if rowsposts != nil {
			defer rowsposts.Close()
			for rowsposts.Next() {
				var newAge int64
				err = rowsposts.Scan(&newAge)
				if err != nil {
					core.LogDebug("Could not get profile joined date from database: " + err.Error())
					return nil
				}
				if newAge < postAge || postAge == 0 {
					postAge = newAge
				}
			}
		}
	} else {
		core.LogDebug("Could not get profile joined date from database: " + err.Error())
		return nil
	}
	if metaAge > 0 && metaAge < postAge {
		joinedDate = metaAge
	} else {
		joinedDate = postAge
	}
	if joinedDate == 0 {
		return nil
	}
	return &joinedDate
}
func (db *MySQL) ProfileGetPosts(address string, blockchain string) []map[string]interface{} {
	var posts []map[string]interface{}
	query := fmt.Sprintf("SELECT txHash, COALESCE(parentTxHash, '') as parentTxHash, timestamp, data FROM onchain_%s_post WHERE fromAddress = LOWER(?) AND blockchain = ? AND data IS NOT NULL ORDER BY timestamp DESC", blockchain)
	rowsPosts, err := db.runParamSQLSelect(query, address, blockchain)
	if err != nil {
		core.LogDebug("Could not get user posts from database: " + err.Error())
		return nil
	}
	if rowsPosts == nil {
		core.LogDebug("No posts found for address: " + address + " on blockchain: " + blockchain)
		return nil
	}
	defer rowsPosts.Close()
	for rowsPosts.Next() {
		var timestamp uint64
		var txHash, payload, parent string
		var attachments [][]interface{}
		err := rowsPosts.Scan(&txHash, &parent, &timestamp, &payload)
		if err != nil {
			core.LogDebug("Could not scan database rows for user posts: " + err.Error())
			return nil
		}
		sqlQuery := "SELECT f.mimeType, f.size, f.fileUrl, f.fileName FROM files f INNER JOIN file_txn_hash fth ON f.fileUUID = fth.fileUUID WHERE fth.txHash = ? AND fth.blockchain = ?"
		rowsAttachments, err := db.runParamSQLSelect(sqlQuery, txHash, blockchain)
		if err != nil {
			core.LogDebug("Could not get attachments for post: " + err.Error())
		} else if rowsAttachments != nil {
			for rowsAttachments.Next() {
				var mimeType string
				var size uint64
				var fileUrl string
				var fileName string
				err := rowsAttachments.Scan(&mimeType, &size, &fileUrl, &fileName)
				if err != nil {
					core.LogDebug("Could parse rows for post attachment: " + err.Error())
					break
				}
				attachment := []interface{}{fileUrl, mimeType, size, fileName}
				attachments = append(attachments, attachment)
			}
			rowsAttachments.Close()
		}
		commentCount := db.GetCommentCount(txHash, blockchain)
		post := map[string]interface{}{
			"resultType":   "profile post",
			"txHash":       txHash,
			"parent":       parent,
			"timestamp":    timestamp,
			"payload":      payload,
			"blockchain":   blockchain,
			"address":      address,
			"commentCount": commentCount,
		}
		if attachments != nil {
			post["attachments"] = attachments
		}
		posts = append(posts, post)
	}
	return posts
}
func (db *MySQL) ProfileGetFollowerCount(address string, blockchain string) *int64 {
	var postCount int64 = 0
	query := fmt.Sprintf("SELECT COUNT(*) FROM onchain_%s_follow WHERE followeeAddress = ? AND followeeBlockchain = ?", blockchain)
	rows, err := db.runParamSQLSelect(query, address, blockchain)
	if err != nil {
		core.LogDebug("Could not get follower count from database: " + err.Error())
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&postCount)
		if err != nil {
			core.LogDebug("Could not parse database rows for follower count: " + err.Error())
			return nil
		}
	}
	return &postCount
}
func (db *MySQL) ProfileGetFollowingCount(address string, blockchain string) *int64 {
	var postCount int64 = 0
	query := fmt.Sprintf("SELECT COUNT(*) FROM onchain_%s_follow WHERE followerAddress = ? AND followerBlockchain = ?", blockchain)
	rows, err := db.runParamSQLSelect(query, address, blockchain)
	if err != nil {
		core.LogDebug("Could not get following count from database: " + err.Error())
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&postCount)
		if err != nil {
			core.LogDebug("Could not parse database rows for following count: " + err.Error())
			return nil
		}
	}
	return &postCount
}
func (db *MySQL) ProfileIsFollower(followeeAddress string, followeeBlockchain string, followerAddress string, followerBlockchain string) bool {
	query := fmt.Sprintf("SELECT COUNT(*) FROM onchain_%s_follow WHERE followeeAddress = ? AND followeeBlockchain = ? AND followerAddress = ? AND followerBlockchain = ?", followeeBlockchain)
	rows, err := db.runParamSQLSelect(query, followeeAddress, followeeBlockchain, followerAddress, followerBlockchain)
	if err != nil {
		core.LogDebug("Could not get follower status from database: " + err.Error())
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var count int
		err = rows.Scan(&count)
		if err != nil {
			core.LogDebug("Could not parse database rows for follower status: " + err.Error())
			return false
		}
		if count > 0 {
			return true
		}
	}
	return false
}

// --- Search --- //
func (db *MySQL) SearchGetPosts(query string) []map[string]interface{} {
	var posts []map[string]interface{}
	for _, _blockchain := range core.ValidNetworks {
		selectionQueryFmt := "SELECT txHash, COALESCE(parentTxHash, '') as parentHash, timestamp, data, fromAddress, blockchain FROM onchain_%s_post WHERE LOWER(data) LIKE LOWER(?)"
		selectionQuery := fmt.Sprintf(selectionQueryFmt, _blockchain)
		search := "%" + query + "%"
		rows, err := db.runParamSQLSelect(selectionQuery, search)
		if err != nil {
			core.LogDebug("Could not get searched posts from database: " + err.Error())
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var timestamp uint64
			var txHash, parentHash, payload, blockchain, address string
			var attachments [][]interface{}
			err := rows.Scan(&txHash, &parentHash, &timestamp, &payload, &address, &blockchain)
			if err != nil {
				core.LogDebug("Could not scan database rows: " + err.Error())
				return nil
			}
			sqlQuery := "SELECT f.mimeType, f.size, f.fileUrl, f.fileName FROM files f INNER JOIN file_txn_hash fth ON f.fileUUID = fth.fileUUID WHERE fth.txHash = ? AND fth.blockchain = ?"
			rowsAttachments, err := db.runParamSQLSelect(sqlQuery, txHash, blockchain)
			if err != nil {
				core.LogDebug("Could not get attachments for post: " + err.Error())
			}
			if rowsAttachments != nil {
				defer rowsAttachments.Close()
				for rowsAttachments.Next() {
					var mimeType string
					var size uint64
					var fileURL string
					var fileName string
					err := rowsAttachments.Scan(&mimeType, &size, &fileURL, &fileName)
					if err != nil {
						core.LogDebug("Could parse rows for post attachment: " + err.Error())
						break
					}
					attachment := []interface{}{fileURL, mimeType, size, fileName}
					attachments = append(attachments, attachment)
				}
			}
			post := map[string]interface{}{
				"resultType": "post",
				"blockchain": blockchain,
				"address":    address,
				"txHash":     txHash,
				"timestamp":  timestamp,
				"payload":    payload,
				"parentHash": parentHash,
			}
			if attachments != nil {
				post["attachments"] = attachments
			}
			posts = append(posts, post)
		}
	}
	return posts
}
func (db *MySQL) SearchGetProfiles(query string) []map[string]interface{} {
	var profiles []map[string]interface{}
	search := "%" + query + "%"
	searchPrefix := query + "%"
	for _, _blockchain := range core.ValidNetworks {
		sqlQueryFmt := "SELECT address, blockchain FROM onchain_%s_meta WHERE address LIKE ? OR name LIKE ? OR name LIKE ? OR name LIKE ? OR name LIKE ? OR name LIKE ?"
		sqlQuery := fmt.Sprintf(sqlQueryFmt, _blockchain)
		rows, err := db.runParamSQLSelect(sqlQuery, search, search, searchPrefix, searchPrefix+".eth", searchPrefix+".base.eth", searchPrefix+".algo")
		if err != nil {
			core.LogDebug("Could not get searched profiles from database: " + err.Error())
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var address, blockchain string
			err = rows.Scan(&address, &blockchain)
			profile := map[string]interface{}{
				"resultType": "profile",
				"address":    address,
				"blockchain": blockchain,
			}
			if err != nil {
				core.LogDebug("Could not parse posts from database rows")
				return nil
			}
			profiles = append(profiles, profile)
		}
	}
	return profiles
}
func (db *MySQL) DiscoverGetRandomProfiles(limit int) []map[string]interface{} {
	var profiles []map[string]interface{}
	for _, _blockchain := range core.ValidNetworks {
		sqlQueryFmt := "SELECT address, blockchain FROM (SELECT address, blockchain FROM onchain_%s_meta UNION SELECT followeeAddress, followeeBlockchain FROM onchain_%s_follow) t ORDER BY RAND() LIMIT ?"
		sqlQuery := fmt.Sprintf(sqlQueryFmt, _blockchain, _blockchain)
		rows, err := db.runParamSQLSelect(sqlQuery, limit)
		if err != nil {
			core.LogDebug("Could not get random profiles from database: " + err.Error())
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var address, blockchain string
			err = rows.Scan(&address, &blockchain)
			if err != nil {
				core.LogDebug("Could not parse random profiles from database rows: " + err.Error())
				return nil
			}
			profile := map[string]interface{}{
				"address":    address,
				"blockchain": blockchain,
			}
			profiles = append(profiles, profile)
		}
	}
	return profiles
}
func (db *MySQL) DiscoverGetTopByFollowers(limit int) []map[string]interface{} {
	var profiles []map[string]interface{}
	for _, _blockchain := range core.ValidNetworks {
		sqlQueryFmt := "SELECT followeeAddress, followeeBlockchain, COUNT(*) as follower_count FROM onchain_%s_follow GROUP BY followeeAddress, followeeBlockchain ORDER BY follower_count DESC LIMIT ?"
		sqlQuery := fmt.Sprintf(sqlQueryFmt, _blockchain)
		rows, err := db.runParamSQLSelect(sqlQuery, limit)
		if err != nil {
			core.LogDebug("Could not get top profiles by followers from database: " + err.Error())
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var address, blockchain string
			var followerCount int
			err = rows.Scan(&address, &blockchain, &followerCount)
			if err != nil {
				core.LogDebug("Could not parse top follower profiles from database rows: " + err.Error())
				return nil
			}
			profile := map[string]interface{}{
				"address":       address,
				"blockchain":    blockchain,
				"followerCount": followerCount,
			}
			profiles = append(profiles, profile)
		}
	}
	return profiles
}
func (db *MySQL) DiscoverGetTopByPosts(limit int) []map[string]interface{} {
	var profiles []map[string]interface{}
	for _, _blockchain := range core.ValidNetworks {
		sqlQueryFmt := "SELECT fromAddress, blockchain, COUNT(*) as post_count FROM onchain_%s_post GROUP BY fromAddress, blockchain ORDER BY post_count DESC LIMIT ?"
		sqlQuery := fmt.Sprintf(sqlQueryFmt, _blockchain)
		rows, err := db.runParamSQLSelect(sqlQuery, limit)
		if err != nil {
			core.LogDebug("Could not get top profiles by posts from database: " + err.Error())
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var address, blockchain string
			var postCount int
			err = rows.Scan(&address, &blockchain, &postCount)
			if err != nil {
				core.LogDebug("Could not parse top post profiles from database rows: " + err.Error())
				return nil
			}
			profile := map[string]interface{}{
				"address":    address,
				"blockchain": blockchain,
				"postCount":  postCount,
			}
			profiles = append(profiles, profile)
		}
	}
	return profiles
}

// --- Auth --- //
func (db *MySQL) AuthGetNonceStatus(nonce string) string {
	rows, err := db.runParamSQLSelect("SELECT status FROM auth_nonce WHERE nonce = ?", nonce)
	if err != nil {
		core.LogDebug("Could not get nonce status from the database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			core.LogDebug("Could not get the rows from getting the nonce status from the database: " + err.Error())
			return ""
		}
		return value
	}
	return ""
}
func (db *MySQL) AuthUpdateNonce(nonce string, status string) {
	_, err := db.runParamSQLUpdate("INSERT INTO auth_nonce (nonce, status) VALUES (?, ?) ON DUPLICATE KEY UPDATE status = VALUES(status)", nonce, status)
	if err != nil {
		core.LogDebug("Could not update auth nonce in database: " + err.Error())
	}
}
func (db *MySQL) AuthDeleteNonce(nonce string) {
	_, err := db.runParamSQLUpdate("DELETE FROM auth_nonce WHERE nonce = ?", nonce)
	if err != nil {
		core.LogDebug("Could not delete the auth nonce from the database: " + err.Error())
	}
}
func (db *MySQL) AuthExpireCookie(uuid string) {
	_, err := db.runParamSQLUpdate("INSERT INTO auth_expired (uuid, status) VALUES (?, 'expired') ON DUPLICATE KEY UPDATE status = 'expired'", uuid)
	if err != nil {
		core.LogDebug("Could not expire the auth cookie from the database: " + err.Error())
	}
}
func (db *MySQL) AuthGetCookieStatus(uuid string) string {
	rows, err := db.runParamSQLSelect("SELECT status FROM auth_expired WHERE uuid = ?", uuid)
	if err != nil {
		core.LogDebug("Could not get the auth cookie status from the database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			core.LogDebug("Could not get the rows from the auth cookie status from the database: " + err.Error())
			return ""
		}
		return value
	}
	return ""
}
func (db *MySQL) AuthUpdateLoginNonce(nonce string, domain string, expiration uint64, nonceHash string) {
	query := "INSERT INTO login_nonce (nonce, domain, expiration, nonceHash) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE nonce = nonce"
	_, err := db.runParamSQLUpdate(query, nonce, domain, expiration, nonceHash)
	if err != nil {
		core.LogDebug("Could not update login_nonce: " + err.Error())
	}
}
func (db *MySQL) AuthGetLoginNonceByHash(nonceHash string) string {
	currentTime := core.GetTimestamp()
	rows, err := db.runParamSQLSelect("SELECT nonce FROM login_nonce WHERE nonceHash = ? AND expiration > ?", nonceHash, currentTime)
	if err != nil {
		core.LogDebug("Could not get login nonce by hash: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var nonce string
		err = rows.Scan(&nonce)
		if err != nil {
			core.LogDebug("Could not scan login nonce: " + err.Error())
			return ""
		}
		return nonce
	}
	return ""
}
func (db *MySQL) AuthDeleteLoginNonce(nonce string) {
	_, err := db.runParamSQLUpdate("DELETE FROM login_nonce WHERE nonce = ?", nonce)
	if err != nil {
		core.LogDebug("Could not delete the login nonce from the database: " + err.Error())
	}
}
func (db *MySQL) AuthExpireLoginNonce() {
	_, err := db.runParamSQLUpdate("DELETE FROM login_nonce WHERE expiration < ?", core.GetTimestamp())
	if err != nil {
		core.LogDebug("Could not delete any expired login nonces from the database: " + err.Error())
	}
}
func (db *MySQL) AuthGetServerOwnerAddress() string {
	rows, err := db.runParamSQLSelect("SELECT value FROM meta WHERE `key` = 'accountAddress' LIMIT 1")
	if err != nil {
		core.LogDebug("Could not get the server owner address from the database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			core.LogDebug("Could not get the rows from the server owner address from the database: " + err.Error())
			return ""
		}
		return value
	}
	core.LogDebug("Could not get the server owner address from the database - no entry found")
	return ""
}
func (db *MySQL) AuthGetServerOwnerNetwork() string {
	rows, err := db.runParamSQLSelect("SELECT value FROM meta WHERE `key` = 'accountNetwork' LIMIT 1")
	if err != nil {
		core.LogDebug("Could not get the server owner network from the database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			core.LogDebug("Could not get the rows from the server owner network from the database: " + err.Error())
			return ""
		}
		return value
	}
	core.LogDebug("Could not get the server owner network from the database - no entry found")
	return ""
}

// --- File & IPFS --- //
func (db *MySQL) FileAdd(fileUUID string, fileHash string, mimeType string, fileName string, size int64) {
	query := "INSERT IGNORE INTO files (fileUUID, fileHash, mimeType, fileName, size, addedDate) VALUES (?, ?, ?, ?, ?, ?)"
	_, err := db.runParamSQLUpdate(query, fileUUID, fileHash, mimeType, fileName, size, core.GetTimestamp())
	if err != nil {
		core.LogDebug("Could not add the file to the database: " + err.Error())
	}
}
func (db *MySQL) IPFSAdd(fileUUID string, cid string) {
	fileURL := "ipfs://" + cid
	query := "UPDATE files SET cid = ?, fileURL = ? WHERE fileUUID = ?"
	_, err := db.runParamSQLUpdate(query, cid, fileURL, fileUUID)
	if err != nil {
		core.LogDebug("Could not add the IPFS CID to the database: " + err.Error())
	}
}
func (db *MySQL) GetFileHashFromUUID(uuid string) string {
	rows, err := db.runParamSQLSelect("SELECT fileHash FROM files WHERE fileUUID = ?", uuid)
	if err != nil {
		core.LogDebug("Could not get the hash from the UUID: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var fileHash string
		err = rows.Scan(&fileHash)
		if err != nil {
			core.LogDebug("Could not get the hash from the UUID: " + err.Error())
			return ""
		}
		return fileHash
	}
	return ""
}

// --- Indexer --- //
func (db *MySQL) IndexerCreateJob(uuid string, blockchain string) {
	core.LogDebug("IndexerCreateJob(): " + uuid + " - " + blockchain)
	timestamp := core.GetTimestamp()
	queryFmt := "INSERT INTO %s_indexer_jobs (uuid, blockchain, headBlock, status, tailBlock, timestamp, rps) VALUES (?, ?, 0, 'pending', 0, ?, 0) ON DUPLICATE KEY UPDATE status = VALUES(status), tailBlock = VALUES(tailBlock), timestamp = VALUES(timestamp)"
	query := fmt.Sprintf(queryFmt, blockchain)
	core.LogDebug("IndexerCreateJob(): " + query)
	_, err := db.runParamSQLUpdate(query, uuid, blockchain, timestamp)
	if err != nil {
		core.LogDebug("Could not create indexer job in the database: " + err.Error())
	}
}
func (db *MySQL) IndexerGetJobUUID(blockchain string) string {
	queryFmt := "SELECT uuid FROM %s_indexer_jobs WHERE blockchain = ?"
	query := fmt.Sprintf(queryFmt, blockchain)
	rows, err := db.runParamSQLSelect(query, blockchain)
	if err != nil {
		core.LogDebug("Could not get the indexer job UUID from the database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			core.LogDebug("Could not get the rows for the indexer job UUID from the database: " + err.Error())
			return ""
		}
		return value
	}
	return ""
}
func (db *MySQL) IndexerGetJobStatus(uuid string) string {
	for _, blockchain := range core.ValidNetworks {
		queryFmt := "SELECT status FROM %s_indexer_jobs WHERE uuid = ?"
		query := fmt.Sprintf(queryFmt, blockchain)
		rows, err := db.runParamSQLSelect(query, uuid)
		if err != nil {
			continue
		}
		defer rows.Close()
		for rows.Next() {
			var value string
			err = rows.Scan(&value)
			if err != nil {
				continue
			}
			return value
		}
	}
	return ""
}
func (db *MySQL) IndexerGetHeadBlock(uuid string) uint64 {
	for _, blockchain := range core.ValidNetworks {
		queryFmt := "SELECT headBlock FROM %s_indexer_jobs WHERE uuid = ?"
		query := fmt.Sprintf(queryFmt, blockchain)
		rows, err := db.runParamSQLSelect(query, uuid)
		if err != nil {
			continue
		}
		defer rows.Close()
		for rows.Next() {
			var value uint64
			err = rows.Scan(&value)
			if err != nil {
				continue
			}
			return value
		}
	}
	return 0
}
func (db *MySQL) IndexerGetTailBlock(uuid string) uint64 {
	for _, blockchain := range core.ValidNetworks {
		queryFmt := "SELECT tailBlock FROM %s_indexer_jobs WHERE uuid = ?"
		query := fmt.Sprintf(queryFmt, blockchain)
		rows, err := db.runParamSQLSelect(query, uuid)
		if err != nil {
			continue
		}
		defer rows.Close()
		for rows.Next() {
			var value uint64
			err = rows.Scan(&value)
			if err != nil {
				continue
			}
			return value
		}
	}
	return 0
}
func (db *MySQL) IndexerGetRunningJobsUUIDs() []string {
	var uuids []string
	for _, blockchain := range core.ValidNetworks {
		queryFmt := "SELECT uuid FROM %s_indexer_jobs WHERE status = 'running'"
		query := fmt.Sprintf(queryFmt, blockchain)
		rows, err := db.getRows(query)
		if err != nil {
			continue
		}
		defer rows.Close()
		for rows.Next() {
			var value string
			err = rows.Scan(&value)
			if err != nil {
				continue
			}
			uuids = append(uuids, value)
		}
	}
	return uuids
}
func (db *MySQL) IndexerUpdateJobStatus(uuid string, status string) {
	timestamp := core.GetTimestamp()
	for _, blockchain := range core.ValidNetworks {
		queryFmt := "UPDATE %s_indexer_jobs SET status = ?, timestamp = ? WHERE uuid = ?"
		query := fmt.Sprintf(queryFmt, blockchain)
		_, err := db.runParamSQLUpdate(query, status, timestamp, uuid)
		if err != nil {
			core.LogDebug("Could not update the indexer job status in the database: " + err.Error())
		}
	}
}
func (db *MySQL) IndexerUpdateHeadBlock(uuid string, headBlock uint64) {
	for _, blockchain := range core.ValidNetworks {
		queryFmt := "UPDATE %s_indexer_jobs SET headBlock = ?, timestamp = ? WHERE uuid = ?"
		query := fmt.Sprintf(queryFmt, blockchain)
		_, err := db.runParamSQLUpdate(query, headBlock, core.GetTimestamp(), uuid)
		if err != nil {
			core.LogDebug("Could not update the indexer head block in the database: " + err.Error())
		}
	}
}
func (db *MySQL) IndexerUpdateTailBlock(uuid string, tailBlock uint64) {
	for _, blockchain := range core.ValidNetworks {
		queryFmt := "UPDATE %s_indexer_jobs SET tailBlock = ?, timestamp = ? WHERE uuid = ?"
		query := fmt.Sprintf(queryFmt, blockchain)
		_, err := db.runParamSQLUpdate(query, tailBlock, core.GetTimestamp(), uuid)
		if err != nil {
			core.LogDebug("Could not update the indexer tail block in the database: " + err.Error())
		}
	}
}
func (db *MySQL) IndexerUpdateJobSpeed(uuid string, speed uint64) {
	for _, blockchain := range core.ValidNetworks {
		queryFmt := "UPDATE %s_indexer_jobs SET rps = ? WHERE uuid = ?"
		query := fmt.Sprintf(queryFmt, blockchain)
		_, err := db.runParamSQLUpdate(query, speed, uuid)
		if err != nil {
			if strings.Contains(err.Error(), "Lock wait timeout") || strings.Contains(err.Error(), "Deadlock") {
				return
			}
			core.LogDebug("Could not update the indexer job speed in the database: " + err.Error())
		}
	}
}
func (db *MySQL) IndexerAddPost(txHash string, blockchain string, fromAddress string, toAddress string, parentTxHash string, amount uint64, timestamp uint64, data string, blockNumber uint64) {
	queryFmt := "INSERT IGNORE INTO onchain_%s_post (txHash, blockchain, fromAddress, parentTxHash, amount, timestamp, data) VALUES (?, ?, ?, ?, ?, ?, ?)"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddress, parentTxHash, amount, timestamp, data)
	if err != nil {
		core.LogDebug("Could not add a post from the indexer into the database: " + err.Error())
	}
}
func (db *MySQL) IndexerResetJobs(blockchain string) {
	queryFmt := "UPDATE %s_indexer_jobs SET status = 'pending', headBlock = 0, tailBlock = 0, timestamp = ? WHERE blockchain = ?"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, core.GetTimestamp(), blockchain)
	if err != nil {
		core.LogDebug("Could not reset the indexer in the database: " + err.Error())
	}
	queryFmt = "DELETE FROM onchain_%s_post WHERE blockchain = ?"
	query = fmt.Sprintf(queryFmt, blockchain)
	_, err = db.runParamSQLUpdate(query, blockchain)
	if err != nil {
		core.LogDebug("Could not clear onchain_" + blockchain + "_post for " + blockchain + ": " + err.Error())
	}
	queryFmt = "DELETE FROM onchain_%s_meta WHERE blockchain = ?"
	query = fmt.Sprintf(queryFmt, blockchain)
	_, err = db.runParamSQLUpdate(query, blockchain)
	if err != nil {
		core.LogDebug("Could not clear onchain_" + blockchain + "_meta for " + blockchain + ": " + err.Error())
	}
	queryFmt = "DELETE FROM onchain_%s_follow WHERE followerBlockchain = ? OR followeeBlockchain = ?"
	query = fmt.Sprintf(queryFmt, blockchain)
	_, err = db.runParamSQLUpdate(query, blockchain, blockchain)
	if err != nil {
		core.LogDebug("Could not clear onchain_" + blockchain + "_follow for " + blockchain + ": " + err.Error())
	}
	queryFmt = "DELETE FROM onchain_%s_block WHERE blockerBlockchain = ? OR blockeeBlockchain = ?"
	query = fmt.Sprintf(queryFmt, blockchain)
	_, err = db.runParamSQLUpdate(query, blockchain, blockchain)
	if err != nil {
		core.LogDebug("Could not clear onchain_" + blockchain + "_block for " + blockchain + ": " + err.Error())
	}
	core.LogDebug("Cleared all onchain data for " + blockchain)
}

// --- Onchain Tokenized --- //
func (db *MySQL) OnchainC(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string) {
	queryFmt := "INSERT IGNORE INTO onchain_%s_comment (txHash, blockchain, fromAddress, parentTxHash, amount, timestamp, data) VALUES (?, ?, ?, ?, ?, ?, ?)"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data)
	if err != nil {
		core.LogDebug("Could not tokenize the comment in the database: " + err.Error())
	}
}
func (db *MySQL) OnchainCA(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string, attachments []Attachment) {
	queryFmt := "INSERT IGNORE INTO onchain_%s_comment (txHash, blockchain, fromAddress, parentTxHash, amount, timestamp, data) VALUES (?, ?, ?, ?, ?, ?, ?)"
	query := fmt.Sprintf(queryFmt, blockchain)
	result, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data)
	if err != nil {
		core.LogDebug("Could not tokenize the comment in the database: " + err.Error())
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		core.LogDebug("Could not count the comment in the database: " + err.Error())
		return
	}
	if rowsAffected == 0 {
		core.LogDebug("Duplicate comment detected, aborting entry")
		return
	}
	for _, attachment := range attachments {
		fileURL := attachment.FileURL
		fileUUID := uuid.New().String()
		cid := ""
		if strings.HasPrefix(fileURL, "ipfs://") {
			cid = strings.TrimPrefix(fileURL, "ipfs://")
		}
		mimeType := attachment.MimeType
		size := attachment.FileSize
		fileName := attachment.FileName
		var existingFileUUID string
		if fileURL != "" || cid != "" {
			rows, err := db.runParamSQLSelect("SELECT fileUUID FROM files WHERE (fileURL = ? AND fileURL IS NOT NULL AND fileURL != '') OR (cid = ? AND cid IS NOT NULL AND cid != '') LIMIT 1", fileURL, cid)
			if err != nil {
				core.LogDebug("Could not check for existing file: " + err.Error())
				continue
			}
			if rows.Next() {
				err = rows.Scan(&existingFileUUID)
				if err != nil {
					core.LogDebug("Could not scan existing file UUID: " + err.Error())
					rows.Close()
					continue
				}
			}
			rows.Close()
		}
		if existingFileUUID != "" {
			fileUUID = existingFileUUID
		} else {
			insertFileQuery := "INSERT INTO files (fileUUID, fileName, mimeType, size, addedDate, cid, fileURL, source) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
			_, err = db.runParamSQLUpdate(insertFileQuery, fileUUID, fileName, mimeType, size, timestamp, cid, fileURL, "onchain")
			if err != nil {
				core.LogDebug("Could not insert file record: " + err.Error())
				continue
			}
		}
		fileTxnQuery := "INSERT IGNORE INTO file_txn_hash (fileUUID, txHash, blockchain) VALUES (?, ?, ?)"
		_, err = db.runParamSQLUpdate(fileTxnQuery, fileUUID, txHash, blockchain)
		if err != nil {
			core.LogDebug("Could not link file to transaction: " + err.Error())
		}
	}
}
func (db *MySQL) OnchainR(txHash string, blockchain string, fromAddr string, targetTxHash string, targetType string, reactionType string, timestamp uint64) {
	if reactionType == "like" {
		deleteQueryFmt := "DELETE FROM onchain_%s_reaction WHERE fromAddress = ? AND targetTxHash = ? AND blockchain = ? AND reactionType = 'dislike'"
		deleteQuery := fmt.Sprintf(deleteQueryFmt, blockchain)
		_, _ = db.runParamSQLUpdate(deleteQuery, fromAddr, targetTxHash, blockchain)
	} else if reactionType == "dislike" {
		deleteQueryFmt := "DELETE FROM onchain_%s_reaction WHERE fromAddress = ? AND targetTxHash = ? AND blockchain = ? AND reactionType = 'like'"
		deleteQuery := fmt.Sprintf(deleteQueryFmt, blockchain)
		_, _ = db.runParamSQLUpdate(deleteQuery, fromAddr, targetTxHash, blockchain)
	} else {
		deleteQueryFmt := "DELETE FROM onchain_%s_reaction WHERE fromAddress = ? AND targetTxHash = ? AND blockchain = ? AND reactionType NOT IN ('like', 'dislike')"
		deleteQuery := fmt.Sprintf(deleteQueryFmt, blockchain)
		_, _ = db.runParamSQLUpdate(deleteQuery, fromAddr, targetTxHash, blockchain)
	}
	queryFmt := "INSERT IGNORE INTO onchain_%s_reaction (txHash, blockchain, fromAddress, targetTxHash, targetType, reactionType, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, targetTxHash, targetType, reactionType, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the reaction in the database: " + err.Error())
	}
}
func (db *MySQL) OnchainP(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string) {
	queryFmt := "INSERT IGNORE INTO onchain_%s_post (txHash, blockchain, fromAddress, parentTxHash, amount, timestamp, data) VALUES (?, ?, ?, ?, ?, ?, ?)"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data)
	if err != nil {
		core.LogDebug("Could not tokenize the post in the database: " + err.Error())
	}
}
func (db *MySQL) OnchainPA(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string, attachments []Attachment) {
	queryFmt := "INSERT IGNORE INTO onchain_%s_post (txHash, blockchain, fromAddress, parentTxHash, amount, timestamp, data) VALUES (?, ?, ?, ?, ?, ?, ?)"
	query := fmt.Sprintf(queryFmt, blockchain)
	result, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data)
	if err != nil {
		core.LogDebug("Could not tokenize the post in the database: " + err.Error())
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		core.LogDebug("Could not count the post in the database: " + err.Error())
		return
	}
	if rowsAffected == 0 {
		core.LogDebug("Duplicate post detected, aborting entry")
		return
	}
	for _, attachment := range attachments {
		fileURL := attachment.FileURL
		fileUUID := uuid.New().String()
		cid := ""
		if strings.HasPrefix(fileURL, "ipfs://") {
			cid = strings.TrimPrefix(fileURL, "ipfs://")
		}
		mimeType := attachment.MimeType
		size := attachment.FileSize
		fileName := attachment.FileName
		var existingFileUUID string
		if fileURL != "" || cid != "" {
			rows, err := db.runParamSQLSelect("SELECT fileUUID FROM files WHERE (fileURL = ? AND fileURL IS NOT NULL AND fileURL != '') OR (cid = ? AND cid IS NOT NULL AND cid != '') LIMIT 1", fileURL, cid)
			if err != nil {
				core.LogDebug("Could not check for existing file: " + err.Error())
				continue
			}
			if rows.Next() {
				err = rows.Scan(&existingFileUUID)
				if err != nil {
					core.LogDebug("Could not scan existing file UUID: " + err.Error())
					rows.Close()
					continue
				}
			}
			rows.Close()
		}
		if existingFileUUID != "" {
			fileUUID = existingFileUUID
		} else {
			insertFileQuery := "INSERT INTO files (fileUUID, fileName, mimeType, size, addedDate, cid, fileURL, source) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
			_, err = db.runParamSQLUpdate(insertFileQuery, fileUUID, fileName, mimeType, size, timestamp, cid, fileURL, "onchain")
			if err != nil {
				core.LogDebug("Could not insert file record: " + err.Error())
				continue
			}
		}
		fileTxnQuery := "INSERT IGNORE INTO file_txn_hash (fileUUID, txHash, blockchain) VALUES (?, ?, ?)"
		_, err = db.runParamSQLUpdate(fileTxnQuery, fileUUID, txHash, blockchain)
		if err != nil {
			core.LogDebug("Could not link file to transaction: " + err.Error())
		}
	}
}
func (db *MySQL) OnchainMN(blockchain string, address string, name string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, name, nameTimestamp) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE name = IF(VALUES(nameTimestamp) > nameTimestamp, VALUES(name), name), nameTimestamp = IF(VALUES(nameTimestamp) > nameTimestamp, VALUES(nameTimestamp), nameTimestamp)"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, name, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *MySQL) OnchainMA(blockchain string, address string, avatar string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, avatar, avatarTimestamp) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE avatar = IF(VALUES(avatarTimestamp) > avatarTimestamp, VALUES(avatar), avatar), avatarTimestamp = IF(VALUES(avatarTimestamp) > avatarTimestamp, VALUES(avatarTimestamp), avatarTimestamp)"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, avatar, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *MySQL) OnchainMB(blockchain string, address string, banner string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, banner, bannerTimestamp) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE banner = IF(VALUES(bannerTimestamp) > bannerTimestamp, VALUES(banner), banner), bannerTimestamp = IF(VALUES(bannerTimestamp) > bannerTimestamp, VALUES(bannerTimestamp), bannerTimestamp)"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, banner, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *MySQL) OnchainMV(blockchain string, address string, vertical string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, vertical, verticalTimestamp) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE vertical = IF(VALUES(verticalTimestamp) > verticalTimestamp, VALUES(vertical), vertical), verticalTimestamp = IF(VALUES(verticalTimestamp) > verticalTimestamp, VALUES(verticalTimestamp), verticalTimestamp)"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, vertical, int64(timestamp))
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *MySQL) OnchainML(blockchain string, address string, location string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, location, locationTimestamp) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE location = IF(VALUES(locationTimestamp) > locationTimestamp, VALUES(location), location), locationTimestamp = IF(VALUES(locationTimestamp) > locationTimestamp, VALUES(locationTimestamp), locationTimestamp)"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, location, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *MySQL) OnchainMW(blockchain string, address string, website string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, website, websiteTimestamp) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE website = IF(VALUES(websiteTimestamp) > websiteTimestamp, VALUES(website), website), websiteTimestamp = IF(VALUES(websiteTimestamp) > websiteTimestamp, VALUES(websiteTimestamp), websiteTimestamp)"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, website, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *MySQL) OnchainMC(blockchain string, address string, colors string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, colors, colorsTimestamp) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE colors = IF(VALUES(colorsTimestamp) > colorsTimestamp, VALUES(colors), colors), colorsTimestamp = IF(VALUES(colorsTimestamp) > colorsTimestamp, VALUES(colorsTimestamp), colorsTimestamp)"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, colors, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta colors in the database: " + err.Error())
	}
}
func (db *MySQL) OnchainMD(blockchain string, address string, description string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, description, descriptionTimestamp) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE description = IF(VALUES(descriptionTimestamp) > descriptionTimestamp, VALUES(description), description), descriptionTimestamp = IF(VALUES(descriptionTimestamp) > descriptionTimestamp, VALUES(descriptionTimestamp), descriptionTimestamp)"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, description, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *MySQL) OnchainF(txHash string, blockchain string, followerAddress string, followerBlockchain string, followeeAddress string, followeeBlockchain string, timestamp uint64) {
	followQueryFmt := "SELECT 1 FROM onchain_%s_follow WHERE followerAddress = ? AND followerBlockchain = ? AND followeeAddress = ? AND followeeBlockchain = ? LIMIT 1"
	followQuery := fmt.Sprintf(followQueryFmt, blockchain)
	rows, err := db.runParamSQLSelect(followQuery, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain)
	if err != nil {
		core.LogDebug("Could not check if the follow already exists in the database: " + err.Error())
		return
	}
	followExists := rows.Next()
	rows.Close()
	if followExists {
		return
	}
	queryFmt := "INSERT IGNORE INTO onchain_%s_follow (txHash, blockchain, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err = db.runParamSQLUpdate(query, txHash, blockchain, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the follow in the database: " + err.Error())
	}
}
func (db *MySQL) OnchainFU(txHash string, blockchain string, followerAddress string, followerBlockchain string, followeeAddress string, followeeBlockchain string, timestamp uint64) {
	followQueryFmt := "SELECT 1 FROM onchain_%s_follow WHERE followerAddress = ? AND followerBlockchain = ? AND followeeAddress = ? AND followeeBlockchain = ? LIMIT 1"
	followQuery := fmt.Sprintf(followQueryFmt, blockchain)
	rows, err := db.runParamSQLSelect(followQuery, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain)
	if err != nil {
		core.LogDebug("Could not check if the follow exists in the database: " + err.Error())
		return
	}
	followExists := rows.Next()
	rows.Close()
	if !followExists {
		core.LogDebug("Unfollow transaction dropped: follow relationship does not exist")
		return
	}
	queryFmt := "DELETE FROM onchain_%s_follow WHERE followerAddress = ? AND followerBlockchain = ? AND followeeAddress = ? AND followeeBlockchain = ?"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err = db.runParamSQLUpdate(query, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain)
	if err != nil {
		core.LogDebug("Could not remove the follow relationship from the database: " + err.Error())
	}
}
func (db *MySQL) OnchainDeleteExpired(blockchain string, cutoffTimestamp uint64) {
	queryFmt := "DELETE FROM onchain_%s_post WHERE blockchain = ? AND timestamp < ?"
	query := fmt.Sprintf(queryFmt, blockchain)
	result, err := db.runParamSQLUpdate(query, blockchain, cutoffTimestamp)
	if err != nil {
		core.LogDebug("Could not delete expired posts: " + err.Error())
	} else if rows, _ := result.RowsAffected(); rows > 0 {
		core.LogDebug("Deleted " + fmt.Sprintf("%d", rows) + " expired posts from " + blockchain)
	}
	queryFmt = "DELETE FROM onchain_%s_follow WHERE followerBlockchain = ? AND timestamp < ?"
	query = fmt.Sprintf(queryFmt, blockchain)
	result, err = db.runParamSQLUpdate(query, blockchain, cutoffTimestamp)
	if err != nil {
		core.LogDebug("Could not delete expired follows: " + err.Error())
	} else if rows, _ := result.RowsAffected(); rows > 0 {
		core.LogDebug("Deleted " + fmt.Sprintf("%d", rows) + " expired follows from " + blockchain)
	}
	queryFmt = "DELETE FROM onchain_%s_meta WHERE blockchain = ? AND blockchainTimestamp < ? AND blockchainTimestamp > 0"
	query = fmt.Sprintf(queryFmt, blockchain)
	result, err = db.runParamSQLUpdate(query, blockchain, cutoffTimestamp)
	if err != nil {
		core.LogDebug("Could not delete expired metadata: " + err.Error())
	} else if rows, _ := result.RowsAffected(); rows > 0 {
		core.LogDebug("Deleted " + fmt.Sprintf("%d", rows) + " expired metadata entries from " + blockchain)
	}
}

// --- Comment Functions --- //
func (db *MySQL) GetComments(parentTxHash string, blockchain string, limit int, offset int) []map[string]interface{} {
	var comments []map[string]interface{}
	queryFmt := `SELECT c.txHash, c.blockchain, c.fromAddress, c.parentTxHash, c.timestamp, c.data,
		COALESCE(m.name, '') as author, COALESCE(m.avatar, '') as avatarSrc,
		(SELECT COUNT(*) FROM onchain_%s_reaction r WHERE r.targetTxHash = c.txHash AND r.blockchain = c.blockchain AND r.reactionType = 'like') as likeCount,
		(SELECT COUNT(*) FROM onchain_%s_reaction r WHERE r.targetTxHash = c.txHash AND r.blockchain = c.blockchain AND r.reactionType = 'dislike') as dislikeCount,
		(SELECT COUNT(*) FROM onchain_%s_comment c2 WHERE c2.parentTxHash = c.txHash AND c2.blockchain = c.blockchain) as replyCount
		FROM onchain_%s_comment c
		LEFT JOIN onchain_%s_meta m ON c.fromAddress = m.address AND c.blockchain = m.blockchain
		WHERE c.parentTxHash = ? AND c.blockchain = ?
		ORDER BY likeCount DESC, c.timestamp DESC
		LIMIT ? OFFSET ?`
	query := fmt.Sprintf(queryFmt, blockchain, blockchain, blockchain, blockchain, blockchain)
	rows, err := db.runParamSQLSelect(query, parentTxHash, blockchain, limit, offset)
	if err != nil {
		core.LogDebug("Could not get comments: " + err.Error())
		return comments
	}
	defer rows.Close()
	for rows.Next() {
		var txHash, bc, fromAddress, pTxHash, data, author, avatarSrc string
		var timestamp, likeCount, dislikeCount, replyCount int64
		err := rows.Scan(&txHash, &bc, &fromAddress, &pTxHash, &timestamp, &data, &author, &avatarSrc, &likeCount, &dislikeCount, &replyCount)
		if err != nil {
			core.LogDebug("Could not scan comment row: " + err.Error())
			continue
		}
		comment := map[string]interface{}{
			"txHash":       txHash,
			"blockchain":   bc,
			"address":      fromAddress,
			"parentTxHash": pTxHash,
			"timestamp":    timestamp,
			"payload":      data,
			"author":       author,
			"avatarSrc":    avatarSrc,
			"likeCount":    likeCount,
			"dislikeCount": dislikeCount,
			"replyCount":   replyCount,
		}
		attachments := db.GetPostAttachments(txHash, bc)
		if len(attachments) > 0 {
			comment["attachments"] = attachments
		}
		comments = append(comments, comment)
	}
	return comments
}
func (db *MySQL) GetCommentCount(targetTxHash string, blockchain string) int64 {
	queryFmt := `WITH RECURSIVE descendants AS (
		SELECT txHash FROM onchain_%s_comment WHERE parentTxHash = ? AND blockchain = ?
		UNION ALL
		SELECT c.txHash FROM onchain_%s_comment c
		INNER JOIN descendants d ON c.parentTxHash = d.txHash AND c.blockchain = ?
	)
	SELECT COUNT(*) FROM descendants`
	query := fmt.Sprintf(queryFmt, blockchain, blockchain)
	rows, err := db.runParamSQLSelect(query, targetTxHash, blockchain, blockchain)
	if err != nil {
		return 0
	}
	defer rows.Close()
	var count int64
	if rows.Next() {
		rows.Scan(&count)
	}
	return count
}
func (db *MySQL) HasUserCommented(parentTxHash string, blockchain string, address string) bool {
	queryFmt := "SELECT COUNT(*) FROM onchain_%s_comment WHERE parentTxHash = ? AND blockchain = ? AND fromAddress = ? LIMIT 1"
	query := fmt.Sprintf(queryFmt, blockchain)
	rows, err := db.runParamSQLSelect(query, parentTxHash, blockchain, address)
	if err != nil {
		return false
	}
	defer rows.Close()
	var count int64
	if rows.Next() {
		rows.Scan(&count)
	}
	return count > 0
}

// --- Reaction Functions --- //
func (db *MySQL) GetReactionCounts(targetTxHash string, blockchain string) map[string]interface{} {
	result := map[string]interface{}{
		"likes":    int64(0),
		"dislikes": int64(0),
		"emoji":    map[string]int64{},
	}
	queryFmt := `SELECT reactionType, COUNT(*) as count FROM (
		SELECT fromAddress, reactionType,
			ROW_NUMBER() OVER (
				PARTITION BY fromAddress,
					CASE WHEN reactionType IN ('like', 'dislike') THEN 'vote' ELSE 'emoji' END
				ORDER BY timestamp DESC
			) as rn
		FROM onchain_%s_reaction
		WHERE targetTxHash = ? AND blockchain = ?
	) t WHERE rn = 1
	GROUP BY reactionType`
	query := fmt.Sprintf(queryFmt, blockchain)
	rows, err := db.runParamSQLSelect(query, targetTxHash, blockchain)
	if err != nil {
		core.LogDebug("Could not get reaction counts: " + err.Error())
		return result
	}
	defer rows.Close()
	emojiCounts := map[string]int64{}
	for rows.Next() {
		var reactionType string
		var count int64
		if err := rows.Scan(&reactionType, &count); err != nil {
			continue
		}
		switch reactionType {
		case "like":
			result["likes"] = count
		case "dislike":
			result["dislikes"] = count
		default:
			emojiCounts[reactionType] = count
		}
	}
	result["emoji"] = emojiCounts
	return result
}
func (db *MySQL) GetUserReaction(targetTxHash string, blockchain string, fromAddress string) string {
	queryFmt := `SELECT reactionType FROM onchain_%s_reaction
		WHERE targetTxHash = ? AND blockchain = ? AND fromAddress = ?
		ORDER BY timestamp DESC LIMIT 1`
	query := fmt.Sprintf(queryFmt, blockchain)
	rows, err := db.runParamSQLSelect(query, targetTxHash, blockchain, fromAddress)
	if err != nil {
		return ""
	}
	defer rows.Close()
	var reactionType string
	if rows.Next() {
		rows.Scan(&reactionType)
	}
	return reactionType
}
func (db *MySQL) GetUserReactions(targetTxHash string, blockchain string, fromAddress string) map[string]string {
	result := map[string]string{"likeDislike": "", "emoji": ""}
	queryFmt := `SELECT reactionType FROM (
		SELECT reactionType,
			ROW_NUMBER() OVER (
				PARTITION BY CASE WHEN reactionType IN ('like', 'dislike') THEN 'vote' ELSE 'emoji' END
				ORDER BY timestamp DESC
			) as rn
		FROM onchain_%s_reaction
		WHERE targetTxHash = ? AND blockchain = ? AND fromAddress = ?
	) t WHERE rn = 1`
	query := fmt.Sprintf(queryFmt, blockchain)
	rows, err := db.runParamSQLSelect(query, targetTxHash, blockchain, fromAddress)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var reactionType string
		rows.Scan(&reactionType)
		if reactionType == "like" || reactionType == "dislike" {
			result["likeDislike"] = reactionType
		} else if reactionType != "" {
			result["emoji"] = reactionType
		}
	}
	return result
}

// --- Post Get Functions --- //
func (db *MySQL) GetPost(txHash string, blockchain string) map[string]interface{} {
	var post map[string]interface{}
	queryFmt := `SELECT p.txHash, p.blockchain, p.fromAddress, p.parentTxHash, p.timestamp, p.data,
		COALESCE(m.name, '') as author, COALESCE(m.avatar, '') as avatarSrc
		FROM onchain_%s_post p
		LEFT JOIN onchain_%s_meta m ON p.fromAddress = m.address AND p.blockchain = m.blockchain
		WHERE p.txHash = ? AND p.blockchain = ?`
	query := fmt.Sprintf(queryFmt, blockchain, blockchain)
	rows, err := db.runParamSQLSelect(query, txHash, blockchain)
	if err != nil {
		core.LogDebug("Could not get post: " + err.Error())
		return post
	}
	defer rows.Close()
	if rows.Next() {
		var pTxHash, bc, fromAddress, parentTxHash, data, author, avatarSrc string
		var timestamp int64
		err := rows.Scan(&pTxHash, &bc, &fromAddress, &parentTxHash, &timestamp, &data, &author, &avatarSrc)
		if err != nil {
			core.LogDebug("Could not scan post row: " + err.Error())
			return post
		}
		post = map[string]interface{}{
			"txHash":       pTxHash,
			"blockchain":   bc,
			"address":      fromAddress,
			"parentTxHash": parentTxHash,
			"timestamp":    timestamp,
			"payload":      data,
			"author":       author,
			"avatarSrc":    avatarSrc,
		}
		attachments := db.GetPostAttachments(txHash, blockchain)
		if len(attachments) > 0 {
			post["attachments"] = attachments
		}
	}
	return post
}
func (db *MySQL) GetPostAttachments(txHash string, blockchain string) [][]interface{} {
	var attachments [][]interface{}
	query := `SELECT f.fileURL, f.mimeType, f.size, f.fileName
		FROM files f
		JOIN file_txn_hash fth ON f.fileUUID = fth.fileUUID
		WHERE fth.txHash = ? AND fth.blockchain = ?`
	rows, err := db.runParamSQLSelect(query, txHash, blockchain)
	if err != nil {
		return attachments
	}
	defer rows.Close()
	for rows.Next() {
		var fileURL, mimeType, fileName string
		var size int64
		if err := rows.Scan(&fileURL, &mimeType, &size, &fileName); err != nil {
			continue
		}
		attachments = append(attachments, []interface{}{fileURL, mimeType, size, fileName})
	}
	return attachments
}

// --- Followers Feed --- //
func (db *MySQL) GetFollowersFeed(followerAddress string, followerBlockchain string, limit int, offset int) []map[string]interface{} {
	var posts []map[string]interface{}
	queryFmt := `SELECT p.txHash, COALESCE(p.parentTxHash, '') as parentTxHash, p.timestamp, p.data, p.fromAddress, p.blockchain
			  FROM onchain_%s_post p
			  INNER JOIN onchain_%s_follow f ON p.fromAddress = f.followeeAddress AND p.blockchain = f.followeeBlockchain
			  WHERE f.followerAddress = LOWER(?) AND f.followerBlockchain = ? AND p.data IS NOT NULL
			  ORDER BY p.timestamp DESC
			  LIMIT ? OFFSET ?`
	query := fmt.Sprintf(queryFmt, followerBlockchain, followerBlockchain)
	rows, err := db.runParamSQLSelect(query, followerAddress, followerBlockchain, limit, offset)
	if err != nil {
		core.LogDebug("Could not get followers feed from database: " + err.Error())
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var timestamp uint64
		var txHash, parentTxHash, payload, blockchain, address string
		var attachments [][]interface{}
		err := rows.Scan(&txHash, &parentTxHash, &timestamp, &payload, &address, &blockchain)
		if err != nil {
			core.LogDebug("Could not scan database rows for followers feed: " + err.Error())
			return nil
		}
		sqlQuery := "SELECT f.mimeType, f.size, f.fileUrl, f.fileName FROM files f INNER JOIN file_txn_hash fth ON f.fileUUID = fth.fileUUID WHERE fth.txHash = ? AND fth.blockchain = ?"
		rowsAttachments, err := db.runParamSQLSelect(sqlQuery, txHash, blockchain)
		if err != nil {
			core.LogDebug("Could not get attachments for post: " + err.Error())
		} else if rowsAttachments != nil {
			for rowsAttachments.Next() {
				var mimeType string
				var size uint64
				var fileUrl string
				var fileName string
				err := rowsAttachments.Scan(&mimeType, &size, &fileUrl, &fileName)
				if err != nil {
					core.LogDebug("Could parse rows for post attachment: " + err.Error())
					break
				}
				attachment := []interface{}{fileUrl, mimeType, size, fileName}
				attachments = append(attachments, attachment)
			}
			rowsAttachments.Close()
		}
		commentCount := db.GetCommentCount(txHash, blockchain)
		post := map[string]interface{}{
			"resultType":   "post",
			"txHash":       txHash,
			"parentHash":   parentTxHash,
			"timestamp":    timestamp,
			"payload":      payload,
			"blockchain":   blockchain,
			"address":      address,
			"commentCount": commentCount,
		}
		if attachments != nil {
			post["attachments"] = attachments
		}
		posts = append(posts, post)
	}
	return posts
}

// --- Notifications --- //
func (db *MySQL) NotificationInsert(uid string, message string) {
	_, err := db.runParamSQLUpdate("INSERT INTO notifications (uid, message, timestamp) VALUES (?, ?, ?)", uid, message, core.GetTimestamp())
	if err != nil {
		core.LogDebug("Could not insert notification: " + err.Error())
	}
}
func (db *MySQL) NotificationDismiss(uid string) {
	_, err := db.runParamSQLUpdate("DELETE FROM notifications WHERE uid = ?", uid)
	if err != nil {
		core.LogDebug("Could not dismiss notification: " + err.Error())
	}
}
func (db *MySQL) NotificationGetActive() []map[string]string {
	rows, err := db.runParamSQLSelect("SELECT uid, message FROM notifications")
	if err != nil {
		core.LogDebug("Could not get active notifications: " + err.Error())
		return nil
	}
	defer rows.Close()
	var notifications []map[string]string
	for rows.Next() {
		var uid, message string
		if err := rows.Scan(&uid, &message); err != nil {
			core.LogDebug("Could not scan notification row: " + err.Error())
			continue
		}
		notifications = append(notifications, map[string]string{"uid": uid, "message": message})
	}
	return notifications
}

// --- oEmbed Cache Functions --- //
func (db *MySQL) OEmbedCacheGet(url string) (string, int64) {
	rows, err := db.runParamSQLSelect("SELECT data, fetchedAt FROM oembed_cache WHERE url = ?", url)
	if err != nil {
		core.LogDebug("Could not get oEmbed cache: " + err.Error())
		return "", 0
	}
	defer rows.Close()
	if rows.Next() {
		var data string
		var fetchedAt int64
		if err := rows.Scan(&data, &fetchedAt); err != nil {
			core.LogDebug("Could not scan oEmbed cache row: " + err.Error())
			return "", 0
		}
		return data, fetchedAt
	}
	return "", 0
}
func (db *MySQL) OEmbedCacheSet(url string, data string) {
	_, err := db.runParamSQLUpdate("INSERT INTO oembed_cache (url, data, fetchedAt) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE data = VALUES(data), fetchedAt = VALUES(fetchedAt)", url, data, core.GetTimestamp())
	if err != nil {
		core.LogDebug("Could not set oEmbed cache: " + err.Error())
	}
}

// --- Snapshot Functions --- //
func (db *MySQL) exportSnapshots(exportDir string, blockchain string, headBlock uint64, tailBlock uint64) error {
	return core.LogDebugReturn("MySQL snapshot export not implemented - use mysqldump instead")
}

// --- Wallet Functions --- //
func (db *MySQL) WalletStore(publicKey string, blockchain string, address string, encryptedPrivateKey string, isDefault bool) error {
	isDefaultInt := 0
	if isDefault {
		isDefaultInt = 1
		_, err := db.runParamSQLUpdate("UPDATE wallets SET isDefault = 0 WHERE blockchain = ?", blockchain)
		if err != nil {
			return core.LogDebugReturn("Could not unset existing default wallet: " + err.Error())
		}
	}
	query := "INSERT INTO wallets (publicKey, blockchain, address, encryptedPrivateKey, isDefault) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE address = VALUES(address), encryptedPrivateKey = VALUES(encryptedPrivateKey), isDefault = VALUES(isDefault)"
	_, err := db.runParamSQLUpdate(query, publicKey, blockchain, address, encryptedPrivateKey, isDefaultInt)
	if err != nil {
		return core.LogDebugReturn("Could not store wallet: " + err.Error())
	}
	return nil
}
func (db *MySQL) WalletGet(publicKey string, blockchain string) (map[string]interface{}, error) {
	rows, err := db.runParamSQLSelect("SELECT publicKey, blockchain, address, isDefault FROM wallets WHERE publicKey = ? AND blockchain = ?", publicKey, blockchain)
	if err != nil {
		return nil, core.LogDebugReturn("Could not get wallet: " + err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var pubKey, bc, addr string
		var isDefaultInt int
		err = rows.Scan(&pubKey, &bc, &addr, &isDefaultInt)
		if err != nil {
			return nil, core.LogDebugReturn("Could not scan wallet row: " + err.Error())
		}
		wallet := map[string]interface{}{
			"publicKey":  pubKey,
			"blockchain": bc,
			"address":    addr,
			"isDefault":  isDefaultInt == 1,
		}
		return wallet, nil
	}
	return nil, core.LogDebugReturn("Wallet not found")
}
func (db *MySQL) WalletGetDefault(blockchain string) (map[string]interface{}, error) {
	rows, err := db.runParamSQLSelect("SELECT publicKey, blockchain, address, isDefault FROM wallets WHERE blockchain = ? AND isDefault = 1", blockchain)
	if err != nil {
		return nil, core.LogDebugReturn("Could not get default wallet: " + err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var pubKey, bc, addr string
		var isDefaultInt int
		err = rows.Scan(&pubKey, &bc, &addr, &isDefaultInt)
		if err != nil {
			return nil, core.LogDebugReturn("Could not scan default wallet row: " + err.Error())
		}
		wallet := map[string]interface{}{
			"publicKey":  pubKey,
			"blockchain": bc,
			"address":    addr,
			"isDefault":  true,
		}
		return wallet, nil
	}
	return nil, core.LogDebugReturn("No default wallet found for blockchain: " + blockchain)
}
func (db *MySQL) WalletGetPrivateKey(publicKey string, blockchain string) (string, error) {
	rows, err := db.runParamSQLSelect("SELECT encryptedPrivateKey FROM wallets WHERE publicKey = ? AND blockchain = ?", publicKey, blockchain)
	if err != nil {
		return "", core.LogDebugReturn("Could not get wallet private key: " + err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var encPrivKey []byte
		err = rows.Scan(&encPrivKey)
		if err != nil {
			return "", core.LogDebugReturn("Could not scan private key row: " + err.Error())
		}
		return string(encPrivKey), nil
	}
	return "", core.LogDebugReturn("Wallet not found")
}
func (db *MySQL) WalletSetDefault(publicKey string, blockchain string) error {
	_, err := db.runParamSQLUpdate("UPDATE wallets SET isDefault = 0 WHERE blockchain = ?", blockchain)
	if err != nil {
		return core.LogDebugReturn("Could not unset existing default wallet: " + err.Error())
	}
	result, err := db.runParamSQLUpdate("UPDATE wallets SET isDefault = 1 WHERE publicKey = ? AND blockchain = ?", publicKey, blockchain)
	if err != nil {
		return core.LogDebugReturn("Could not set default wallet: " + err.Error())
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return core.LogDebugReturn("Could not get rows affected: " + err.Error())
	}
	if rowsAffected == 0 {
		return core.LogDebugReturn("Wallet not found")
	}
	return nil
}
func (db *MySQL) WalletGetAll() ([]map[string]interface{}, error) {
	var wallets []map[string]interface{}
	rows, err := db.runParamSQLSelect("SELECT publicKey, blockchain, address, isDefault FROM wallets ORDER BY blockchain, isDefault DESC")
	if err != nil {
		return nil, core.LogDebugReturn("Could not get all wallets: " + err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var pubKey, bc, addr string
		var isDefaultInt int
		err = rows.Scan(&pubKey, &bc, &addr, &isDefaultInt)
		if err != nil {
			return nil, core.LogDebugReturn("Could not scan wallet row: " + err.Error())
		}
		wallet := map[string]interface{}{
			"publicKey":  pubKey,
			"blockchain": bc,
			"address":    addr,
			"isDefault":  isDefaultInt == 1,
		}
		wallets = append(wallets, wallet)
	}
	return wallets, nil
}
