package db

import (
	"YourPlace/src/core"
	"fmt"
)

// SchemaVersion is the current schema version of the database.
// Increment this value when adding a new migration.
const SchemaVersion = 2

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
