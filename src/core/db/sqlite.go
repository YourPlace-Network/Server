package db

import (
	"YourPlace/src/core"
	"context"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/glebarez/go-sqlite"
	"os"
	"regexp"
	"strings"
	"time"
)

type SQLite struct {
	database *sql.DB
	path     string
}

func (db *SQLite) Init(path string) {
	startupCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	dbPath := path + "yourplace.sqlite.db"
	core.LogDebug("Using SQLite database at: " + dbPath)
	db.path = dbPath
	database, err := sql.Open("sqlite", dbPath)
	if err != nil || database == nil {
		core.LogFatal("Could not open sqlite db: " + err.Error())
		return
	}
	database.SetMaxOpenConns(50)
	database.SetMaxIdleConns(20)
	database.SetConnMaxLifetime(15 * time.Minute)
	database.SetConnMaxIdleTime(3 * time.Minute)
	db.database = database
	// Create Tables
	err = db.createTables(startupCtx)
	if err != nil {
		core.LogError("Could not create tables: " + err.Error())
	}
}

// --- SQL Functions --- //
func (db *SQLite) runSQL(query string) {
	_, err := db.database.Exec(query)
	if err != nil {
		core.LogError("Could not run SQLite query: " + query + " - " + err.Error())
	}
}
func (db *SQLite) runParamSQLSelect(query string, params ...interface{}) (*sql.Rows, error) {
	queryUpper := strings.ToUpper(strings.TrimSpace(query))
	timeout := 30 * time.Second
	if strings.Contains(query, "settings") || strings.Contains(query, "meta") {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// if query starts with SELECT or EXPLAIN use Query()
	if strings.HasPrefix(queryUpper, "SELECT") || strings.HasPrefix(queryUpper, "EXPLAIN") {
		statement, err := db.database.PrepareContext(ctx, query)
		if err != nil {
			if ctx.Err() == context.Canceled {
				return nil, core.LogErrorReturn("Query preparation canceled: " + err.Error())
			}
			return nil, core.LogErrorReturn("Could not prepare SQLite query: " + query + " - " + err.Error())
		}
		defer statement.Close()
		queryCtx := context.Background()
		rows, err := statement.QueryContext(queryCtx, params...)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil, core.LogErrorReturn("Query canceled: " + err.Error())
			}
			return nil, core.LogErrorReturn("Could not run SQLite query: " + err.Error())
		}
		return rows, nil
	}
	return nil, core.LogErrorReturn("Invalid sql method")
}
func (db *SQLite) runParamSQLUpdate(query string, params ...interface{}) (sql.Result, error) {
	// For INSERT, UPDATE, DELETE, etc. use Exec()
	queryUpper := strings.ToUpper(strings.TrimSpace(query))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if strings.HasPrefix(queryUpper, "SELECT") || strings.HasPrefix(queryUpper, "EXPLAIN") {
		return nil, core.LogErrorReturn("Invalid method for SQL update")
	}
	statement, err := db.database.PrepareContext(ctx, query)
	if err != nil {
		return nil, core.LogErrorReturn("Could not prepare SQLite query: " + query + " - " + err.Error())
	}
	defer statement.Close()
	result, err := statement.ExecContext(ctx, params...)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, core.LogErrorReturn("Query timed out after 15s: " + err.Error())
		}
		return nil, core.LogErrorReturn("Could not run SQLite query: " + query + " - " + err.Error())
	}
	return result, nil
}
func (db *SQLite) getRows(query string) (*sql.Rows, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stmt, err := db.database.PrepareContext(ctx, query)
	if err != nil {
		return nil, core.LogErrorReturn("prepare failed: " + err.Error())
	}
	defer stmt.Close()
	return stmt.QueryContext(ctx)
}
func (db *SQLite) flushTable(name string) {
	query := "DELETE * FROM " + sanitizeSQLiteTableName(name)
	db.runSQL(query)
}
func (db *SQLite) deleteTable(name string) {
	query := "DROP TABLE IF EXISTS " + sanitizeSQLiteTableName(name)
	db.runSQL(query)
}

// --- Helper Functions --- //
func sanitizeSQLiteTableName(payload string) string {
	// Remove any character that isn't alphanumeric or an underscore
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	sanitized := reg.ReplaceAllString(payload, "")
	// Ensure the sanitized string starts with a letter
	if len(sanitized) > 0 && !strings.HasPrefix(sanitized, "_") {
		firstChar := sanitized[0]
		if !((firstChar >= 'a' && firstChar <= 'z') || (firstChar >= 'A' && firstChar <= 'Z')) {
			sanitized = "t_" + sanitized
		}
	} else {
		sanitized = "t_" + sanitized
	}
	// Truncate if too long (SQLite has a limit of 1024 bytes for identifiers
	if len(sanitized) > 1025 {
		sanitized = sanitized[:1024]
	}
	return sanitized
}
func (db *SQLite) execWithRetry(ctx context.Context, query string, maxRetries int) error {
	if db.database == nil {
		return core.LogErrorReturn("Database connection not initialized")
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
func (db *SQLite) withTransaction(fn func(*sql.Tx) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := db.database.BeginTx(ctx, nil)
	if err != nil {
		return core.LogErrorReturn("Begin transaction failed: " + err.Error())
	}
	if err = fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return core.LogErrorReturn(fmt.Sprintf("Rollback failed: %v (original error: %w)", rbErr, err))
		}
		return err
	}
	if err = tx.Commit(); err != nil {
		return core.LogErrorReturn("Commit failed: " + err.Error())
	}
	return nil
}
func (db *SQLite) createTables(ctx context.Context) error {
	// First create a temporary table to store the current table schemas
	schemaMigration := `CREATE TABLE IF NOT EXISTS _schema_migrations (table_name TEXT PRIMARY KEY, schema_hash TEXT);`
	err := db.execWithRetry(ctx, schemaMigration, 3)
	if err != nil {
		core.LogError("Schema migration table creation failed: " + err.Error())
	}
	// Tables schema map
	tables := map[string]string{
		"meta":               "CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT)",
		"posts":              "CREATE TABLE IF NOT EXISTS posts (txHash TEXT, blockchain TEXT, fromAddr TEXT, toAddr TEXT, parentTxHash TEXT, amount REAL, timestamp INTEGER, data TEXT, blockNumber INTEGER, PRIMARY KEY(txHash, blockchain))",
		"profiles":           "CREATE TABLE IF NOT EXISTS profiles (address TEXT, blockchain TEXT, name TEXT, avatar TEXT, banner TEXT, description TEXT, location TEXT, website TEXT, joined INTEGER, birthdate INTEGER, PRIMARY KEY(address, blockchain))",
		"settings":           "CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT)",
		"files":              "CREATE TABLE IF NOT EXISTS files (fileUUID TEXT PRIMARY KEY, extension TEXT, path TEXT, unsafeNameB64 TEXT, size INTEGER, addedDate INTEGER)",
		"ipfsFiles":          "CREATE TABLE IF NOT EXISTS ipfsFiles (fileUUID TEXT PRIMARY KEY, cid TEXT, addedDate INTEGER)",
		"postsBackfill":      "CREATE TABLE IF NOT EXISTS postsBackfill (uuid TEXT PRIMARY KEY, blockchain TEXT, headBlock INTEGER, status TEXT, tailBlock INTEGER, timestamp INTEGER)",
		"authNonce":          "CREATE TABLE IF NOT EXISTS authNonce (nonce TEXT PRIMARY KEY, status TEXT, timestamp INTEGER)",
		"authExpired":        "CREATE TABLE IF NOT EXISTS authExpired (uuid TEXT PRIMARY KEY, status TEXT)",
		"loginNonce":         "CREATE TABLE IF NOT EXISTS loginNonce (nonce TEXT PRIMARY KEY, domain TEXT, expiration INTEGER, nonceHash TEXT)",
		"onchain_post":       "CREATE TABLE IF NOT EXISTS onchain_post (txHash TEXT, blockchain TEXT, fromAddr TEXT DEFAULT '', toAddr TEXT DEFAULT '', parentTxHash TEXT DEFAULT '', amount REAL DEFAULT 0, timestamp INTEGER DEFAULT 0, data TEXT DEFAULT '', blockNumber INTEGER DEFAULT 0, PRIMARY KEY(txHash, blockchain))",
		"onchain_attachment": "CREATE TABLE IF NOT EXISTS onchain_attachment (txHash TEXT, blockchain TEXT, address TEXT DEFAULT '', name TEXT DEFAULT '', contentType TEXT DEFAULT '', size INTEGER DEFAULT 0, timestamp INTEGER DEFAULT 0, PRIMARY KEY(txHash, blockchain))",
		"onchain_meta": "CREATE TABLE IF NOT EXISTS onchain_meta (blockchain TEXT, address TEXT, name TEXT DEFAULT '', avatar TEXT DEFAULT '', description TEXT DEFAULT '', location TEXT DEFAULT '', banner TEXT DEFAULT '', website TEXT DEFAULT '', birthdate INTEGER DEFAULT NULL, server TEXT DEFAULT '', " +
			"blockchainTimestamp INTEGER DEFAULT 0, addressTimestamp INTEGER DEFAULT 0, nameTimestamp INTEGER DEFAULT 0, avatarTimestamp INTEGER DEFAULT 0, descriptionTimestamp INTEGER DEFAULT 0, locationTimestamp INTEGER DEFAULT 0, bannerTimestamp INTEGER DEFAULT 0, websiteTimestamp INTEGER DEFAULT 0, birthdateTimestamp INTEGER DEFAULT 0, serverTimestamp INTEGER DEFAULT 0, PRIMARY KEY(blockchain, address))",
		"onchain_block":  "CREATE TABLE IF NOT EXISTS onchain_block (txHash TEXT, blockchain TEXT, address TEXT, key TEXT, value TEXT, timestamp INTEGER DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"onchain_follow": "CREATE TABLE IF NOT EXISTS onchain_follow (txHash TEXT, blockchain TEXT, toAddr TEXT, fromAddr TEXT, timestamp INTEGER DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
	}
	for _, createStatement := range tables {
		err = db.execWithRetry(ctx, createStatement, 3)
		if err != nil {
			return core.LogErrorReturn("Table creation failed: " + err.Error())
		}
	}
	return nil
}
func (db *SQLite) runExternalSQLFile(path string) {
	core.LogWarn("Running external SQL file: " + path)
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		core.LogError("Could not read external SQL file: " + err.Error())
		return
	}
	_, err = db.database.Prepare(string(sqlBytes)) // try to validate by parsing
	if err != nil {
		core.LogError("Could not validate external SQL file: " + err.Error())
		return
	}
	db.runSQL(string(sqlBytes))
}
func (db *SQLite) exportDatabase(exportPath string) error {
	if db.database == nil {
		return core.LogErrorReturn("Database connection not initialized")
	}
	core.LogDebug("Exporting SQLite database to: " + exportPath)
	// Retry parameters
	maxRetries := 5
	var lastErr error
	// Generous timeout for the backup operation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s, 8s, 16s
			backoffDuration := time.Duration(1<<uint(attempt)) * time.Second
			select {
			case <-time.After(backoffDuration):
				// Continue with retry
			case <-ctx.Done():
				return core.LogErrorReturn("Export operation timed out: " + ctx.Err().Error())
			}
		}
		// Create a context for this specific attempt
		attemptCtx, attemptCancel := context.WithTimeout(ctx, 2*time.Minute)
		// Use VACUUM INTO for a consistent backup
		_, _err := db.database.ExecContext(attemptCtx, fmt.Sprintf("VACUUM INTO '%s'", exportPath))
		// Always cancel the attempt context
		attemptCancel()
		if _err == nil {
			core.LogDebug("Exported SQLite database to: " + exportPath)
			return nil
		}
		lastErr = _err
		// If context is canceled, stop retrying
		if ctx.Err() != nil {
			return core.LogErrorReturn("Export operation canceled: " + ctx.Err().Error())
		}
	}
	return core.LogWarningReturn("Export operation failed: " + lastErr.Error())
}
func (db *SQLite) Ping() bool {
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
func (db *SQLite) Close() error {
	return db.database.Close()
}

// --- Metadata & Settings Functions --- //
func (db *SQLite) MetaUpdateValue(key string, value string) {
	query := "INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value"
	_, err := db.runParamSQLUpdate(query, key, value)
	if err != nil {
		core.LogError("Meta update failed: " + err.Error())
	}
}
func (db *SQLite) MetaGetValue(key string) string {
	rows, err := db.runParamSQLSelect("SELECT value FROM meta WHERE key = ?", key)
	if err != nil {
		core.LogError("Could not get meta value, query failed: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			return ""
		}
		return value
	}
	return ""
}
func (db *SQLite) SettingsGetValue(key string) string {
	startupCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		select {
		case <-startupCtx.Done():
			return ""
		default:
			rows, err := db.runParamSQLSelect("SELECT value FROM settings WHERE key = ?", key)
			if err != nil {
				if strings.Contains(err.Error(), "context canceled") {
					backoff := time.Duration(1<<uint(i)) * time.Second
					core.LogWarn("Settings query failed, retrying...")
					time.Sleep(backoff)
					continue
				}
				core.LogError("Could not get setting value for key: " + key + " - query failed: " + err.Error())
				return ""
			}
			defer rows.Close()
			for rows.Next() {
				var value string
				err = rows.Scan(&value)
				if err != nil {
					return ""
				}
				return value
			}
		}
	}
	return ""
}
func (db *SQLite) SettingsUpdateValue(key string, value string) {
	query := "INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value"
	_, err := db.runParamSQLUpdate(query, key, value)
	if err != nil {
		core.LogError("Settings update failed: " + err.Error())
	}
}

// --- Profile Functions --- //
func (db *SQLite) ProfileGetName(address string, blockchain string) string {
	rows, err := db.runParamSQLSelect("SELECT name FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
	if err != nil {
		core.LogError("Could not get profile name from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			core.LogError("Could not get profile name from database: " + err.Error())
		}
		return name
	}
	return ""
}
func (db *SQLite) ProfileGetAvatar(address string, blockchain string) string {
	rows, err := db.runParamSQLSelect("SELECT avatar FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
	if err != nil {
		core.LogError("could not get profile avatar from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var avatar string
		err = rows.Scan(&avatar)
		if err != nil {
			core.LogError("could not parse database rows for profile avatar: " + err.Error())
			return ""
		}
		return avatar
	}
	return ""
}
func (db *SQLite) ProfileGetBanner(address string, blockchain string) string {
	rows, err := db.runParamSQLSelect("SELECT banner FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
	if err != nil {
		core.LogError("Could not get profile banner from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var banner string
		err = rows.Scan(&banner)
		if err != nil {
			core.LogError("Could not parse database rows for profile banner")
			return ""
		}
		return banner
	}
	return ""
}
func (db *SQLite) ProfileGetDescription(address string, blockchain string) string {
	rows, err := db.runParamSQLSelect("SELECT description FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
	if err != nil {
		core.LogError("Could not get profile description from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var description string
		err = rows.Scan(&description)
		if err != nil {
			core.LogError("Could not parse database rows for profile description: " + err.Error())
			return ""
		}
		return description
	}
	return ""
}
func (db *SQLite) ProfileGetLocation(address string, blockchain string) string {
	rows, err := db.runParamSQLSelect("SELECT location FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
	if err != nil {
		core.LogError("Could not get profile location from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var location string
		err = rows.Scan(&location)
		if err != nil {
			core.LogError("Could not parse database rows for profile location: " + err.Error())
			return ""
		}
		return location
	}
	return ""
}
func (db *SQLite) ProfileGetWebsite(address string, blockchain string) string {
	rows, err := db.runParamSQLSelect("SELECT website FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
	if err != nil {
		core.LogError("Could not get profile website from database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var website string
		err = rows.Scan(&website)
		if err != nil {
			core.LogError("Could not parse database rows for profile website: " + err.Error())
			return ""
		}
		return website
	}
	return ""
}
func (db *SQLite) ProfileGetBirthDate(address string, blockchain string) *int64 {
	rows, err := db.runParamSQLSelect("SELECT birthdate FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
	if err != nil {
		core.LogError("Could not get profile birthdate from database: " + err.Error())
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var birthDateRaw sql.NullInt64
		err = rows.Scan(&birthDateRaw)
		if !birthDateRaw.Valid {
			return nil
		}
		birthDate := birthDateRaw.Int64
		if err != nil {
			core.LogError("Could not parse database rows for profile birthdate: " + err.Error())
			return nil
		}
		return &birthDate
	}
	return nil
}
func (db *SQLite) ProfileGetJoinedDate(address string, blockchain string) *int64 {
	var metaAge int64 = 0
	var postAge int64 = 0
	var joinedDate int64 = 0
	rowsmeta, err := db.runParamSQLSelect(`SELECT COALESCE( MIN(
		CASE
	WHEN blockchainTimestamp > 0 THEN blockchainTimestamp WHEN addressTimestamp > 0 THEN addressTimestamp WHEN nameTimestamp > 0 THEN nameTimestamp WHEN avatarTimestamp > 0 THEN avatarTimestamp WHEN descriptionTimestamp > 0 THEN descriptionTimestamp
	WHEN locationTimestamp > 0 THEN locationTimestamp WHEN bannerTimestamp > 0 THEN bannerTimestamp WHEN websiteTimestamp > 0 THEN websiteTimestamp WHEN birthdateTimestamp > 0 THEN birthdateTimestamp WHEN serverTimestamp > 0 THEN serverTimestamp
	ELSE 0 END), 0)
	AS min_timestamp FROM onchain_meta WHERE blockchain = ? AND address = LOWER(?)`, blockchain, address)
	if err == nil {
		if rowsmeta != nil {
			defer rowsmeta.Close()
			for rowsmeta.Next() {
				err = rowsmeta.Scan(&metaAge)
				if err != nil {
					core.LogError("Could not parse database rows for profile joined date: " + err.Error())
					return nil

				}
			}
		}
	} else {
		core.LogError("Could not parse database rows for profile joined date: " + err.Error())
		return nil
	}
	rowsposts, err := db.runParamSQLSelect("SELECT timestamp FROM onchain_post WHERE fromAddr = LOWER(?) AND blockchain = ?", address, blockchain)
	if err == nil {
		if rowsposts != nil {
			defer rowsposts.Close()
			for rowsposts.Next() {
				var newAge int64
				err = rowsposts.Scan(&newAge)
				if err != nil {
					core.LogError("Could not get profile joined date from database: " + err.Error())
					return nil
				}
				if newAge < postAge || postAge == 0 {
					postAge = newAge
				}
			}
		}
	} else {
		core.LogError("Could not get profile joined date from database: " + err.Error())
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
func (db *SQLite) ProfileGetPosts(address string, blockchain string) []map[string]interface{} {
	var posts []map[string]interface{}
	rows, err := db.runParamSQLSelect("SELECT txHash, COALESCE(parentTxHash, '') as parentTxHash, timestamp, data FROM onchain_post WHERE fromAddr = LOWER (?) AND blockchain = ? ORDER BY timestamp DESC", address, blockchain)
	if err != nil {
		core.LogError("could not get user posts from database: " + err.Error())
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var timestamp uint64
		var txHash, data, parent string
		_err := rows.Scan(&txHash, &parent, &timestamp, &data)
		var payload, err = parsePostText(data)
		if err != nil {
			core.LogError(err.Error())
		}
		post := map[string]interface{}{
			"resultType": "profile post",
			"txHash":     txHash,
			"parent":     parent,
			"timestamp":  timestamp,
			"payload":    payload,
			"blockchain": blockchain,
			"address":    address,
		}
		if _err != nil {
			core.LogError("could not parse posts from database rows: " + _err.Error())
			return nil
		}
		posts = append(posts, post)
	}
	return posts
}

// --- Search Functions --- //
type Result struct {
	ResultType string `json:"type"`
	Blockchain string `json:"blockchain"`
	Address    string `json:"address"`
	TxHash     string `json:"txHash"`
	Timestamp  uint64 `json:"timestamp"`
	Payload    string `json:"payload"`
	ParentHash string `json:"parentHash"`
}

func (db *SQLite) SearchGetPosts(query string) []Result {
	var posts []Result
	search := "%" + query + "%"
	rows, err := db.runParamSQLSelect("SELECT txHash, COALESCE(parentTxHash, '') as parentHash, timestamp, data, fromAddr, blockchain FROM onchain_post WHERE LOWER (data) LIKE LOWER (?)", search)
	if err != nil {
		core.LogError("Could not get searched posts from database: " + err.Error())
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var timestamp uint64
		var txHash, parentHash, data, blockchain, address string
		err := rows.Scan(&txHash, &parentHash, &timestamp, &data, &address, &blockchain)
		if err != nil {
			core.LogError("Could not scan database rows: " + err.Error())
			return nil
		}
		payload, err := parsePostText(data)
		post := Result{
			ResultType: "post",
			Blockchain: blockchain,
			Address:    address,
			TxHash:     txHash,
			Timestamp:  timestamp,
			Payload:    payload,
			ParentHash: parentHash,
		}
		if err != nil {
			core.LogError("Could not parse posts from database rows: " + err.Error())
			return nil
		}
		posts = append(posts, post)
	}
	return posts
}
func (db *SQLite) SearchGetProfiles(query string) []Result {
	var profiles []Result
	search := "%" + query + "%"
	rows, err := db.runParamSQLSelect("SELECT address, blockchain FROM onchain_meta WHERE address LIKE ? OR name LIKE ?", search, search)
	if err != nil {
		core.LogError("Could not get searched profiles from database: " + err.Error())
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var address, blockchain string
		err = rows.Scan(&address, &blockchain)
		profile := Result{
			ResultType: "profile",
			Address:    address,
			Blockchain: blockchain,
		}
		if err != nil {
			core.LogError("Could not parse posts from database rows")
			return nil
		}
		profiles = append(profiles, profile)
	}
	return profiles
}

// --- Auth Functions --- //
func (db *SQLite) AuthGetNonceStatus(nonce string) string {
	rows, err := db.runParamSQLSelect("SELECT status FROM authNonce WHERE nonce = ?", nonce)
	if err != nil {
		core.LogError("Could not get nonce status from the database: " + err.Error())
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
func (db *SQLite) AuthUpdateNonce(nonce string, status string) {
	_, err := db.runParamSQLUpdate("INSERT INTO authNonce (nonce, status) VALUES (?, ?) ON CONFLICT (nonce) DO UPDATE SET status = excluded.status", nonce, status)
	if err != nil {
		core.LogError("Could not update auth nonce in database: " + err.Error())
	}
}
func (db *SQLite) AuthDeleteNonce(nonce string) {
	_, err := db.runParamSQLUpdate("DELETE FROM authNonce WHERE nonce = ?", nonce)
	if err != nil {
		core.LogError("Could not delete the auth nonce from the database: " + err.Error())
	}
}
func (db *SQLite) AuthExpireCookie(uuid string) {
	_, err := db.runParamSQLUpdate("INSERT INTO authExpired (uuid, status) VALUES (?, 'expired') ON CONFLICT (uuid) DO UPDATE SET status = 'expired'", uuid)
	if err != nil {
		core.LogError("Could not expire the auth cookie from the database: " + err.Error())
	}
}
func (db *SQLite) AuthGetCookieStatus(uuid string) string {
	rows, err := db.runParamSQLSelect("SELECT status FROM authExpired WHERE uuid = ?", uuid)
	if err != nil {
		core.LogError("Could not get the auth cookie status from the database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			core.LogError("Could not get the rows from the auth cookie status from the database: " + err.Error())
			return ""
		}
		return value
	}
	return ""
}
func (db *SQLite) AuthUpdateLoginNonce(nonce string, domain string, expiration uint64, nonceHash string) {
	query := "INSERT INTO loginNonce (nonce, domain, expiration, nonceHash) VALUES (?, ?, ?, ?) ON CONFLICT (nonce) DO NOTHING"
	_, err := db.runParamSQLUpdate(query, nonce, domain, expiration, nonceHash)
	if err != nil {
		core.LogError("Could not update loginNonce: " + err.Error())
	}
}
func (db *SQLite) AuthDeleteLoginNonce(nonce string) {
	_, err := db.runParamSQLUpdate("DELETE FROM loginNonce WHERE nonce = ?", nonce)
	if err != nil {
		core.LogError("Could not delete the login nonce from the database: " + err.Error())
	}
}
func (db *SQLite) AuthExpireLoginNonce() {
	_, err := db.runParamSQLUpdate("DELETE FROM loginNonce WHERE expiration < ?", core.GetTimestamp())
	if err != nil {
		core.LogError("Could not delete any expired login nonces from the database: " + err.Error())
	}
}
func (db *SQLite) AuthGetServerOwnerAddress() string {
	rows, err := db.runParamSQLSelect("SELECT value FROM meta WHERE key = 'accountAddress' LIMIT 1")
	if err != nil {
		core.LogError("Could not get the server owner address from the database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			core.LogError("Could not get the rows from the server owner address from the database: " + err.Error())
			return ""
		}
		return value
	}
	core.LogError("Could not get the server owner address from the database - no entry found")
	return ""
}

// --- File & IPFS Functions --- //
func (db *SQLite) FileAdd(fileUUID string, extension string, path string, unsafeNameB64 string, size int64) {
	query := "INSERT INTO files (fileUUID, extension, path, unsafeNameB64, size, addedDate) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING"
	_, err := db.runParamSQLUpdate(query, fileUUID, extension, path, unsafeNameB64, size, core.GetTimestamp())
	if err != nil {
		core.LogError("Could not add the file to the database: " + err.Error())
	}
}
func (db *SQLite) IPFSAdd(fileUUID string, cid string) {
	query := "INSERT INTO ipfsFiles (fileUUID, cid, addedDate) VALUES (?, ?, ?) ON CONFLICT DO NOTHING"
	_, err := db.runParamSQLUpdate(query, fileUUID, cid, core.GetTimestamp())
	if err != nil {
		core.LogError("Could not add the IPFS CID to the database: " + err.Error())
	}
}

// --- Indexer Functions --- //
func (db *SQLite) IndexerCreateJob(uuid string, blockchain string) {
	core.LogDebug("IndexerCreateJob(): " + uuid + " - " + blockchain)
	timestamp := core.GetTimestamp()
	query := "INSERT INTO postsBackfill (uuid, blockchain, headBlock, status, tailBlock, timestamp) VALUES (?, ?, 0, 'pending', 0, ?) ON CONFLICT (uuid) DO UPDATE SET status = excluded.status, tailBlock = excluded.tailBlock, timestamp = excluded.timestamp"
	core.LogDebug("IndexerCreateJob(): " + query)
	_, err := db.runParamSQLUpdate(query, uuid, blockchain, timestamp)
	if err != nil {
		core.LogError("Could not create indexer job in the database: " + err.Error())
	}
}
func (db *SQLite) IndexerGetJobUUID(blockchain string) string {
	rows, err := db.runParamSQLSelect("SELECT uuid FROM postsBackfill WHERE blockchain = ?", blockchain)
	if err != nil {
		core.LogError("Could not get the indexer job UUID from the database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			core.LogError("Could not get the rows for the indexer job UUID from the database: " + err.Error())
			return ""
		}
		return value
	}
	return ""
}
func (db *SQLite) IndexerGetJobStatus(uuid string) string {
	rows, err := db.runParamSQLSelect("SELECT status FROM postsBackfill WHERE uuid = ?", uuid)
	if err != nil {
		core.LogError("Could not get the indexer job status from the database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			core.LogError("Could not get the rows for the indexer job status from the database: " + err.Error())
			return ""
		}
		return value
	}
	return ""
}
func (db *SQLite) IndexerGetHeadBlock(uuid string) uint64 {
	rows, err := db.runParamSQLSelect("SELECT headBlock FROM postsBackfill WHERE uuid = ?", uuid)
	if err != nil {
		core.LogError("Could not get the indexer head block from the database: " + err.Error())
		return 0
	}
	defer rows.Close()
	for rows.Next() {
		var value uint64
		err = rows.Scan(&value)
		if err != nil {
			core.LogError("Could not get the rows for the indexer head block from the database: " + err.Error())
			return 0
		}
		return value
	}
	return 0
}
func (db *SQLite) IndexerGetTailBlock(uuid string) uint64 {
	rows, err := db.runParamSQLSelect("SELECT tailBlock FROM postsBackfill WHERE uuid = ?", uuid)
	if err != nil {
		core.LogError("Could not get the indexer tail block from the database: " + err.Error())
		return 0
	}
	defer rows.Close()
	for rows.Next() {
		var value uint64
		err = rows.Scan(&value)
		if err != nil {
			core.LogError("Could not get the rows for the indexer tail block from the database: " + err.Error())
			return 0
		}
		return value
	}
	return 0
}
func (db *SQLite) IndexerGetRunningJobsUUIDs() []string {
	rows, err := db.getRows("SELECT uuid FROM postsBackfill WHERE status = 'running'")
	if err != nil {
		core.LogError("Could not find the running indexer job UUIDs from the database: " + err.Error())
		return []string{}
	}
	defer rows.Close()
	var uuids []string
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			core.LogError("Could not find the rows for the running indexer job UUIDs from the database: " + err.Error())
			return []string{}
		}
		uuids = append(uuids, value)
	}
	return uuids
}
func (db *SQLite) IndexerUpdateJobStatus(uuid string, status string) {
	timestamp := core.GetTimestamp()
	_, err := db.runParamSQLUpdate("UPDATE postsBackfill SET status = ?, timestamp = ? WHERE uuid = ?", status, timestamp, uuid)
	db.runSQL("PRAGMA wal_checkpoint(FULL)")
	if err != nil {
		core.LogError("Could not update the indexer job status in the database: " + err.Error())
	}
}
func (db *SQLite) IndexerUpdateHeadBlock(uuid string, headBlock uint64) {
	core.LogDebug("IndexerUpdateHeadBlock(): " + uuid + " - " + fmt.Sprint(headBlock))
	_, err := db.runParamSQLUpdate("UPDATE postsBackfill SET headBlock = ?, timestamp = ? WHERE uuid = ?", headBlock, core.GetTimestamp(), uuid)
	if err != nil {
		core.LogError("Could not update the indexer head block in the database: " + err.Error())
	}
}
func (db *SQLite) IndexerUpdateTailBlock(uuid string, tailBlock uint64) {
	_, err := db.runParamSQLUpdate("UPDATE postsBackfill SET tailBlock = ?, timestamp = ? WHERE uuid = ?", tailBlock, core.GetTimestamp(), uuid)
	if err != nil {
		core.LogError("Could not update the indexer tail block in the database: " + err.Error())
	}
}
func (db *SQLite) IndexerAddPost(txHash string, blockchain string, fromAddr string, toAddr string, parentTxHash string, amount uint64, timestamp uint64, data string, blockNumber uint64) {
	query := "INSERT INTO posts (txHash, blockchain, fromAddr, toAddr, parentTxHash, amount, timestamp, data, blockNumber) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, toAddr, parentTxHash, amount, timestamp, data, blockNumber)
	if err != nil {
		core.LogError("Could not add a post from the indexer into the database: " + err.Error())
	}
}
func (db *SQLite) IndexerResetJobs(blockchain string) {
	_, err := db.runParamSQLUpdate("UPDATE postsBackfill SET status = 'pending', headBlock = 0, tailBlock = 0, timestamp = ? WHERE blockchain = ?", core.GetTimestamp(), blockchain)
	if err != nil {
		core.LogError("Could not reset the indexer in the database: " + err.Error())
	}
}

// --- Onchain Tokenized Functions --- //
func (db *SQLite) OnchainP(txHash string, blockchain string, fromAddr string, toAddr string, parentTxHash string, amount uint64, timestamp uint64, data string, blockNumber uint64) {
	query := "INSERT INTO onchain_post (txHash, blockchain, fromAddr, toAddr, parentTxHash, amount, timestamp, data, blockNumber) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, toAddr, parentTxHash, amount, timestamp, data, blockNumber)
	if err != nil {
		core.LogError("Could not tokenize the post in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainPA(txHash string, blockchain string, fromAddr string, toAddr string, parentTxHash string, amount uint64, timestamp uint64, data string, blockNumber uint64) {
	query := "INSERT INTO onchain_post (txHash, blockchain, fromAddr, toAddr, parentTxHash, amount, timestamp, data, blockNumber) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, toAddr, parentTxHash, amount, timestamp, data, blockNumber)
	if err != nil {
		core.LogError("Could not tokenize the post in the database: " + err.Error())
		return
	}
	query2 := "INSERT INTO attachment (txHash, blockchain, address, name, contentType, size) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	_, err = db.runParamSQLUpdate(query2, txHash, blockchain, toAddr)
	if err != nil {
		core.LogError("Could not tokenize the attachment in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMN(blockchain string, address string, name string, timestamp uint64) {
	query := "INSERT INTO onchain_meta (blockchain, address, name, nameTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET name = excluded.name, nameTimestamp = excluded.nameTimestamp WHERE excluded.nameTimestamp > nameTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, name, timestamp)
	if err != nil {
		core.LogError("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMA(blockchain string, address string, avatar string, timestamp uint64) {
	query := "INSERT INTO onchain_meta (blockchain, address, avatar, avatarTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET avatar = excluded.avatar, avatarTimestamp = excluded.avatarTimestamp WHERE excluded.avatarTimestamp > avatarTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, avatar, timestamp)
	if err != nil {
		core.LogError("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMB(blockchain string, address string, banner string, timestamp uint64) {
	query := "INSERT INTO onchain_meta (blockchain, address, banner, bannerTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET banner = excluded.banner, bannerTimestamp = excluded.bannerTimestamp WHERE excluded.bannerTimestamp > bannerTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, banner, timestamp)
	if err != nil {
		core.LogError("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMBD(blockchain string, address string, birthdate uint64, timestamp uint64) {
	query := "INSERT INTO onchain_meta (blockchain, address, birthdate, birthdateTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET birthdate = excluded.birthdate, birthdateTimestamp = excluded.birthdateTimestamp WHERE excluded.birthdateTimestamp > birthdateTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, birthdate, timestamp)
	if err != nil {
		core.LogError("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainML(blockchain string, address string, location string, timestamp uint64) {
	query := "INSERT INTO onchain_meta (blockchain, address, location, locationTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET location = excluded.location, locationTimestamp = excluded.locationTimestamp WHERE excluded.locationTimestamp > locationTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, location, timestamp)
	if err != nil {
		core.LogError("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMW(blockchain string, address string, website string, timestamp uint64) {
	query := "INSERT INTO onchain_meta (blockchain, address, website, websiteTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET website = excluded.website, websiteTimestamp = excluded.websiteTimestamp WHERE excluded.websiteTimestamp > websiteTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, website, timestamp)
	if err != nil {
		core.LogError("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMD(blockchain string, address string, description string, timestamp uint64) {
	query := "INSERT INTO onchain_meta (blockchain, address, description, descriptionTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET description = excluded.description, descriptionTimestamp = excluded.descriptionTimestamp WHERE excluded.descriptionTimestamp > descriptionTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, description, timestamp)
	if err != nil {
		core.LogError("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainF(txHash string, blockchain string, fromAddr string, toAddr string, timestamp uint64) {
	query := "INSERT INTO onchain_follow (txHash, blockchain, toAddr, fromAddr, timestamp) VALUES (?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, toAddr, timestamp)
	if err != nil {
		core.LogError("Could not tokenize the follow in the database: " + err.Error())
	}
}
