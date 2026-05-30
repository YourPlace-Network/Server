package db

import (
	"YourPlace/src/core"
	"context"
	"fmt"
	"strings"
	"time"

	ipfscid "github.com/ipfs/go-cid"
)

// SchemaVersion is the current schema version of the database.
// Increment this value when adding a new migration.
const SchemaVersion = 14

// Migration represents a single schema migration that upgrades the database from version N-1 to version N.
type Migration struct {
	Version     int
	Description string
	Up          func(db *SQLite) error
}

// migrations is the ordered list of all schema migrations.
// Each migration upgrades the database from the previous version to the specified version.
// To add a new migration:
//  1. Increment SchemaVersion constant above
//  2. Add a new Migration struct to this slice with the new version number
//  3. Create a corresponding migrateVN function below
var migrations = []Migration{
	{Version: 1, Description: "Initial schema - base tables created by createTables()", Up: migrateV1},
	{Version: 2, Description: "Add comment and reaction tables for social interactions", Up: migrateV2},
	{Version: 3, Description: "Add colors and colorsTimestamp columns to meta tables", Up: migrateV3},
	{Version: 4, Description: "Add oEmbed cache table for X.com embeds", Up: migrateV4},
	{Version: 5, Description: "Add cached ENS/NFD name and avatar columns to meta tables", Up: migrateV5},
	{Version: 6, Description: "Add Ethereum blockchain tables", Up: migrateV6},
	{Version: 7, Description: "Add bot and nsfw flags to meta tables", Up: migrateV7},
	{Version: 8, Description: "Add user notifications and notification seen tables", Up: migrateV8},
	{Version: 9, Description: "Add musicEmbed and musicEmbedTimestamp columns to meta tables", Up: migrateV9},
	{Version: 10, Description: "Remove redundant blockchain column from chain-specific tables and drop legacy onchain_comment/onchain_reaction tables", Up: migrateV10},
	{Version: 11, Description: "Move file tracking to local_files and onchain chain-specific files tables", Up: migrateV11},
	{Version: 12, Description: "Add deleted markers to chain-specific file tables", Up: migrateV12},
	{Version: 13, Description: "Drop deleted markers from chain-specific file tables", Up: migrateV13},
	{Version: 14, Description: "Add followers feed indexes", Up: migrateV14},
}

// --- Migration Functions --- //
// Each function upgrades the database schema from the previous version.
// Migrations should be idempotent where possible (check before altering).
func migrateV1(db *SQLite) error {
	// Version 1 is the initial schema.
	// Tables are created by createTables() using CREATE TABLE IF NOT EXISTS.
	// This migration is a no-op placeholder for the initial version.
	return nil
}
func migrateV2(db *SQLite) error {
	// Version 2 adds comment and reaction tables for social interactions
	tables := []string{
		"CREATE TABLE IF NOT EXISTS onchain_comment (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', parentTxHash TEXT DEFAULT '', amount REAL DEFAULT 0, timestamp INTEGER DEFAULT 0, data TEXT DEFAULT '', PRIMARY KEY(txHash, blockchain))",
		"CREATE TABLE IF NOT EXISTS onchain_algorand_comment (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', parentTxHash TEXT DEFAULT '', amount REAL DEFAULT 0, timestamp INTEGER DEFAULT 0, data TEXT DEFAULT '', PRIMARY KEY(txHash, blockchain))",
		"CREATE TABLE IF NOT EXISTS onchain_reaction (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', targetTxHash TEXT DEFAULT '', targetType TEXT DEFAULT 'post', reactionType TEXT DEFAULT '', timestamp INTEGER DEFAULT 0, PRIMARY KEY(txHash, blockchain))",
		"CREATE TABLE IF NOT EXISTS onchain_algorand_reaction (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', targetTxHash TEXT DEFAULT '', targetType TEXT DEFAULT 'post', reactionType TEXT DEFAULT '', timestamp INTEGER DEFAULT 0, PRIMARY KEY(txHash, blockchain))",
	}
	for _, createStatement := range tables {
		if _, err := db.database.Exec(createStatement); err != nil {
			return err
		}
	}
	return nil
}
func migrateV3(db *SQLite) error {
	// Version 3 adds colors and colorsTimestamp columns to meta tables for profile theming
	if err := db.migrateAddColumn("onchain_base_meta", "colors", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := db.migrateAddColumn("onchain_base_meta", "colorsTimestamp", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := db.migrateAddColumn("onchain_algorand_meta", "colors", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := db.migrateAddColumn("onchain_algorand_meta", "colorsTimestamp", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	return nil
}
func migrateV4(db *SQLite) error {
	// Version 4 adds oEmbed cache table for X.com embeds
	_, err := db.database.Exec("CREATE TABLE IF NOT EXISTS oembed_cache (url TEXT PRIMARY KEY, data TEXT DEFAULT '', fetchedAt INTEGER DEFAULT 0)")
	return err
}
func migrateV5(db *SQLite) error {
	// Version 5 adds cached ENS/NFD name and avatar columns to meta tables
	columns := []struct {
		table  string
		column string
		def    string
	}{
		{"onchain_algorand_meta", "ensAvatar", "TEXT DEFAULT ''"},
		{"onchain_algorand_meta", "ensAvatarTimestamp", "INTEGER DEFAULT 0"},
		{"onchain_algorand_meta", "ensName", "TEXT DEFAULT ''"},
		{"onchain_algorand_meta", "ensNameTimestamp", "INTEGER DEFAULT 0"},
		{"onchain_base_meta", "ensAvatar", "TEXT DEFAULT ''"},
		{"onchain_base_meta", "ensAvatarTimestamp", "INTEGER DEFAULT 0"},
		{"onchain_base_meta", "ensName", "TEXT DEFAULT ''"},
		{"onchain_base_meta", "ensNameTimestamp", "INTEGER DEFAULT 0"},
	}
	for _, col := range columns {
		if err := db.migrateAddColumn(col.table, col.column, col.def); err != nil {
			return err
		}
	}
	return nil
}
func migrateV6(db *SQLite) error {
	tables := []string{
		"CREATE TABLE IF NOT EXISTS ethereum_indexer_jobs (uuid TEXT PRIMARY KEY, blockchain TEXT, headBlock INTEGER, status TEXT, tailBlock INTEGER, timestamp INTEGER, rps INTEGER DEFAULT 0)",
		"CREATE TABLE IF NOT EXISTS onchain_ethereum_block (txHash TEXT, blockchain TEXT, blockerAddress TEXT, blockerBlockchain TEXT, blockeeAddress TEXT, blockeeBlockchain TEXT, key TEXT, value TEXT, timestamp INTEGER DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"CREATE TABLE IF NOT EXISTS onchain_ethereum_comment (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', parentTxHash TEXT DEFAULT '', amount REAL DEFAULT 0, timestamp INTEGER DEFAULT 0, data TEXT DEFAULT '', PRIMARY KEY(txHash, blockchain))",
		"CREATE TABLE IF NOT EXISTS onchain_ethereum_follow (txHash TEXT, blockchain TEXT, followerAddress TEXT, followerBlockchain TEXT, followeeAddress TEXT, followeeBlockchain TEXT, timestamp INTEGER DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"CREATE TABLE IF NOT EXISTS onchain_ethereum_meta (blockchain TEXT, address TEXT, avatar TEXT DEFAULT '', banner TEXT DEFAULT '', colors TEXT DEFAULT '', description TEXT DEFAULT '', ensAvatar TEXT DEFAULT '', ensName TEXT DEFAULT '', location TEXT DEFAULT '', name TEXT DEFAULT '', server TEXT DEFAULT '', vertical TEXT DEFAULT '', website TEXT DEFAULT '', addressTimestamp INTEGER DEFAULT 0, avatarTimestamp INTEGER DEFAULT 0, bannerTimestamp INTEGER DEFAULT 0, blockchainTimestamp INTEGER DEFAULT 0, colorsTimestamp INTEGER DEFAULT 0, descriptionTimestamp INTEGER DEFAULT 0, ensAvatarTimestamp INTEGER DEFAULT 0, ensNameTimestamp INTEGER DEFAULT 0, locationTimestamp INTEGER DEFAULT 0, nameTimestamp INTEGER DEFAULT 0, serverTimestamp INTEGER DEFAULT 0, verticalTimestamp INTEGER DEFAULT 0, websiteTimestamp INTEGER DEFAULT 0, PRIMARY KEY(blockchain, address))",
		"CREATE TABLE IF NOT EXISTS onchain_ethereum_post (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', parentTxHash TEXT DEFAULT '', amount REAL DEFAULT 0, timestamp INTEGER DEFAULT 0, data TEXT DEFAULT '', PRIMARY KEY(txHash, blockchain))",
		"CREATE TABLE IF NOT EXISTS onchain_ethereum_reaction (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', targetTxHash TEXT DEFAULT '', targetType TEXT DEFAULT 'post', reactionType TEXT DEFAULT '', timestamp INTEGER DEFAULT 0, PRIMARY KEY(txHash, blockchain))",
	}
	for _, createStatement := range tables {
		if _, err := db.database.Exec(createStatement); err != nil {
			return err
		}
	}
	return nil
}

func migrateV7(db *SQLite) error {
	columns := []struct {
		table  string
		column string
		def    string
	}{
		{"onchain_algorand_meta", "bot", "INTEGER DEFAULT 0"},
		{"onchain_algorand_meta", "botTimestamp", "INTEGER DEFAULT 0"},
		{"onchain_algorand_meta", "nsfw", "INTEGER DEFAULT 0"},
		{"onchain_algorand_meta", "nsfwTimestamp", "INTEGER DEFAULT 0"},
		{"onchain_base_meta", "bot", "INTEGER DEFAULT 0"},
		{"onchain_base_meta", "botTimestamp", "INTEGER DEFAULT 0"},
		{"onchain_base_meta", "nsfw", "INTEGER DEFAULT 0"},
		{"onchain_base_meta", "nsfwTimestamp", "INTEGER DEFAULT 0"},
		{"onchain_ethereum_meta", "bot", "INTEGER DEFAULT 0"},
		{"onchain_ethereum_meta", "botTimestamp", "INTEGER DEFAULT 0"},
		{"onchain_ethereum_meta", "nsfw", "INTEGER DEFAULT 0"},
		{"onchain_ethereum_meta", "nsfwTimestamp", "INTEGER DEFAULT 0"},
	}
	for _, col := range columns {
		if err := db.migrateAddColumn(col.table, col.column, col.def); err != nil {
			return err
		}
	}
	return nil
}
func migrateV8(db *SQLite) error {
	tables := []string{
		"CREATE TABLE IF NOT EXISTS user_notifications (id TEXT PRIMARY KEY, userAddress TEXT, userBlockchain TEXT, fromAddress TEXT, fromBlockchain TEXT, type TEXT, targetTxHash TEXT DEFAULT '', reactionType TEXT DEFAULT '', timestamp INTEGER DEFAULT 0, dismissed INTEGER DEFAULT 0)",
		"CREATE TABLE IF NOT EXISTS user_notification_seen (userAddress TEXT, userBlockchain TEXT, lastSeenAt INTEGER DEFAULT 0, PRIMARY KEY (userAddress, userBlockchain))",
	}
	for _, createStatement := range tables {
		if _, err := db.database.Exec(createStatement); err != nil {
			return err
		}
	}
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_user_notifications_user ON user_notifications (userAddress, userBlockchain)",
		"CREATE INDEX IF NOT EXISTS idx_user_notifications_timestamp ON user_notifications (timestamp)",
	}
	for _, idx := range indexes {
		if _, err := db.database.Exec(idx); err != nil {
			return err
		}
	}
	return nil
}
func migrateV9(db *SQLite) error {
	columns := []struct {
		table  string
		column string
		def    string
	}{
		{"onchain_algorand_meta", "musicEmbed", "TEXT DEFAULT ''"},
		{"onchain_algorand_meta", "musicEmbedTimestamp", "INTEGER DEFAULT 0"},
		{"onchain_base_meta", "musicEmbed", "TEXT DEFAULT ''"},
		{"onchain_base_meta", "musicEmbedTimestamp", "INTEGER DEFAULT 0"},
		{"onchain_ethereum_meta", "musicEmbed", "TEXT DEFAULT ''"},
		{"onchain_ethereum_meta", "musicEmbedTimestamp", "INTEGER DEFAULT 0"},
	}
	for _, col := range columns {
		if err := db.migrateAddColumn(col.table, col.column, col.def); err != nil {
			return err
		}
	}
	return nil
}
func migrateV10(db *SQLite) error {
	// Version 10 drops the redundant blockchain column from chain-specific tables.
	// SQLite strategy: drop chain-specific tables and recreate them at the new schema;
	// the indexer will re-populate them from on-chain data on next run.
	// Also drops the legacy (and unused) generic onchain_comment / onchain_reaction tables.
	tablesToDrop := []string{
		"algorand_indexer_jobs",
		"base_indexer_jobs",
		"ethereum_indexer_jobs",
		"onchain_algorand_block",
		"onchain_algorand_comment",
		"onchain_algorand_follow",
		"onchain_algorand_meta",
		"onchain_algorand_post",
		"onchain_algorand_reaction",
		"onchain_base_block",
		"onchain_base_comment",
		"onchain_base_follow",
		"onchain_base_meta",
		"onchain_base_post",
		"onchain_base_reaction",
		"onchain_comment",
		"onchain_ethereum_block",
		"onchain_ethereum_comment",
		"onchain_ethereum_follow",
		"onchain_ethereum_meta",
		"onchain_ethereum_post",
		"onchain_ethereum_reaction",
		"onchain_reaction",
	}
	for _, table := range tablesToDrop {
		if err := db.migrateDropTable(table); err != nil {
			return err
		}
	}
	// Recreate chain-specific tables at the new schema (no blockchain column).
	// Legacy onchain_comment / onchain_reaction are not recreated.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return db.createTables(ctx)
}
func migrateV11(db *SQLite) error {
	if err := createFileTrackingTablesSQLite(db); err != nil {
		return err
	}
	return backfillLegacyFilesSQLite(db)
}
func migrateV12(db *SQLite) error {
	columns := []struct {
		table  string
		column string
		def    string
	}{
		{"onchain_base_files", "deletedTxHash", "TEXT DEFAULT ''"},
		{"onchain_base_files", "deletedTimestamp", "INTEGER DEFAULT 0"},
		{"onchain_algorand_files", "deletedTxHash", "TEXT DEFAULT ''"},
		{"onchain_algorand_files", "deletedTimestamp", "INTEGER DEFAULT 0"},
		{"onchain_ethereum_files", "deletedTxHash", "TEXT DEFAULT ''"},
		{"onchain_ethereum_files", "deletedTimestamp", "INTEGER DEFAULT 0"},
	}
	for _, col := range columns {
		if err := db.migrateAddColumn(col.table, col.column, col.def); err != nil {
			return err
		}
	}
	return nil
}
func migrateV13(db *SQLite) error {
	columns := []struct {
		table  string
		column string
	}{
		{"onchain_base_files", "deletedTxHash"},
		{"onchain_base_files", "deletedTimestamp"},
		{"onchain_algorand_files", "deletedTxHash"},
		{"onchain_algorand_files", "deletedTimestamp"},
		{"onchain_ethereum_files", "deletedTxHash"},
		{"onchain_ethereum_files", "deletedTimestamp"},
	}
	for _, col := range columns {
		if err := db.migrateDropColumn(col.table, col.column); err != nil {
			return err
		}
	}
	return nil
}
func migrateV14(db *SQLite) error {
	for _, blockchain := range core.ValidNetworks {
		postTable := "onchain_" + blockchain + "_post"
		followTable := "onchain_" + blockchain + "_follow"
		if err := db.migrateCreateIndex("idx_"+postTable+"_timestamp_txhash", postTable, "timestamp, txHash"); err != nil {
			return err
		}
		if err := db.migrateCreateIndex("idx_"+postTable+"_from_timestamp_txhash", postTable, "fromAddress, timestamp, txHash"); err != nil {
			return err
		}
		if err := db.migrateCreateIndex("idx_"+followTable+"_feed", followTable, "followerAddress, followerBlockchain, followeeBlockchain, followeeAddress"); err != nil {
			return err
		}
	}
	return nil
}

func normalizeLegacyCID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "ipfs://") {
		value = strings.TrimPrefix(value, "ipfs://")
	}
	if idx := strings.IndexAny(value, "/?#"); idx != -1 {
		value = value[:idx]
	}
	if dot := strings.Index(value, "."); dot != -1 {
		candidate := value[:dot]
		if isValidCIDString(candidate) {
			return candidate
		}
	}
	if isValidCIDString(value) {
		return value
	}
	return ""
}

func legacyCIDFromFields(cidValue string, fileURL string) string {
	cidValue = normalizeLegacyCID(cidValue)
	if cidValue != "" {
		return cidValue
	}
	return normalizeLegacyCID(fileURL)
}

func createFileTrackingTablesSQLite(db *SQLite) error {
	queries := []string{
		"CREATE TABLE IF NOT EXISTS local_files (ownerAddress TEXT, ownerBlockchain TEXT, cid TEXT, fileHash TEXT, mimeType TEXT, fileName TEXT, size INTEGER, addedDate INTEGER, source TEXT, state TEXT, PRIMARY KEY (ownerAddress, ownerBlockchain, cid))",
		"CREATE TABLE IF NOT EXISTS local_posts (localPostUUID TEXT PRIMARY KEY, ownerAddress TEXT, ownerBlockchain TEXT, timestamp INTEGER DEFAULT 0, payload TEXT DEFAULT '')",
		"CREATE TABLE IF NOT EXISTS local_post_files (localPostUUID TEXT, cid TEXT, PRIMARY KEY (localPostUUID, cid))",
		"CREATE TABLE IF NOT EXISTS onchain_base_files (txHash TEXT, fileIndex INTEGER, fromAddress TEXT DEFAULT '', cid TEXT DEFAULT '', mimeType TEXT DEFAULT '', fileName TEXT DEFAULT '', size INTEGER DEFAULT 0, timestamp INTEGER DEFAULT 0, source TEXT DEFAULT '', PRIMARY KEY (txHash, source, fileIndex))",
		"CREATE TABLE IF NOT EXISTS onchain_algorand_files (txHash TEXT, fileIndex INTEGER, fromAddress TEXT DEFAULT '', cid TEXT DEFAULT '', mimeType TEXT DEFAULT '', fileName TEXT DEFAULT '', size INTEGER DEFAULT 0, timestamp INTEGER DEFAULT 0, source TEXT DEFAULT '', PRIMARY KEY (txHash, source, fileIndex))",
		"CREATE TABLE IF NOT EXISTS onchain_ethereum_files (txHash TEXT, fileIndex INTEGER, fromAddress TEXT DEFAULT '', cid TEXT DEFAULT '', mimeType TEXT DEFAULT '', fileName TEXT DEFAULT '', size INTEGER DEFAULT 0, timestamp INTEGER DEFAULT 0, source TEXT DEFAULT '', PRIMARY KEY (txHash, source, fileIndex))",
	}
	for _, query := range queries {
		if _, err := db.database.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

func backfillLegacyFilesSQLite(db *SQLite) error {
	if !sqliteTableExists(db, "files") {
		return nil
	}
	ownerAddress := db.AuthGetServerOwnerAddress()
	ownerBlockchain := db.AuthGetServerOwnerNetwork()
	if ownerAddress != "" && ownerBlockchain != "" {
		rows, err := db.runParamSQLSelect("SELECT fileHash, mimeType, fileName, size, addedDate, COALESCE(cid, ''), COALESCE(fileURL, ''), COALESCE(source, '') FROM files")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var fileHash, mimeType, fileName, cidValue, fileURL, source string
			var size, addedDate int64
			if err = rows.Scan(&fileHash, &mimeType, &fileName, &size, &addedDate, &cidValue, &fileURL, &source); err != nil {
				return err
			}
			cid := legacyCIDFromFields(cidValue, fileURL)
			if cid == "" {
				continue
			}
			state := "staged"
			if cidValue != "" || fileURL != "" {
				state = "publishedLocalCopy"
			}
			if source == "" {
				source = "direct_upload"
			}
			query := `INSERT INTO local_files (ownerAddress, ownerBlockchain, cid, fileHash, mimeType, fileName, size, addedDate, source, state)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (ownerAddress, ownerBlockchain, cid) DO UPDATE SET
					fileHash = excluded.fileHash,
					mimeType = excluded.mimeType,
					fileName = excluded.fileName,
					size = excluded.size,
					addedDate = excluded.addedDate,
					source = excluded.source,
					state = excluded.state`
			if _, err = db.runParamSQLUpdate(query, ownerAddress, ownerBlockchain, cid, fileHash, mimeType, fileName, size, addedDate, source, state); err != nil {
				return err
			}
		}
	}
	if !sqliteTableExists(db, "file_txn_hash") {
		return nil
	}
	for _, blockchain := range core.ValidNetworks {
		postTable := "onchain_" + blockchain + "_post"
		commentTable := "onchain_" + blockchain + "_comment"
		if !sqliteTableExists(db, postTable) || !sqliteTableExists(db, commentTable) {
			continue
		}
		query := fmt.Sprintf(`SELECT fth.txHash, f.mimeType, f.fileName, f.size, COALESCE(f.cid, ''), COALESCE(f.fileURL, ''), COALESCE(f.source, ''),
			COALESCE(p.fromAddress, c.fromAddress, ''), COALESCE(p.timestamp, c.timestamp, f.addedDate),
			CASE
				WHEN c.txHash IS NOT NULL THEN 'comment_attachment'
				WHEN p.txHash IS NOT NULL THEN 'post_attachment'
				ELSE 'direct_upload'
			END
			FROM file_txn_hash fth
			INNER JOIN files f ON f.fileUUID = fth.fileUUID
			LEFT JOIN onchain_%s_post p ON p.txHash = fth.txHash
			LEFT JOIN onchain_%s_comment c ON c.txHash = fth.txHash
			WHERE fth.blockchain = ?`, blockchain, blockchain)
		rows, err := db.runParamSQLSelect(query, blockchain)
		if err != nil {
			return err
		}
		fileIndexes := make(map[string]int)
		for rows.Next() {
			var txHash, mimeType, fileName, cidValue, fileURL, storedSource, fromAddress, derivedSource string
			var size, timestamp int64
			if err = rows.Scan(&txHash, &mimeType, &fileName, &size, &cidValue, &fileURL, &storedSource, &fromAddress, &timestamp, &derivedSource); err != nil {
				rows.Close()
				return err
			}
			cid := legacyCIDFromFields(cidValue, fileURL)
			if cid == "" {
				continue
			}
			source := derivedSource
			if source == "" {
				source = storedSource
			}
			if source == "" {
				source = "direct_upload"
			}
			if fromAddress == "" {
				fromAddress = ownerAddress
			}
			key := txHash + ":" + source
			fileIndex := fileIndexes[key]
			fileIndexes[key] = fileIndex + 1
			insertQuery := fmt.Sprintf("INSERT INTO onchain_%s_files (txHash, fileIndex, fromAddress, cid, mimeType, fileName, size, timestamp, source) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, source, fileIndex) DO NOTHING", blockchain)
			if _, err = db.runParamSQLUpdate(insertQuery, txHash, fileIndex, fromAddress, cid, mimeType, fileName, size, timestamp, source); err != nil {
				rows.Close()
				return err
			}
		}
		rows.Close()
	}
	return nil
}

// Example migration templates for future use:
//
// func migrateV2(db *SQLite) error {
//     // Add a new column to an existing table
//     return db.migrateAddColumn("onchain_post", "blockNumber", "INTEGER DEFAULT 0")
// }
//
// func migrateV3(db *SQLite) error {
//     // Create indexes for performance
//     indexes := []string{
//         "CREATE INDEX IF NOT EXISTS idx_onchain_post_from ON onchain_post(fromAddress)",
//         "CREATE INDEX IF NOT EXISTS idx_onchain_post_timestamp ON onchain_post(timestamp)",
//     }
//     for _, idx := range indexes {
//         if _, err := db.database.Exec(idx); err != nil {
//             return err
//         }
//     }
//     return nil
// }
//
// func migrateV4(db *SQLite) error {
//     // Create a new table
//     _, err := db.database.Exec(`
//         CREATE TABLE IF NOT EXISTS reactions (
//             txHash TEXT,
//             blockchain TEXT,
//             postTxHash TEXT,
//             fromAddress TEXT,
//             reactionType TEXT,
//             timestamp INTEGER DEFAULT 0,
//             PRIMARY KEY (txHash, blockchain)
//         )
//     `)
//     return err
// }

// --- Migration Execution --- //
// RunMigrations checks the current schema version and runs all necessary migrations
// to bring the database up to the current SchemaVersion.
// For fresh databases (version 0), it sets the version directly since createTables()
// already created tables at the latest schema version.
func (db *SQLite) RunMigrations() error {
	currentVersion := db.getSchemaVersion()
	targetVersion := SchemaVersion
	// Fresh database - tables were just created at the latest schema by createTables()
	// Set version directly and skip migrations
	if currentVersion == 0 {
		db.setSchemaVersion(targetVersion)
		core.LogDebug(fmt.Sprintf("Fresh database initialized at schema version %d", targetVersion))
		return nil
	}
	// Prevent running an older binary against a newer database
	if currentVersion > targetVersion {
		return fmt.Errorf("database schema version (%d) is ahead of binary schema version (%d) - please upgrade the binary", currentVersion, targetVersion)
	}
	// No migrations needed
	if currentVersion == targetVersion {
		core.LogDebug(fmt.Sprintf("Database schema is up to date (version %d)", currentVersion))
		return nil
	}
	core.LogDebug(fmt.Sprintf("Upgrading database schema from version %d to %d", currentVersion, targetVersion))
	// Run each migration sequentially
	for _, migration := range migrations {
		if migration.Version <= currentVersion {
			continue // Skip already-applied migrations
		}
		if migration.Version > targetVersion {
			break // Don't run migrations beyond target
		}
		core.LogDebug(fmt.Sprintf("Running migration %d: %s", migration.Version, migration.Description))
		if err := migration.Up(db); err != nil {
			return fmt.Errorf("migration %d (%s) failed: %w", migration.Version, migration.Description, err)
		}
		// Update schema version after each successful migration
		db.setSchemaVersion(migration.Version)
		core.LogDebug(fmt.Sprintf("Migration %d completed successfully", migration.Version))
	}
	core.LogDebug(fmt.Sprintf("Database schema upgrade completed (now at version %d)", targetVersion))
	return nil
}

// --- Migration Helper Functions --- //
// migrateAddColumn safely adds a column to a table if it doesn't already exist.
// SQLite doesn't support IF NOT EXISTS for ALTER TABLE ADD COLUMN, so we check first.
func (db *SQLite) migrateAddColumn(table, column, definition string) error {
	// Check if column already exists using PRAGMA table_info
	rows, err := db.database.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("failed to get table info for %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan table info: %w", err)
		}
		if name == column {
			core.LogDebug(fmt.Sprintf("Column %s.%s already exists, skipping", table, column))
			return nil // Column already exists, nothing to do
		}
	}
	// Add the column
	query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)
	_, err = db.database.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to add column %s to %s: %w", column, table, err)
	}
	core.LogDebug(fmt.Sprintf("Added column %s.%s", table, column))
	return nil
}
func (db *SQLite) migrateDropColumn(table, column string) error {
	rows, err := db.database.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("failed to get table info for %s: %w", table, err)
	}
	defer rows.Close()
	columnExists := false
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan table info: %w", err)
		}
		if name == column {
			columnExists = true
			break
		}
	}
	if !columnExists {
		core.LogDebug(fmt.Sprintf("Column %s.%s does not exist, skipping", table, column))
		return nil
	}
	query := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column)
	if _, err := db.database.Exec(query); err != nil {
		return fmt.Errorf("failed to drop column %s from %s: %w", column, table, err)
	}
	core.LogDebug(fmt.Sprintf("Dropped column %s.%s", table, column))
	return nil
}

// migrateCreateIndex creates an index if it doesn't already exist.
func (db *SQLite) migrateCreateIndex(indexName, table, columns string) error {
	query := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)", indexName, table, columns)
	_, err := db.database.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create index %s: %w", indexName, err)
	}
	core.LogDebug(fmt.Sprintf("Created index %s on %s(%s)", indexName, table, columns))
	return nil
}

// migrateDropTable drops a table if it exists (use with caution).
func (db *SQLite) migrateDropTable(table string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", table)
	_, err := db.database.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to drop table %s: %w", table, err)
	}
	core.LogDebug(fmt.Sprintf("Dropped table %s", table))
	return nil
}

// migrateRenameTable renames a table.
func (db *SQLite) migrateRenameTable(oldName, newName string) error {
	query := fmt.Sprintf("ALTER TABLE %s RENAME TO %s", oldName, newName)
	_, err := db.database.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to rename table %s to %s: %w", oldName, newName, err)
	}
	core.LogDebug(fmt.Sprintf("Renamed table %s to %s", oldName, newName))
	return nil
}

func sqliteTableExists(db *SQLite, table string) bool {
	rows, err := db.database.Query("SELECT name FROM sqlite_master WHERE type='table' AND name = ?", table)
	if err != nil {
		return false
	}
	defer rows.Close()
	return rows.Next()
}

func isValidCIDString(value string) bool {
	_, err := ipfscid.Decode(value)
	return err == nil
}
