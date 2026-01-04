package db

import (
	"YourPlace/src/core"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/google/uuid"
)

type SQLite struct {
	database *sql.DB
	path     string
}

func (db *SQLite) Init(path string) {
	startupCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	db.path = path
	database, err := sql.Open("sqlite", path)
	if err != nil || database == nil {
		core.LogFatal("Could not open sqlite db")
		return
	}
	database.SetMaxOpenConns(50)
	database.SetMaxIdleConns(20)
	database.SetConnMaxLifetime(15 * time.Minute)
	database.SetConnMaxIdleTime(3 * time.Minute)
	db.database = database
	// Enable Caching
	_, err = database.Exec("PRAGMA cache_size = -64000") // 64MB
	if err != nil {
		core.LogDebug("Could not set cache size: " + err.Error())
	}
	// Enable WAL mode for better concurrency
	_, err = database.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		core.LogDebug("Could not enable WAL mode: " + err.Error())
	}
	// Set busy timeout to 10 seconds
	_, err = database.Exec("PRAGMA busy_timeout=10000")
	if err != nil {
		core.LogDebug("Could not set busy timeout: " + err.Error())
	}
	// Create Tables
	err = db.createTables(startupCtx)
	if err != nil {
		core.LogDebug("Could not create tables: " + err.Error())
	}
	// Check and run schema migrations (see schema.go)
	if err := db.RunMigrations(); err != nil {
		core.LogFatal("Schema migration failed: " + err.Error())
		return
	}
}

// --- SQL --- //
func (db *SQLite) runSQL(query string) {
	_, err := db.database.Exec(query)
	if err != nil {
		core.LogDebug("Could not run SQLite query: " + query + " - " + err.Error())
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
				return nil, core.LogDebugReturn("Query preparation canceled: " + err.Error())
			}
			return nil, core.LogDebugReturn("Could not prepare SQLite query: " + query + " - " + err.Error())
		}
		defer statement.Close()
		queryCtx := context.Background()
		rows, err := statement.QueryContext(queryCtx, params...)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil, core.LogDebugReturn("Query canceled: " + err.Error())
			}
			return nil, core.LogDebugReturn("Could not run SQLite query: " + err.Error())
		}
		return rows, nil
	}
	return nil, core.LogDebugReturn("Invalid sql method")
}
func (db *SQLite) runParamSQLUpdate(query string, params ...interface{}) (sql.Result, error) {
	// For INSERT, UPDATE, DELETE, etc. use Exec()
	queryUpper := strings.ToUpper(strings.TrimSpace(query))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if strings.HasPrefix(queryUpper, "SELECT") || strings.HasPrefix(queryUpper, "EXPLAIN") {
		return nil, core.LogDebugReturn("Invalid method for SQL update")
	}
	statement, err := db.database.PrepareContext(ctx, query)
	if err != nil {
		return nil, core.LogDebugReturn("Could not prepare SQLite query: " + query + " - " + err.Error())
	}
	defer statement.Close()
	result, err := statement.ExecContext(ctx, params...)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, core.LogDebugReturn("Query timed out after 15s: " + err.Error())
		}
		return nil, core.LogDebugReturn("Could not run SQLite query: " + query + " - " + err.Error())
	}
	return result, nil
}
func (db *SQLite) getRows(query string) (*sql.Rows, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stmt, err := db.database.PrepareContext(ctx, query)
	if err != nil {
		return nil, core.LogDebugReturn("prepare failed: " + err.Error())
	}
	defer stmt.Close()
	return stmt.QueryContext(ctx)
}
func (db *SQLite) rowCount(query string) (int, error) {
	rows, err := db.runParamSQLSelect(query)
	if err != nil {
		return 0, core.LogDebugReturn("Could not get row count: " + err.Error())
	}
	defer rows.Close()
	rowCount := 0
	for rows.Next() {
		rowCount++
	}
	err = rows.Err()
	if err != nil {
		return 0, core.LogDebugReturn("Could not get row count: " + err.Error())
	}
	return rowCount, nil
}
func (db *SQLite) flushTable(name string) {
	query := "DELETE * FROM " + sanitizeSQLiteTableName(name)
	db.runSQL(query)
}
func (db *SQLite) deleteTable(name string) {
	query := "DROP TABLE IF EXISTS " + sanitizeSQLiteTableName(name)
	db.runSQL(query)
}

// --- Helper --- //
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
func (db *SQLite) withTransaction(fn func(*sql.Tx) error) error {
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
func (db *SQLite) createTables(ctx context.Context) error {
	/* createTables creates all database tables using CREATE TABLE IF NOT EXISTS.
	// These table definitions should always reflect the LATEST schema version.
	//
	// When modifying the database schema:
	//  1. Update the CREATE TABLE statement(s) below to the new structure
	//  2. Increment SchemaVersion in schema.go
	//  3. Add a migration function in schema.go to ALTER existing tables
	//
	// This ensures:
	//   - Fresh installations get the latest schema directly (no migrations needed)
	//   - Existing databases are upgraded via migrations in schema.go
	*/
	// Tables schema map - keep at LATEST schema version
	tables := map[string]string{
		"meta":          "CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT)",
		"settings":      "CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT)",
		"files":         "CREATE TABLE IF NOT EXISTS files (fileUUID TEXT PRIMARY KEY, fileHash TEXT, mimeType TEXT, fileName TEXT, size INTEGER, addedDate INTEGER, cid TEXT, fileURL TEXT, source TEXT)",
		"file_txn_hash": "CREATE TABLE IF NOT EXISTS file_txn_hash (fileUUID TEXT, txHash TEXT, blockchain TEXT, PRIMARY KEY (fileUUID, txHash, blockchain))",
		"auth_nonce":    "CREATE TABLE IF NOT EXISTS auth_nonce (nonce TEXT PRIMARY KEY, status TEXT, timestamp INTEGER)",
		"auth_expired":  "CREATE TABLE IF NOT EXISTS auth_expired (uuid TEXT PRIMARY KEY, status TEXT)",
		"login_nonce":   "CREATE TABLE IF NOT EXISTS login_nonce (nonce TEXT PRIMARY KEY, domain TEXT, expiration INTEGER, nonceHash TEXT)",
		"csrf_tokens":   "CREATE TABLE IF NOT EXISTS csrf_tokens (token TEXT PRIMARY KEY, expiration INTEGER)",
		"notifications": "CREATE TABLE IF NOT EXISTS notifications (uid TEXT PRIMARY KEY, message TEXT, timestamp INTEGER DEFAULT 0)",
		"wallets":       "CREATE TABLE IF NOT EXISTS wallets (publicKey TEXT, blockchain TEXT, address TEXT, encryptedPrivateKey TEXT, isDefault INTEGER DEFAULT 0, PRIMARY KEY (publicKey, blockchain))",
		// Base-specific tables
		"base_indexer_jobs":     "CREATE TABLE IF NOT EXISTS base_indexer_jobs (uuid TEXT PRIMARY KEY, blockchain TEXT, headBlock INTEGER, status TEXT, tailBlock INTEGER, timestamp INTEGER, rps INTEGER DEFAULT 0)",
		"onchain_base_post":     "CREATE TABLE IF NOT EXISTS onchain_base_post (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', parentTxHash TEXT DEFAULT '', amount REAL DEFAULT 0, timestamp INTEGER DEFAULT 0, data TEXT DEFAULT '', PRIMARY KEY(txHash, blockchain))",
		"onchain_base_meta":     "CREATE TABLE IF NOT EXISTS onchain_base_meta (blockchain TEXT, address TEXT, name TEXT DEFAULT '', avatar TEXT DEFAULT '', description TEXT DEFAULT '', location TEXT DEFAULT '', banner TEXT DEFAULT '', website TEXT DEFAULT '', vertical TEXT DEFAULT '', server TEXT DEFAULT '', blockchainTimestamp INTEGER DEFAULT 0, addressTimestamp INTEGER DEFAULT 0, nameTimestamp INTEGER DEFAULT 0, avatarTimestamp INTEGER DEFAULT 0, descriptionTimestamp INTEGER DEFAULT 0, locationTimestamp INTEGER DEFAULT 0, bannerTimestamp INTEGER DEFAULT 0, websiteTimestamp INTEGER DEFAULT 0, verticalTimestamp INTEGER DEFAULT 0, serverTimestamp INTEGER DEFAULT 0, PRIMARY KEY(blockchain, address))",
		"onchain_base_block":    "CREATE TABLE IF NOT EXISTS onchain_base_block (txHash TEXT, blockchain TEXT, blockerAddress TEXT, blockerBlockchain TEXT, blockeeAddress TEXT, blockeeBlockchain TEXT, key TEXT, value TEXT, timestamp INTEGER DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"onchain_base_follow":   "CREATE TABLE IF NOT EXISTS onchain_base_follow (txHash TEXT, blockchain TEXT, followerAddress TEXT, followerBlockchain TEXT, followeeAddress TEXT, followeeBlockchain TEXT, timestamp INTEGER DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"onchain_base_comment":  "CREATE TABLE IF NOT EXISTS onchain_base_comment (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', parentTxHash TEXT DEFAULT '', parentType TEXT DEFAULT 'post', amount REAL DEFAULT 0, timestamp INTEGER DEFAULT 0, data TEXT DEFAULT '', PRIMARY KEY(txHash, blockchain))",
		"onchain_base_reaction": "CREATE TABLE IF NOT EXISTS onchain_base_reaction (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', targetTxHash TEXT DEFAULT '', targetType TEXT DEFAULT 'post', reactionType TEXT DEFAULT '', timestamp INTEGER DEFAULT 0, PRIMARY KEY(txHash, blockchain))",
		// Algorand-specific tables
		"algorand_indexer_jobs":     "CREATE TABLE IF NOT EXISTS algorand_indexer_jobs (uuid TEXT PRIMARY KEY, blockchain TEXT, headBlock INTEGER, status TEXT, tailBlock INTEGER, timestamp INTEGER, rps INTEGER DEFAULT 0)",
		"onchain_algorand_post":     "CREATE TABLE IF NOT EXISTS onchain_algorand_post (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', parentTxHash TEXT DEFAULT '', amount REAL DEFAULT 0, timestamp INTEGER DEFAULT 0, data TEXT DEFAULT '', PRIMARY KEY(txHash, blockchain))",
		"onchain_algorand_meta":     "CREATE TABLE IF NOT EXISTS onchain_algorand_meta (blockchain TEXT, address TEXT, name TEXT DEFAULT '', avatar TEXT DEFAULT '', description TEXT DEFAULT '', location TEXT DEFAULT '', banner TEXT DEFAULT '', website TEXT DEFAULT '', vertical TEXT DEFAULT '', server TEXT DEFAULT '', blockchainTimestamp INTEGER DEFAULT 0, addressTimestamp INTEGER DEFAULT 0, nameTimestamp INTEGER DEFAULT 0, avatarTimestamp INTEGER DEFAULT 0, descriptionTimestamp INTEGER DEFAULT 0, locationTimestamp INTEGER DEFAULT 0, bannerTimestamp INTEGER DEFAULT 0, websiteTimestamp INTEGER DEFAULT 0, verticalTimestamp INTEGER DEFAULT 0, serverTimestamp INTEGER DEFAULT 0, PRIMARY KEY(blockchain, address))",
		"onchain_algorand_block":    "CREATE TABLE IF NOT EXISTS onchain_algorand_block (txHash TEXT, blockchain TEXT, blockerAddress TEXT, blockerBlockchain TEXT, blockeeAddress TEXT, blockeeBlockchain TEXT, key TEXT, value TEXT, timestamp INTEGER DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"onchain_algorand_follow":   "CREATE TABLE IF NOT EXISTS onchain_algorand_follow (txHash TEXT, blockchain TEXT, followerAddress TEXT, followerBlockchain TEXT, followeeAddress TEXT, followeeBlockchain TEXT, timestamp INTEGER DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"onchain_algorand_comment":  "CREATE TABLE IF NOT EXISTS onchain_algorand_comment (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', parentTxHash TEXT DEFAULT '', parentType TEXT DEFAULT 'post', amount REAL DEFAULT 0, timestamp INTEGER DEFAULT 0, data TEXT DEFAULT '', PRIMARY KEY(txHash, blockchain))",
		"onchain_algorand_reaction": "CREATE TABLE IF NOT EXISTS onchain_algorand_reaction (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', targetTxHash TEXT DEFAULT '', targetType TEXT DEFAULT 'post', reactionType TEXT DEFAULT '', timestamp INTEGER DEFAULT 0, PRIMARY KEY(txHash, blockchain))",
	}
	for _, createStatement := range tables {
		err := db.execWithRetry(ctx, createStatement, 3)
		if err != nil {
			return core.LogDebugReturn("Table creation failed: " + err.Error())
		}
	}
	return nil
}
func (db *SQLite) getSchemaVersion() int {
	rows, err := db.runParamSQLSelect("SELECT value FROM meta WHERE key = ?", "schema_version")
	if err != nil {
		return 0
	}
	defer rows.Close()
	if rows.Next() {
		var versionStr string
		err = rows.Scan(&versionStr)
		if err != nil {
			return 0
		}
		var version int
		_, err = fmt.Sscanf(versionStr, "%d", &version)
		if err != nil {
			return 0
		}
		return version
	}
	return 0
}
func (db *SQLite) setSchemaVersion(version int) {
	query := "INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)"
	_, err := db.runParamSQLUpdate(query, "schema_version", fmt.Sprintf("%d", version))
	if err != nil {
		core.LogDebug("Could not set schema version: " + err.Error())
	}
}
func (db *SQLite) runExternalSQLFile(path string) {
	core.LogWarn("Running external SQL file: " + path)
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		core.LogDebug("Could not read external SQL file: " + err.Error())
		return
	}
	_, err = db.database.Prepare(string(sqlBytes)) // try to validate by parsing
	if err != nil {
		core.LogDebug("Could not validate external SQL file: " + err.Error())
		return
	}
	db.runSQL(string(sqlBytes))
}
func (db *SQLite) ExportSnapshot(exportPath string) error {
	if db.database == nil {
		return core.LogDebugReturn("Database connection not initialized")
	}
	core.LogDebug("Exporting SQLite Snapshot to: " + exportPath)
	// Tables to export
	tables := []string{
		"file_txn_hash",
		"files",
	}
	for _, _blockchain := range core.ValidNetworks {
		tables = append(tables, "onchain_"+strings.ToLower(_blockchain)+"_post")
		tables = append(tables, "onchain_"+strings.ToLower(_blockchain)+"_meta")
		tables = append(tables, "onchain_"+strings.ToLower(_blockchain)+"_follow")
		tables = append(tables, "onchain_"+strings.ToLower(_blockchain)+"_comment")
		tables = append(tables, "onchain_"+strings.ToLower(_blockchain)+"_reaction")
		tables = append(tables, "onchain_"+strings.ToLower(_blockchain)+"_block")
		tables = append(tables, strings.ToLower(_blockchain)+"_indexer_jobs")
	}
	// Create buffer to hold the serialized data
	var buffer bytes.Buffer
	// Create metadata for the export
	metaData := map[string]interface{}{
		"timestamp": core.GetTimestamp(),
		"version":   "1.0",
		"tables":    tables,
	}
	// Create the output file
	exportFile, err := os.Create(exportPath)
	if err != nil {
		return core.LogDebugReturn("Could not create export file: " + err.Error())
	}
	defer exportFile.Close()
	// Use a gzip writer for compression
	gzWriter, err := gzip.NewWriterLevel(exportFile, gzip.BestCompression)
	if err != nil {
		return core.LogDebugReturn("Could not create gzip writer: " + err.Error())
	}
	defer gzWriter.Close()
	// First write the metadata
	metaJSON, err := json.Marshal(metaData)
	if err != nil {
		return core.LogDebugReturn("Could not serialize metadata: " + err.Error())
	}
	// Write metadata length as a binary header (4 bytes)
	err = binary.Write(gzWriter, binary.LittleEndian, uint32(len(metaJSON)))
	if err != nil {
		return err
	}
	// Write metadata
	_, err = gzWriter.Write(metaJSON)
	if err != nil {
		return core.LogDebugReturn("Could not write metadata: " + err.Error())
	}
	// Export each table directly to the compressed stream
	for _, table := range tables {
		core.LogDebug("Exporting table: " + table)
		// Get table schema - use parameterized query to prevent SQL injection
		rows, err := db.runParamSQLSelect("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table)
		if err != nil {
			return core.LogDebugReturn("Could not get table schema: " + err.Error())
		}
		var createStatement string
		if rows.Next() {
			err = rows.Scan(&createStatement)
			if err != nil {
				rows.Close()
				return core.LogDebugReturn("Could not read table schema: " + err.Error())
			}
		}
		rows.Close()
		if createStatement == "" {
			return core.LogDebugReturn("Table not found: " + table)
		}
		// Reset buffer
		buffer.Reset()
		// Write schema to buffer
		err = binary.Write(&buffer, binary.LittleEndian, uint32(len(createStatement)))
		if err != nil {
			return core.LogDebugReturn("Could not write schema length: " + err.Error())
		}
		_, err = buffer.WriteString(createStatement)
		if err != nil {
			return core.LogDebugReturn("Could not write schema: " + err.Error())
		}
		// Get all data from the table - table name comes from predefined list so safe
		dataRows, err := db.runParamSQLSelect("SELECT * FROM " + sanitizeSQLiteTableName(table))
		if err != nil {
			return core.LogDebugReturn("Could not get table data: " + err.Error())
		}
		// Get column information
		columns, err := dataRows.Columns()
		if err != nil {
			dataRows.Close()
			return core.LogDebugReturn("Could not get column information: " + err.Error())
		}
		// Serialize column count and names
		err = binary.Write(&buffer, binary.LittleEndian, uint32(len(columns)))
		if err != nil {
			dataRows.Close()
			return core.LogDebugReturn("Could not write column count: " + err.Error())
		}
		for _, column := range columns {
			err = binary.Write(&buffer, binary.LittleEndian, uint32(len(column)))
			if err != nil {
				dataRows.Close()
				return core.LogDebugReturn("Could not write column name length: " + err.Error())
			}
			_, err = buffer.WriteString(column)
			if err != nil {
				dataRows.Close()
				return core.LogDebugReturn("Could not write column name: " + err.Error())
			}
		}
		// Count rows (first pass)
		rowCount := 0
		for dataRows.Next() {
			rowCount++
		}
		// Check for error in first pass
		err = dataRows.Err()
		if err != nil {
			dataRows.Close()
			return core.LogDebugReturn("Could not read rows: " + err.Error())
		}
		dataRows.Close()
		// Write row count
		err = binary.Write(&buffer, binary.LittleEndian, uint32(rowCount))
		if err != nil {
			return core.LogDebugReturn("Could not write row count: " + err.Error())
		}
		// If there are no rows, continue to next table
		if rowCount == 0 {
			// Write this table's buffer to compressed file
			_, err = gzWriter.Write(buffer.Bytes())
			if err != nil {
				return core.LogDebugReturn("Could not write table buffer: " + err.Error())
			}
			continue
		}
		// Get data again for second pass - table name comes from predefined list so safe
		dataRows, err = db.runParamSQLSelect("SELECT * FROM " + sanitizeSQLiteTableName(table))
		if err != nil {
			return core.LogDebugReturn("Could not get table data (second pass): " + err.Error())
		}
		// Write this table's header to compressed file now
		_, err = gzWriter.Write(buffer.Bytes())
		if err != nil {
			dataRows.Close()
			return core.LogDebugReturn("Could not write table header: " + err.Error())
		}
		// Reset buffer for row data
		buffer.Reset()
		rowBuffer := bytes.NewBuffer(nil)
		// Serialize each row
		rowsProcessed := 0
		values := make([]interface{}, len(columns))
		valuePointers := make([]interface{}, len(columns))
		for i := range values {
			valuePointers[i] = &values[i]
		}
		for dataRows.Next() {
			err = dataRows.Scan(valuePointers...)
			if err != nil {
				dataRows.Close()
				return core.LogDebugReturn("Could not scan row: " + err.Error())
			}
			// Reset row buffer
			rowBuffer.Reset()
			// Serialize each value in the row
			for _, value := range values {
				if value == nil {
					// Write a type indicator for NULL (0)
					err = rowBuffer.WriteByte(0)
					if err != nil {
						dataRows.Close()
						return core.LogDebugReturn("Could not write NULL indicator: " + err.Error())
					}
				} else {
					// Determine the type and serialize accordingly
					switch v := value.(type) {
					case int64:
						rowBuffer.WriteByte(1) // Type indicator for int64
						err = binary.Write(rowBuffer, binary.LittleEndian, v)
						if err != nil {
							return err
						}
					case float64:
						rowBuffer.WriteByte(2) // Type indicator for float64
						err = binary.Write(rowBuffer, binary.LittleEndian, v)
						if err != nil {
							return err
						}
					case []byte:
						rowBuffer.WriteByte(3) // Type indicator for []byte
						err = binary.Write(rowBuffer, binary.LittleEndian, uint32(len(v)))
						if err != nil {
							return err
						}
						rowBuffer.Write(v)
					case string:
						rowBuffer.WriteByte(4) // Type indicator for string
						err = binary.Write(rowBuffer, binary.LittleEndian, uint32(len(v)))
						if err != nil {
							return err
						}
						rowBuffer.WriteString(v)
					case time.Time:
						rowBuffer.WriteByte(5) // Type indicator for time.Time
						err = binary.Write(rowBuffer, binary.LittleEndian, v.Unix())
						if err != nil {
							return err
						}
					default:
						// For any other type, convert to string
						str := fmt.Sprintf("%v", v)
						rowBuffer.WriteByte(4) // Type indicator for string
						err = binary.Write(rowBuffer, binary.LittleEndian, uint32(len(str)))
						if err != nil {
							return err
						}
						rowBuffer.WriteString(str)
					}
				}
			}
			// Write row length and then row data
			rowData := rowBuffer.Bytes()
			err = binary.Write(&buffer, binary.LittleEndian, uint32(len(rowData)))
			if err != nil {
				dataRows.Close()
				return core.LogDebugReturn("Could not write row length: " + err.Error())
			}
			_, err = buffer.Write(rowData)
			if err != nil {
				dataRows.Close()
				return core.LogDebugReturn("Could not write row data: " + err.Error())
			}
			rowsProcessed++
			// Flush to gzip writer ever 1000 rows to avoid memory buildup
			if buffer.Len() > 1024*1024 || rowsProcessed%1000 == 0 {
				_, err = gzWriter.Write(buffer.Bytes())
				if err != nil {
					dataRows.Close()
					return core.LogDebugReturn("Could not write batch of rows to buffer: " + err.Error())
				}
				buffer.Reset()
			}
			// Log progress for large tables
			if rowsProcessed%10000 == 0 {
				core.LogDebug(fmt.Sprintf("Exported %d/%d rows from table %s", rowsProcessed, rowCount, table))
			}
		}
		// Check for error in second pass
		err = dataRows.Err()
		if err != nil {
			dataRows.Close()
			return core.LogDebugReturn("Could not read rows (second pass): " + err.Error())
		}
		dataRows.Close()
		// Write any remaining data
		if buffer.Len() > 0 {
			_, err = gzWriter.Write(buffer.Bytes())
			if err != nil {
				return core.LogDebugReturn("Could not write remaining rows to buffer: " + err.Error())
			}
		}
		core.LogDebug(fmt.Sprintf("Exported %d rows from table %s", rowsProcessed, table))
	}
	// Close the gzip writer to flush any remaining data
	err = gzWriter.Close()
	if err != nil {
		return core.LogDebugReturn("Could not close gzip writer: " + err.Error())
	}
	core.LogInfo("SQLite Snapshot Exported Successfully To: " + exportPath)
	return nil
}
func (db *SQLite) ImportSnapshotNoMetadata(importPath string) error {
	if db.database == nil {
		return core.LogDebugReturn("Database connection not initialized")
	}
	// Open the import file
	importFile, err := os.Open(importPath)
	if err != nil {
		return core.LogDebugReturn("Could not open import file: " + err.Error())
	}
	defer importFile.Close()
	// Create a gzip reader
	gzReader, err := gzip.NewReader(importFile)
	if err != nil {
		return core.LogDebugReturn("Could not create gzip reader: " + err.Error())
	}
	defer gzReader.Close()
	// Process tables without reading metadata header (new snapshot format)
	// Continue reading tables until EOF
	for {
		// Read schema length
		var schemaLength uint32
		err = binary.Read(gzReader, binary.LittleEndian, &schemaLength)
		if err != nil {
			if err == io.EOF {
				break // End of file, all tables processed successfully
			}
			return core.LogDebugReturn("Could not read schema length: " + err.Error())
		}
		// Read schema
		schemaBytes := make([]byte, schemaLength)
		_, err = io.ReadFull(gzReader, schemaBytes)
		if err != nil {
			return core.LogDebugReturn("Could not read schema: " + err.Error())
		}
		schema := string(schemaBytes)
		// Extract table name from schema
		tableName := extractTableName(schema)
		if tableName == "" {
			return core.LogDebugReturn("Could not extract table name from schema: " + schema)
		}
		core.LogDebug("Importing table: " + tableName)
		// Ensure table exists
		_, err = db.database.Exec(schema)
		if err != nil {
			core.LogDebug("Table already exists, continuing with import: " + tableName)
		}
		// Read column count
		var columnCount uint32
		err = binary.Read(gzReader, binary.LittleEndian, &columnCount)
		if err != nil {
			return core.LogDebugReturn("Could not read column count: " + err.Error())
		}
		// Read column names
		columns := make([]string, columnCount)
		for i := 0; i < int(columnCount); i++ {
			var nameLength uint32
			err = binary.Read(gzReader, binary.LittleEndian, &nameLength)
			if err != nil {
				return core.LogDebugReturn("Could not read column name length: " + err.Error())
			}
			nameBytes := make([]byte, nameLength)
			_, err = io.ReadFull(gzReader, nameBytes)
			if err != nil {
				return core.LogDebugReturn("Could not read column name: " + err.Error())
			}
			columns[i] = string(nameBytes)
		}
		// Read row count
		var rowCount uint32
		err = binary.Read(gzReader, binary.LittleEndian, &rowCount)
		if err != nil {
			return core.LogDebugReturn("Could not read row count: " + err.Error())
		}
		// If no rows, continue to next table
		if rowCount == 0 {
			core.LogDebug("No rows to import for table: " + tableName)
			continue
		}
		// Start transaction for this table
		tx, _err := db.database.Begin()
		if _err != nil {
			return core.LogDebugReturn("Could not start transaction: " + _err.Error())
		}
		// Prepare insert statement
		placeholders := make([]string, len(columns))
		for i := range columns {
			placeholders[i] = "?"
		}
		var insertSQL string
		noQuoteTableName := strings.Trim(tableName, `"`)
		if strings.HasSuffix(noQuoteTableName, "_indexer_jobs") {
			insertSQL = fmt.Sprintf("INSERT OR REPLACE INTO %s (%s) VALUES (%s)", tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))
		} else {
			insertSQL = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING", tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))
		}
		statement, _err := tx.Prepare(insertSQL)
		if _err != nil {
			__err := tx.Rollback()
			if __err != nil {
				return __err
			} else {
				return core.LogDebugReturn("Could not prepare insert statement: " + _err.Error())
			}
		}
		// Import each row
		rowsProcessed := 0
		for rowIdx := 0; rowIdx < int(rowCount); rowIdx++ {
			// Read row length
			var rowLength uint32
			_err = binary.Read(gzReader, binary.LittleEndian, &rowLength)
			if _err != nil {
				___err := statement.Close()
				if ___err != nil {
					return ___err
				}
				__err := tx.Rollback()
				if __err != nil {
					return __err
				}
				return core.LogDebugReturn("Could not read row length: " + _err.Error())
			}
			// Read row data
			rowData := make([]byte, rowLength)
			_, _err = io.ReadFull(gzReader, rowData)
			if _err != nil {
				__err := statement.Close()
				if __err != nil {
					return __err
				}
				___err := tx.Rollback()
				if ___err != nil {
					return ___err
				}
				return core.LogDebugReturn("Could not read row data: " + _err.Error())
			}
			// Parse row data
			rowReader := bytes.NewReader(rowData)
			values := make([]interface{}, len(columns))
			for i := range columns {
				// Read type indicator
				typeIndicator, __err := rowReader.ReadByte()
				if __err != nil {
					___err := statement.Close()
					if ___err != nil {
						return ___err
					}
					____err := tx.Rollback()
					if ____err != nil {
						return ____err
					}
					return core.LogDebugReturn("Could not read type indicator: " + __err.Error())
				}
				// Parse based on type
				switch typeIndicator {
				case 0: // NULL
					values[i] = nil
				case 1: // int64
					var value int64
					_err := binary.Read(rowReader, binary.LittleEndian, &value)
					if _err != nil {
						return _err
					}
					values[i] = value
				case 2: // float64
					var value float64
					_err := binary.Read(rowReader, binary.LittleEndian, &value)
					if _err != nil {
						return _err
					}
					values[i] = value
				case 3: // []byte
					var length uint32
					_err := binary.Read(rowReader, binary.LittleEndian, &length)
					if _err != nil {
						return _err
					}
					_bytes := make([]byte, length)
					_, __err := io.ReadFull(rowReader, _bytes)
					if __err != nil {
						___err := statement.Close()
						if ___err != nil {
							return ___err
						}
						____err := tx.Rollback()
						if ____err != nil {
							return ____err
						}
						return core.LogDebugReturn("Could not read []byte value: " + __err.Error())
					}
					values[i] = _bytes
				case 4: // string
					var length uint32
					_err := binary.Read(rowReader, binary.LittleEndian, &length)
					if _err != nil {
						return _err
					}
					_bytes := make([]byte, length)
					_, __err := io.ReadFull(rowReader, _bytes)
					if __err != nil {
						___err := statement.Close()
						if ___err != nil {
							return ___err
						}
						____err := tx.Rollback()
						if ____err != nil {
							return ____err
						}
						return core.LogDebugReturn("Could not read string value: " + __err.Error())
					}
					values[i] = string(_bytes)
				case 5: // time.Time
					var unixTime int64
					_err := binary.Read(rowReader, binary.LittleEndian, &unixTime)
					if _err != nil {
						return _err
					}
					values[i] = time.Unix(unixTime, 0)
				default:
					_err := statement.Close()
					if _err != nil {
						return _err
					}
					__err := tx.Rollback()
					if __err != nil {
						return __err
					}
					return core.LogDebugReturn("Unknown type indicator: " + string(typeIndicator))
				}
			}
			// Execute insert
			_, _err = statement.Exec(values...)
			if _err != nil {
				__err := statement.Close()
				if __err != nil {
					return __err
				}
				___err := tx.Rollback()
				if ___err != nil {
					return ___err
				}
				return core.LogDebugReturn("Could not execute insert: " + _err.Error())
			}
			rowsProcessed++
			// Log Progress
			if rowsProcessed%10000 == 0 {
				core.LogDebug(fmt.Sprintf("Imported %d/%d rows from table %s", rowsProcessed, rowCount, tableName))
			}
		}
		// Commit transaction
		__err := statement.Close()
		if __err != nil {
			return __err
		}
		_err = tx.Commit()
		if _err != nil {
			return core.LogDebugReturn("Could not commit transaction: " + _err.Error())
		}
		core.LogDebug(fmt.Sprintf("Imported %d rows from table %s", rowsProcessed, tableName))
	}
	core.LogInfo("SQLite Snapshot Imported Successfully From: " + importPath)
	return nil
}
func (db *SQLite) ImportSnapshot(importPath string) error {
	if db.database == nil {
		return core.LogDebugReturn("Database connection not initialized")
	}
	// Open the import file
	importFile, err := os.Open(importPath)
	if err != nil {
		return core.LogDebugReturn("Could not open import file: " + err.Error())
	}
	defer importFile.Close()
	// Create a gzip reader
	gzReader, err := gzip.NewReader(importFile)
	if err != nil {
		return core.LogDebugReturn("Could not create gzip reader: " + err.Error())
	}
	defer gzReader.Close()
	// Read metadata length
	var metaLength uint32
	err = binary.Read(gzReader, binary.LittleEndian, &metaLength)
	if err != nil {
		return core.LogDebugReturn("Could not read metadata length: " + err.Error())
	}
	// Read metadata
	metaBytes := make([]byte, metaLength)
	_, err = io.ReadFull(gzReader, metaBytes)
	if err != nil {
		return core.LogDebugReturn("Could not read metadata: " + err.Error())
	}
	var metadata map[string]interface{}
	err = json.Unmarshal(metaBytes, &metadata)
	if err != nil {
		return core.LogDebugReturn("Could not parse metadata: " + err.Error())
	}
	// Get tables
	tablesInterface, ok := metadata["tables"]
	if !ok {
		return core.LogDebugReturn("Metadata missing tables")
	}
	tablesArray, ok := tablesInterface.([]interface{})
	if !ok {
		return core.LogDebugReturn("Metadata tables not an array")
	}
	// Process each table
	for range tablesArray {
		// Read schema length
		var schemaLength uint32
		err = binary.Read(gzReader, binary.LittleEndian, &schemaLength)
		if err != nil {
			if err == io.EOF {
				break // End of file, no more tables
			}
			return core.LogDebugReturn("Could not read schema length: " + err.Error())
		}
		// Read schema
		schemaBytes := make([]byte, schemaLength)
		_, err = io.ReadFull(gzReader, schemaBytes)
		if err != nil {
			return core.LogDebugReturn("Could not read schema: " + err.Error())
		}
		schema := string(schemaBytes)
		// Extract table name from schema
		tableName := extractTableName(schema)
		if tableName == "" {
			return core.LogDebugReturn("Could not extract table name from schema: " + schema)
		}
		core.LogDebug("Importing table: " + tableName)
		// Ensure table exists
		_, err = db.database.Exec(schema)
		if err != nil {
			core.LogDebug("Table already exists, continuing with import: " + tableName)
		}
		// Read column count
		var columnCount uint32
		err = binary.Read(gzReader, binary.LittleEndian, &columnCount)
		if err != nil {
			return core.LogDebugReturn("Could not read column count: " + err.Error())
		}
		// Read column names
		columns := make([]string, columnCount)
		for i := 0; i < int(columnCount); i++ {
			var nameLength uint32
			err = binary.Read(gzReader, binary.LittleEndian, &nameLength)
			if err != nil {
				return core.LogDebugReturn("Could not read column name length: " + err.Error())
			}
			nameBytes := make([]byte, nameLength)
			_, err = io.ReadFull(gzReader, nameBytes)
			if err != nil {
				return core.LogDebugReturn("Could not read column name: " + err.Error())
			}
			columns[i] = string(nameBytes)
		}
		// Read row count
		var rowCount uint32
		err = binary.Read(gzReader, binary.LittleEndian, &rowCount)
		if err != nil {
			return core.LogDebugReturn("Could not read row count: " + err.Error())
		}
		// If no rows, continue to next table
		if rowCount == 0 {
			core.LogDebug("No rows to import for table: " + tableName)
			continue
		}
		// Start transaction for this table
		tx, _err := db.database.Begin()
		if _err != nil {
			return core.LogDebugReturn("Could not start transaction: " + _err.Error())
		}
		// Prepare insert statement
		placeholders := make([]string, len(columns))
		for i := range columns {
			placeholders[i] = "?"
		}
		var insertSQL string
		noQuoteTableName := strings.Trim(tableName, `"`)
		if strings.HasSuffix(noQuoteTableName, "_indexer_jobs") {
			insertSQL = fmt.Sprintf("INSERT OR REPLACE INTO %s (%s) VALUES (%s)", tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))
		} else {
			insertSQL = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING", tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))
		}
		statement, _err := tx.Prepare(insertSQL)
		if _err != nil {
			__err := tx.Rollback()
			if __err != nil {
				return __err
			}
			return core.LogDebugReturn("Could not prepare insert statement: " + _err.Error())
		}
		// Import each row
		rowsProcessed := 0
		for rowIdx := 0; rowIdx < int(rowCount); rowIdx++ {
			// Read row length
			var rowLength uint32
			_err = binary.Read(gzReader, binary.LittleEndian, &rowLength)
			if _err != nil {
				__err := statement.Close()
				if __err != nil {
					return __err
				}
				___err := tx.Rollback()
				if ___err != nil {
					return ___err
				}
				return core.LogDebugReturn("Could not read row length: " + _err.Error())
			}
			// Read row data
			rowData := make([]byte, rowLength)
			_, _err = io.ReadFull(gzReader, rowData)
			if _err != nil {
				__err := statement.Close()
				if __err != nil {
					return __err
				}
				___err := tx.Rollback()
				if ___err != nil {
					return ___err
				}
				return core.LogDebugReturn("Could not read row data: " + _err.Error())
			}
			// Parse row data
			rowReader := bytes.NewReader(rowData)
			values := make([]interface{}, len(columns))
			for i := range columns {
				// Read type indicator
				typeIndicator, __err := rowReader.ReadByte()
				if __err != nil {
					___err := statement.Close()
					if ___err != nil {
						return ___err
					}
					____err := tx.Rollback()
					if ____err != nil {
						return ____err
					}
					return core.LogDebugReturn("Could not read type indicator: " + __err.Error())
				}
				// Parse based on type
				switch typeIndicator {
				case 0: // NULL
					values[i] = nil
				case 1: // int64
					var value int64
					___err := binary.Read(rowReader, binary.LittleEndian, &value)
					if ___err != nil {
						return ___err
					}
					values[i] = value
				case 2: // float64
					var value float64
					___err := binary.Read(rowReader, binary.LittleEndian, &value)
					if ___err != nil {
						return ___err
					}
					values[i] = value
				case 3: // []byte
					var length uint32
					___err := binary.Read(rowReader, binary.LittleEndian, &length)
					if ___err != nil {
						return ___err
					}
					_bytes := make([]byte, length)
					_, ____err := io.ReadFull(rowReader, _bytes)
					if ____err != nil {
						_____err := statement.Close()
						if _____err != nil {
							return _____err
						}
						______err := tx.Rollback()
						if ______err != nil {
							return ______err
						}
						return core.LogDebugReturn("Could not read []byte value: " + ____err.Error())
					}
					values[i] = _bytes
				case 4: // string
					var length uint32
					binary.Read(rowReader, binary.LittleEndian, &length)
					bytes := make([]byte, length)
					_, err := io.ReadFull(rowReader, bytes)
					if err != nil {
						statement.Close()
						tx.Rollback()
						return core.LogDebugReturn("Could not read string value: " + err.Error())
					}
					values[i] = string(bytes)
				case 5: // time.Time
					var unixTime int64
					binary.Read(rowReader, binary.LittleEndian, &unixTime)
					values[i] = time.Unix(unixTime, 0)
				default:
					statement.Close()
					tx.Rollback()
					return core.LogDebugReturn("Unknown type indicator: " + string(typeIndicator))
				}
			}
			// Execute insert
			_, _err = statement.Exec(values...)
			if _err != nil {
				statement.Close()
				tx.Rollback()
				return core.LogDebugReturn("Could not execute insert: " + _err.Error())
			}
			rowsProcessed++
			// Log Progress
			if rowsProcessed%10000 == 0 {
				core.LogDebug(fmt.Sprintf("Imported %d/%d rows from table %s", rowsProcessed, rowCount, tableName))
			}
		}
		// Commit transaction
		statement.Close()
		_err = tx.Commit()
		if _err != nil {
			return core.LogDebugReturn("Could not commit transaction: " + _err.Error())
		}
		core.LogDebug(fmt.Sprintf("Imported %d rows from table %s", rowsProcessed, tableName))
	}
	core.LogInfo("SQLite Snapshot Imported Successfully From: " + importPath)
	return nil
}
func extractTableName(createSQL string) string { // extractTableName extracts the table name from a CREATE TABLE statement
	re := regexp.MustCompile(`CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([^\s(]+)`)
	match := re.FindStringSubmatch(createSQL)
	if len(match) > 1 {
		return match[1]
	}
	return ""
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

// --- Metadata & Settings --- //
func (db *SQLite) MetaUpdateValue(key string, value string) {
	query := "INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value"
	_, err := db.runParamSQLUpdate(query, key, value)
	if err != nil {
		core.LogDebug("Meta update failed: " + err.Error())
	}
}
func (db *SQLite) MetaGetValue(key string) string {
	rows, err := db.runParamSQLSelect("SELECT value FROM meta WHERE key = ?", key)
	if err != nil {
		core.LogDebug("Could not get meta value, query failed: " + err.Error())
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
				core.LogDebug("Could not get setting value for key: " + key + " - query failed: " + err.Error())
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
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		_, err := db.runParamSQLUpdate(query, key, value)
		if err == nil {
			return
		}
		if strings.Contains(err.Error(), "database is locked") || strings.Contains(err.Error(), "SQLITE_BUSY") {
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
func (db *SQLite) SettingsDeleteValue(key string) error {
	_, err := db.runParamSQLUpdate("DELETE FROM settings WHERE key = ?", key)
	if err != nil {
		return core.LogDebugReturn("Could not delete setting: " + err.Error())
	}
	return nil
}

// --- Profile --- //
func (db *SQLite) ProfileGetName(address string, blockchain string) string {
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
func (db *SQLite) ProfileGetAvatar(address string, blockchain string) string {
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
func (db *SQLite) ProfileGetBanner(address string, blockchain string) string {
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
func (db *SQLite) ProfileGetDescription(address string, blockchain string) string {
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
func (db *SQLite) ProfileGetLocation(address string, blockchain string) string {
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
func (db *SQLite) ProfileGetWebsite(address string, blockchain string) string {
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
func (db *SQLite) ProfileGetVertical(address string, blockchain string) string {
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
func (db *SQLite) ProfileGetJoinedDate(address string, blockchain string) *int64 {
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
func (db *SQLite) ProfileGetPosts(address string, blockchain string) []map[string]interface{} {
	var posts []map[string]interface{}
	query := fmt.Sprintf("SELECT txHash, COALESCE(parentTxHash, '') as parentTxHash, timestamp, data FROM onchain_%s_post WHERE fromAddress = LOWER (?) AND blockchain = ? AND data IS NOT NULL ORDER BY timestamp DESC", blockchain)
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
			core.LogDebug("Could not get attachments for post: " + err.Error()) // No bail because we can still return the text of the post
		} else if rowsAttachments != nil {
			for rowsAttachments.Next() {
				var mimeType string
				var size uint64
				var fileUrl string
				var fileName string
				err := rowsAttachments.Scan(&mimeType, &size, &fileUrl, &fileName)
				if err != nil {
					core.LogDebug("Could parse rows for post attachment: " + err.Error())
					break // bail rowsAttachments for loop
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
		if attachments != nil { // Adds attachment field only if attachments exist for this post
			post["attachments"] = attachments
		}
		posts = append(posts, post)
	}
	return posts
}
func (db *SQLite) ProfileGetFollowerCount(address string, blockchain string) *int64 {
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
func (db *SQLite) ProfileGetFollowingCount(address string, blockchain string) *int64 {
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
func (db *SQLite) ProfileIsFollower(followeeAddress string, followeeBlockchain string, followerAddress string, followerBlockchain string) bool {
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
func (db *SQLite) SearchGetPosts(query string) []map[string]interface{} {
	var posts []map[string]interface{}
	for _, _blockchain := range core.ValidNetworks {
		selectionQueryFmt := "SELECT txHash, COALESCE(parentTxHash, '') as parentHash, timestamp, data, fromAddress, blockchain FROM onchain_%s_post WHERE LOWER (data) LIKE LOWER (?)"
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
				core.LogDebug("Could not get attachments for post: " + err.Error()) // No bail because we can still return the text of the post
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
						break // bail rowsAttachments for loop
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
func (db *SQLite) SearchGetProfiles(query string) []map[string]interface{} {
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
func (db *SQLite) DiscoverGetRandomProfiles(limit int) []map[string]interface{} {
	var profiles []map[string]interface{}
	for _, _blockchain := range core.ValidNetworks {
		sqlQueryFmt := "SELECT address, blockchain FROM (SELECT address, blockchain FROM onchain_%s_meta UNION SELECT followeeAddress, followeeBlockchain FROM onchain_%s_follow) ORDER BY RANDOM() LIMIT ?"
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
func (db *SQLite) DiscoverGetTopByFollowers(limit int) []map[string]interface{} {
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
func (db *SQLite) DiscoverGetTopByPosts(limit int) []map[string]interface{} {
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
func (db *SQLite) AuthGetNonceStatus(nonce string) string {
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
func (db *SQLite) AuthUpdateNonce(nonce string, status string) {
	_, err := db.runParamSQLUpdate("INSERT INTO auth_nonce (nonce, status) VALUES (?, ?) ON CONFLICT (nonce) DO UPDATE SET status = excluded.status", nonce, status)
	if err != nil {
		core.LogDebug("Could not update auth nonce in database: " + err.Error())
	}
}
func (db *SQLite) AuthDeleteNonce(nonce string) {
	_, err := db.runParamSQLUpdate("DELETE FROM auth_nonce WHERE nonce = ?", nonce)
	if err != nil {
		core.LogDebug("Could not delete the auth nonce from the database: " + err.Error())
	}
}
func (db *SQLite) AuthExpireCookie(uuid string) {
	_, err := db.runParamSQLUpdate("INSERT INTO auth_expired (uuid, status) VALUES (?, 'expired') ON CONFLICT (uuid) DO UPDATE SET status = 'expired'", uuid)
	if err != nil {
		core.LogDebug("Could not expire the auth cookie from the database: " + err.Error())
	}
}
func (db *SQLite) AuthGetCookieStatus(uuid string) string {
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
func (db *SQLite) AuthUpdateLoginNonce(nonce string, domain string, expiration uint64, nonceHash string) {
	query := "INSERT INTO login_nonce (nonce, domain, expiration, nonceHash) VALUES (?, ?, ?, ?) ON CONFLICT (nonce) DO NOTHING"
	_, err := db.runParamSQLUpdate(query, nonce, domain, expiration, nonceHash)
	if err != nil {
		core.LogDebug("Could not update login_nonce: " + err.Error())
	}
}
func (db *SQLite) AuthGetLoginNonceByHash(nonceHash string) string {
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
func (db *SQLite) AuthDeleteLoginNonce(nonce string) {
	_, err := db.runParamSQLUpdate("DELETE FROM login_nonce WHERE nonce = ?", nonce)
	if err != nil {
		core.LogDebug("Could not delete the login nonce from the database: " + err.Error())
	}
}
func (db *SQLite) AuthExpireLoginNonce() {
	_, err := db.runParamSQLUpdate("DELETE FROM login_nonce WHERE expiration < ?", core.GetTimestamp())
	if err != nil {
		core.LogDebug("Could not delete any expired login nonces from the database: " + err.Error())
	}
}
func (db *SQLite) AuthGetServerOwnerAddress() string {
	rows, err := db.runParamSQLSelect("SELECT value FROM meta WHERE key = 'accountAddress' LIMIT 1")
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
func (db *SQLite) AuthGetServerOwnerNetwork() string {
	rows, err := db.runParamSQLSelect("SELECT value FROM meta WHERE key = 'accountNetwork' LIMIT 1")
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
func (db *SQLite) FileAdd(fileUUID string, fileHash string, mimeType string, fileName string, size int64) {
	query := "INSERT INTO files (fileUUID, fileHash, mimeType, fileName, size, addedDate) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING"
	_, err := db.runParamSQLUpdate(query, fileUUID, fileHash, mimeType, fileName, size, core.GetTimestamp())
	if err != nil {
		core.LogDebug("Could not add the file to the database: " + err.Error())
	}
}
func (db *SQLite) IPFSAdd(fileUUID string, cid string) {
	fileURL := "ipfs://" + cid
	query := "UPDATE files SET cid = ?, fileURL = ? WHERE fileUUID = ?"
	_, err := db.runParamSQLUpdate(query, cid, fileURL, fileUUID)
	if err != nil {
		core.LogDebug("Could not add the IPFS CID to the database: " + err.Error())
	}
}
func (db *SQLite) GetFileHashFromUUID(uuid string) string {
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
func (db *SQLite) IndexerCreateJob(uuid string, blockchain string) {
	core.LogDebug("IndexerCreateJob(): " + uuid + " - " + blockchain)
	timestamp := core.GetTimestamp()
	queryFmt := "INSERT INTO %s_indexer_jobs (uuid, blockchain, headBlock, status, tailBlock, timestamp, rps) VALUES (?, ?, 0, 'pending', 0, ?, 0) ON CONFLICT (uuid) DO UPDATE SET status = excluded.status, tailBlock = excluded.tailBlock, timestamp = excluded.timestamp"
	query := fmt.Sprintf(queryFmt, blockchain)
	core.LogDebug("IndexerCreateJob(): " + query)
	_, err := db.runParamSQLUpdate(query, uuid, blockchain, timestamp)
	if err != nil {
		core.LogDebug("Could not create indexer job in the database: " + err.Error())
	}
}
func (db *SQLite) IndexerGetJobUUID(blockchain string) string {
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
func (db *SQLite) IndexerGetJobStatus(uuid string) string {
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
func (db *SQLite) IndexerGetHeadBlock(uuid string) uint64 {
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
func (db *SQLite) IndexerGetTailBlock(uuid string) uint64 {
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
func (db *SQLite) IndexerGetRunningJobsUUIDs() []string {
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
func (db *SQLite) IndexerUpdateJobStatus(uuid string, status string) {
	timestamp := core.GetTimestamp()
	for _, blockchain := range core.ValidNetworks {
		queryFmt := "UPDATE %s_indexer_jobs SET status = ?, timestamp = ? WHERE uuid = ?"
		query := fmt.Sprintf(queryFmt, blockchain)
		_, err := db.runParamSQLUpdate(query, status, timestamp, uuid)
		db.runSQL("PRAGMA wal_checkpoint(FULL)")
		if err != nil {
			core.LogDebug("Could not update the indexer job status in the database: " + err.Error())
		}
	}
}
func (db *SQLite) IndexerUpdateHeadBlock(uuid string, headBlock uint64) {
	for _, blockchain := range core.ValidNetworks {
		queryFmt := "UPDATE %s_indexer_jobs SET headBlock = ?, timestamp = ? WHERE uuid = ?"
		query := fmt.Sprintf(queryFmt, blockchain)
		_, err := db.runParamSQLUpdate(query, headBlock, core.GetTimestamp(), uuid)
		if err != nil {
			core.LogDebug("Could not update the indexer head block in the database: " + err.Error())
		}
	}
}
func (db *SQLite) IndexerUpdateTailBlock(uuid string, tailBlock uint64) {
	for _, blockchain := range core.ValidNetworks {
		queryFmt := "UPDATE %s_indexer_jobs SET tailBlock = ?, timestamp = ? WHERE uuid = ?"
		query := fmt.Sprintf(queryFmt, blockchain)
		_, err := db.runParamSQLUpdate(query, tailBlock, core.GetTimestamp(), uuid)
		if err != nil {
			core.LogDebug("Could not update the indexer tail block in the database: " + err.Error())
		}
	}
}
func (db *SQLite) IndexerUpdateJobSpeed(uuid string, speed uint64) {
	for _, blockchain := range core.ValidNetworks {
		queryFmt := "UPDATE %s_indexer_jobs SET rps = ? WHERE uuid = ?"
		query := fmt.Sprintf(queryFmt, blockchain)
		_, err := db.runParamSQLUpdate(query, speed, uuid)
		if err != nil {
			// Silently skip SQLITE_BUSY errors since RPS is non-critical and will be updated again soon
			if strings.Contains(err.Error(), "SQLITE_BUSY") || strings.Contains(err.Error(), "database is locked") {
				return
			}
			core.LogDebug("Could not update the indexer job speed in the database: " + err.Error())
		}
	}
}
func (db *SQLite) IndexerAddPost(txHash string, blockchain string, fromAddress string, toAddress string, parentTxHash string, amount uint64, timestamp uint64, data string, blockNumber uint64) {
	queryFmt := "INSERT INTO onchain_%s_post (txHash, blockchain, fromAddress, toAddress, parentTxHash, amount, timestamp, data, blockNumber) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddress, toAddress, parentTxHash, amount, timestamp, data, blockNumber)
	if err != nil {
		core.LogDebug("Could not add a post from the indexer into the database: " + err.Error())
	}
}
func (db *SQLite) IndexerResetJobs(blockchain string) {
	queryFmt := "UPDATE %s_indexer_jobs SET status = 'pending', headBlock = 0, tailBlock = 0, timestamp = ? WHERE blockchain = ?"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, core.GetTimestamp(), blockchain)
	if err != nil {
		core.LogDebug("Could not reset the indexer in the database: " + err.Error())
	}
	// Wipe onchain data for this blockchain - indexer will refill it
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
func (db *SQLite) OnchainC(txHash string, blockchain string, fromAddr string, parentTxHash string, parentType string, amount uint64, timestamp uint64, data string) {
	queryFmt := "INSERT INTO onchain_%s_comment (txHash, blockchain, fromAddress, parentTxHash, parentType, amount, timestamp, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, parentTxHash, parentType, amount, timestamp, data)
	if err != nil {
		core.LogDebug("Could not tokenize the comment in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainCA(txHash string, blockchain string, fromAddr string, parentTxHash string, parentType string, amount uint64, timestamp uint64, data string, attachments []Attachment) {
	queryFmt := "INSERT INTO onchain_%s_comment (txHash, blockchain, fromAddress, parentTxHash, parentType, amount, timestamp, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	query := fmt.Sprintf(queryFmt, blockchain)
	result, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, parentTxHash, parentType, amount, timestamp, data)
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
		fileTxnQuery := "INSERT INTO file_txn_hash (fileUUID, txHash, blockchain) VALUES (?, ?, ?) ON CONFLICT (fileUUID, txHash, blockchain) DO NOTHING"
		_, err = db.runParamSQLUpdate(fileTxnQuery, fileUUID, txHash, blockchain)
		if err != nil {
			core.LogDebug("Could not link file to transaction: " + err.Error())
		}
	}
}
func (db *SQLite) OnchainR(txHash string, blockchain string, fromAddr string, targetTxHash string, targetType string, reactionType string, timestamp uint64) {
	if reactionType != "like" && reactionType != "dislike" {
		deleteQueryFmt := "DELETE FROM onchain_%s_reaction WHERE fromAddress = ? AND targetTxHash = ? AND blockchain = ? AND reactionType NOT IN ('like', 'dislike')"
		deleteQuery := fmt.Sprintf(deleteQueryFmt, blockchain)
		_, _ = db.runParamSQLUpdate(deleteQuery, fromAddr, targetTxHash, blockchain)
	}
	queryFmt := "INSERT INTO onchain_%s_reaction (txHash, blockchain, fromAddress, targetTxHash, targetType, reactionType, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, targetTxHash, targetType, reactionType, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the reaction in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainP(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string) {
	queryFmt := "INSERT INTO onchain_%s_post (txHash, blockchain, fromAddress, parentTxHash, amount, timestamp, data) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data)
	if err != nil {
		core.LogDebug("Could not tokenize the post in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainPA(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string, attachments []Attachment) {
	queryFmt := "INSERT INTO onchain_%s_post (txHash, blockchain, fromAddress, parentTxHash, amount, timestamp, data) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
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
		fileTxnQuery := "INSERT INTO file_txn_hash (fileUUID, txHash, blockchain) VALUES (?, ?, ?) ON CONFLICT (fileUUID, txHash, blockchain) DO NOTHING"
		_, err = db.runParamSQLUpdate(fileTxnQuery, fileUUID, txHash, blockchain)
		if err != nil {
			core.LogDebug("Could not link file to transaction: " + err.Error())
		}
	}
}
func (db *SQLite) OnchainMN(blockchain string, address string, name string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, name, nameTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET name = excluded.name, nameTimestamp = excluded.nameTimestamp WHERE excluded.nameTimestamp > nameTimestamp"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, name, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMA(blockchain string, address string, avatar string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, avatar, avatarTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET avatar = excluded.avatar, avatarTimestamp = excluded.avatarTimestamp WHERE excluded.avatarTimestamp > avatarTimestamp"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, avatar, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMB(blockchain string, address string, banner string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, banner, bannerTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET banner = excluded.banner, bannerTimestamp = excluded.bannerTimestamp WHERE excluded.bannerTimestamp > bannerTimestamp"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, banner, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMV(blockchain string, address string, vertical string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, vertical, verticalTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET vertical = excluded.vertical, verticalTimestamp = excluded.verticalTimestamp WHERE excluded.verticalTimestamp > verticalTimestamp"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, vertical, int64(timestamp))
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainML(blockchain string, address string, location string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, location, locationTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET location = excluded.location, locationTimestamp = excluded.locationTimestamp WHERE excluded.locationTimestamp > locationTimestamp"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, location, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMW(blockchain string, address string, website string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, website, websiteTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET website = excluded.website, websiteTimestamp = excluded.websiteTimestamp WHERE excluded.websiteTimestamp > websiteTimestamp"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, website, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMD(blockchain string, address string, description string, timestamp uint64) {
	queryFmt := "INSERT INTO onchain_%s_meta (blockchain, address, description, descriptionTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET description = excluded.description, descriptionTimestamp = excluded.descriptionTimestamp WHERE excluded.descriptionTimestamp > descriptionTimestamp"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err := db.runParamSQLUpdate(query, blockchain, address, description, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainF(txHash string, blockchain string, followerAddress string, followerBlockchain string, followeeAddress string, followeeBlockchain string, timestamp uint64) {
	followQueryFmt := "SELECT 1 FROM onchain_%s_follow WHERE followerAddress = ? AND followerBlockchain = ? AND followeeAddress = ? AND followeeBlockchain = ? LIMIT 1"
	followQuery := fmt.Sprintf(followQueryFmt, blockchain)
	rows, err := db.runParamSQLSelect(followQuery, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain)
	if err != nil {
		core.LogDebug("Could not check if the follow already exists in the database: " + err.Error())
		return
	}
	followExists := rows.Next() // if rows.Next() returns true, then at least 1 row exists, indicating the relationship already exists
	rows.Close()
	if followExists {
		// Don't add a new follower database entry if the follower relationship already exists (prevent follower count fraud)
		return
	}
	queryFmt := "INSERT INTO onchain_%s_follow (txHash, blockchain, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err = db.runParamSQLUpdate(query, txHash, blockchain, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the follow in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainFU(txHash string, blockchain string, followerAddress string, followerBlockchain string, followeeAddress string, followeeBlockchain string, timestamp uint64) {
	// Check if the follow relationship exists before attempting to remove it
	followQueryFmt := "SELECT 1 FROM onchain_%s_follow WHERE followerAddress = ? AND followerBlockchain = ? AND followeeAddress = ? AND followeeBlockchain = ? LIMIT 1"
	followQuery := fmt.Sprintf(followQueryFmt, blockchain)
	rows, err := db.runParamSQLSelect(followQuery, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain)
	if err != nil {
		core.LogDebug("Could not check if the follow exists in the database: " + err.Error())
		return
	}
	followExists := rows.Next() // if rows.Next() returns true, then at least 1 row exists, indicating the relationship exists
	rows.Close()
	if !followExists {
		// Don't process the unfollow if the follow relationship doesn't exist (drop the transaction)
		core.LogDebug("Unfollow transaction dropped: follow relationship does not exist")
		return
	}
	// Remove the follow relationship from the database
	queryFmt := "DELETE FROM onchain_%s_follow WHERE followerAddress = ? AND followerBlockchain = ? AND followeeAddress = ? AND followeeBlockchain = ?"
	query := fmt.Sprintf(queryFmt, blockchain)
	_, err = db.runParamSQLUpdate(query, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain)
	if err != nil {
		core.LogDebug("Could not remove the follow relationship from the database: " + err.Error())
	}
}
func (db *SQLite) OnchainDeleteExpired(blockchain string, cutoffTimestamp uint64) {
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
func (db *SQLite) GetComments(parentTxHash string, blockchain string, limit int, offset int) []map[string]interface{} {
	var comments []map[string]interface{}
	queryFmt := `SELECT c.txHash, c.blockchain, c.fromAddress, c.parentTxHash, c.parentType, c.timestamp, c.data,
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
		var txHash, bc, fromAddress, pTxHash, parentType, data, author, avatarSrc string
		var timestamp, likeCount, dislikeCount, replyCount int64
		err := rows.Scan(&txHash, &bc, &fromAddress, &pTxHash, &parentType, &timestamp, &data, &author, &avatarSrc, &likeCount, &dislikeCount, &replyCount)
		if err != nil {
			core.LogDebug("Could not scan comment row: " + err.Error())
			continue
		}
		comment := map[string]interface{}{
			"txHash":       txHash,
			"blockchain":   bc,
			"address":      fromAddress,
			"parentTxHash": pTxHash,
			"parentType":   parentType,
			"timestamp":    timestamp,
			"payload":      data,
			"author":       author,
			"avatarSrc":    avatarSrc,
			"likeCount":    likeCount,
			"dislikeCount": dislikeCount,
			"replyCount":   replyCount,
		}
		comments = append(comments, comment)
	}
	return comments
}
func (db *SQLite) GetCommentCount(targetTxHash string, blockchain string) int64 {
	queryFmt := "SELECT COUNT(*) FROM onchain_%s_comment WHERE parentTxHash = ? AND blockchain = ?"
	query := fmt.Sprintf(queryFmt, blockchain)
	rows, err := db.runParamSQLSelect(query, targetTxHash, blockchain)
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

// --- Reaction Functions --- //
func (db *SQLite) GetReactionCounts(targetTxHash string, blockchain string) map[string]interface{} {
	result := map[string]interface{}{
		"likes":    int64(0),
		"dislikes": int64(0),
		"emoji":    map[string]int64{},
	}
	queryFmt := `SELECT reactionType, COUNT(*) as count FROM (
		SELECT fromAddress, reactionType, MAX(timestamp) as maxTs
		FROM onchain_%s_reaction
		WHERE targetTxHash = ? AND blockchain = ?
		GROUP BY fromAddress
	) GROUP BY reactionType`
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
func (db *SQLite) GetUserReaction(targetTxHash string, blockchain string, fromAddress string) string {
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

// --- Post Get Functions --- //
func (db *SQLite) GetPost(txHash string, blockchain string) map[string]interface{} {
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
func (db *SQLite) GetPostAttachments(txHash string, blockchain string) [][]interface{} {
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
func (db *SQLite) GetFollowersFeed(followerAddress string, followerBlockchain string, limit int, offset int) []map[string]interface{} {
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
		// Get attachments for this post
		sqlQuery := "SELECT f.mimeType, f.size, f.fileUrl, f.fileName FROM files f INNER JOIN file_txn_hash fth ON f.fileUUID = fth.fileUUID WHERE fth.txHash = ? AND fth.blockchain = ?"
		rowsAttachments, err := db.runParamSQLSelect(sqlQuery, txHash, blockchain)
		if err != nil {
			core.LogDebug("Could not get attachments for post: " + err.Error()) // No bail because we can still return the text of the post
		} else if rowsAttachments != nil {
			for rowsAttachments.Next() {
				var mimeType string
				var size uint64
				var fileUrl string
				var fileName string
				err := rowsAttachments.Scan(&mimeType, &size, &fileUrl, &fileName)
				if err != nil {
					core.LogDebug("Could parse rows for post attachment: " + err.Error())
					break // bail rowsAttachments for loop
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
func (db *SQLite) NotificationInsert(uid string, message string) {
	_, err := db.runParamSQLUpdate("INSERT INTO notifications (uid, message, timestamp) VALUES (?, ?, ?)", uid, message, core.GetTimestamp())
	if err != nil {
		core.LogDebug("Could not insert notification: " + err.Error())
	}
}
func (db *SQLite) NotificationDismiss(uid string) {
	_, err := db.runParamSQLUpdate("DELETE FROM notifications WHERE uid = ?", uid)
	if err != nil {
		core.LogDebug("Could not dismiss notification: " + err.Error())
	}
}
func (db *SQLite) NotificationGetActive() []map[string]string {
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

// --- Snapshot Functions --- //
func (db *SQLite) exportSnapshots(exportDir string, blockchain string, headBlock uint64, tailBlock uint64) error { // Exports a compressed snapshot file with all data
	if db.database == nil {
		return core.LogDebugReturn("Database connection not initialized")
	}
	currentTime := core.GetTimestamp()
	exportPath := filepath.Join(exportDir, blockchain+"-snapshot-complete.db.gz")
	metadataPath := filepath.Join(exportDir, blockchain+"-snapshot-complete.json")
	core.LogDebug("Exporting SQLite Snapshot (all blockchain data) to: " + exportPath)
	tables := []string{
		"file_txn_hash",
		"files",
	}
	for _, _blockchain := range core.ValidNetworks {
		tables = append(tables, "onchain_"+_blockchain+"_post")
		tables = append(tables, "onchain_"+_blockchain+"_comment")
		tables = append(tables, "onchain_"+_blockchain+"_reaction")
		tables = append(tables, "onchain_"+_blockchain+"_meta")
		tables = append(tables, "onchain_"+_blockchain+"_block")
		tables = append(tables, "onchain_"+_blockchain+"_follow")
		tables = append(tables, _blockchain+"_indexer_jobs")
	}
	var buffer bytes.Buffer
	metaData := map[string]interface{}{
		"timestamp":  currentTime,
		"version":    "1.0",
		"tables":     tables,
		"head_block": headBlock,
		"tail_block": tailBlock,
	}
	metaJSON, err := json.MarshalIndent(metaData, "", "  ")
	if err != nil {
		return core.LogDebugReturn("Could not serialize metadata: " + err.Error())
	}
	err = os.WriteFile(metadataPath, metaJSON, 0644)
	if err != nil {
		return core.LogDebugReturn("Could not write metadata file: " + err.Error())
	}
	exportFile, err := os.Create(exportPath)
	if err != nil {
		return core.LogDebugReturn("Could not create export file: " + err.Error())
	}
	defer exportFile.Close()
	gzWriter, err := gzip.NewWriterLevel(exportFile, gzip.BestCompression)
	if err != nil {
		return core.LogDebugReturn("Could not create gzip writer: " + err.Error())
	}
	defer gzWriter.Close()
	for _, table := range tables {
		core.LogDebug("Exporting table: " + table)
		rows, err := db.runParamSQLSelect("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table)
		if err != nil {
			return core.LogDebugReturn("Could not get table schema: " + err.Error())
		}
		var createStatement string
		if rows.Next() {
			err = rows.Scan(&createStatement)
			if err != nil {
				rows.Close()
				return core.LogDebugReturn("Could not read table schema: " + err.Error())
			}
		}
		rows.Close()
		if createStatement == "" {
			return core.LogDebugReturn("Table not found: " + table)
		}
		buffer.Reset()
		err = binary.Write(&buffer, binary.LittleEndian, uint32(len(createStatement)))
		if err != nil {
			return core.LogDebugReturn("Could not write schema length: " + err.Error())
		}
		_, err = buffer.WriteString(createStatement)
		if err != nil {
			return core.LogDebugReturn("Could not write schema: " + err.Error())
		}
		dataRows, err := db.runParamSQLSelect("SELECT * FROM " + sanitizeSQLiteTableName(table))
		if err != nil {
			return core.LogDebugReturn("Could not get table data: " + err.Error())
		}
		columns, err := dataRows.Columns()
		if err != nil {
			dataRows.Close()
			return core.LogDebugReturn("Could not get column information: " + err.Error())
		}
		err = binary.Write(&buffer, binary.LittleEndian, uint32(len(columns)))
		if err != nil {
			dataRows.Close()
			return core.LogDebugReturn("Could not write column count: " + err.Error())
		}
		for _, column := range columns {
			err = binary.Write(&buffer, binary.LittleEndian, uint32(len(column)))
			if err != nil {
				dataRows.Close()
				return core.LogDebugReturn("Could not write column name length: " + err.Error())
			}
			_, err = buffer.WriteString(column)
			if err != nil {
				dataRows.Close()
				return core.LogDebugReturn("Could not write column name: " + err.Error())
			}
		}
		rowCount := 0
		for dataRows.Next() {
			rowCount++
		}
		err = dataRows.Err()
		if err != nil {
			dataRows.Close()
			return core.LogDebugReturn("Could not read rows: " + err.Error())
		}
		dataRows.Close()
		err = binary.Write(&buffer, binary.LittleEndian, uint32(rowCount))
		if err != nil {
			return core.LogDebugReturn("Could not write row count: " + err.Error())
		}
		if rowCount == 0 {
			_, err = gzWriter.Write(buffer.Bytes())
			if err != nil {
				return core.LogDebugReturn("Could not write table buffer: " + err.Error())
			}
			continue
		}
		dataRows, err = db.runParamSQLSelect("SELECT * FROM " + sanitizeSQLiteTableName(table))
		if err != nil {
			return core.LogDebugReturn("Could not get table data (second pass): " + err.Error())
		}
		_, err = gzWriter.Write(buffer.Bytes())
		if err != nil {
			dataRows.Close()
			return core.LogDebugReturn("Could not write table header: " + err.Error())
		}
		buffer.Reset()
		rowBuffer := bytes.NewBuffer(nil)
		rowsProcessed := 0
		values := make([]interface{}, len(columns))
		valuePointers := make([]interface{}, len(columns))
		for i := range values {
			valuePointers[i] = &values[i]
		}
		for dataRows.Next() {
			err = dataRows.Scan(valuePointers...)
			if err != nil {
				dataRows.Close()
				return core.LogDebugReturn("Could not scan row: " + err.Error())
			}
			rowBuffer.Reset()
			for _, value := range values {
				if value == nil {
					err = rowBuffer.WriteByte(0)
					if err != nil {
						dataRows.Close()
						return core.LogDebugReturn("Could not write NULL indicator: " + err.Error())
					}
				} else {
					switch v := value.(type) {
					case int64:
						rowBuffer.WriteByte(1) // Type indicator for int64
						binary.Write(rowBuffer, binary.LittleEndian, v)
					case float64:
						rowBuffer.WriteByte(2) // Type indicator for float64
						binary.Write(rowBuffer, binary.LittleEndian, v)
					case []byte:
						rowBuffer.WriteByte(3) // Type indicator for []byte
						binary.Write(rowBuffer, binary.LittleEndian, uint32(len(v)))
						rowBuffer.Write(v)
					case string:
						rowBuffer.WriteByte(4) // Type indicator for string
						binary.Write(rowBuffer, binary.LittleEndian, uint32(len(v)))
						rowBuffer.WriteString(v)
					case time.Time:
						rowBuffer.WriteByte(5) // Type indicator for time.Time
						binary.Write(rowBuffer, binary.LittleEndian, v.Unix())
					default:
						// For any other type, convert to string
						str := fmt.Sprintf("%v", v)
						rowBuffer.WriteByte(4) // Type indicator for string
						binary.Write(rowBuffer, binary.LittleEndian, uint32(len(str)))
						rowBuffer.WriteString(str)
					}
				}
			}
			rowData := rowBuffer.Bytes()
			err = binary.Write(&buffer, binary.LittleEndian, uint32(len(rowData)))
			if err != nil {
				dataRows.Close()
				return core.LogDebugReturn("Could not write row length: " + err.Error())
			}
			_, err = buffer.Write(rowData)
			if err != nil {
				dataRows.Close()
				return core.LogDebugReturn("Could not write row data: " + err.Error())
			}
			rowsProcessed++
			if buffer.Len() > 1024*1024 || rowsProcessed%1000 == 0 {
				_, err = gzWriter.Write(buffer.Bytes())
				if err != nil {
					dataRows.Close()
					return core.LogDebugReturn("Could not write batch of rows to buffer: " + err.Error())
				}
				buffer.Reset()
			}
			if rowsProcessed%10000 == 0 {
				core.LogDebug(fmt.Sprintf("Exported %d/%d rows from table %s", rowsProcessed, rowCount, table))
			}
		}
		err = dataRows.Err()
		if err != nil {
			dataRows.Close()
			return core.LogDebugReturn("Could not read rows (second pass): " + err.Error())
		}
		dataRows.Close()
		if buffer.Len() > 0 {
			_, err = gzWriter.Write(buffer.Bytes())
			if err != nil {
				return core.LogDebugReturn("Could not write remaining rows to buffer: " + err.Error())
			}
		}
		core.LogDebug(fmt.Sprintf("Exported %d rows from table %s", rowsProcessed, table))
	}
	err = gzWriter.Close()
	if err != nil {
		return core.LogDebugReturn("Could not close gzip writer: " + err.Error())
	}
	core.LogInfo("SQLite Snapshot (all data) Exported Successfully To: " + exportPath)
	return nil
}

// --- Wallet Functions --- //
func (db *SQLite) WalletStore(publicKey string, blockchain string, address string, encryptedPrivateKey string, isDefault bool) error {
	isDefaultInt := 0
	if isDefault {
		isDefaultInt = 1
		// If this is being set as default, unset any existing default for this blockchain
		_, err := db.runParamSQLUpdate("UPDATE wallets SET isDefault = 0 WHERE blockchain = ?", blockchain)
		if err != nil {
			return core.LogDebugReturn("Could not unset existing default wallet: " + err.Error())
		}
	}
	query := "INSERT INTO wallets (publicKey, blockchain, address, encryptedPrivateKey, isDefault) VALUES (?, ?, ?, ?, ?) ON CONFLICT (publicKey, blockchain) DO UPDATE SET address = excluded.address, encryptedPrivateKey = excluded.encryptedPrivateKey, isDefault = excluded.isDefault"
	_, err := db.runParamSQLUpdate(query, publicKey, blockchain, address, encryptedPrivateKey, isDefaultInt)
	if err != nil {
		return core.LogDebugReturn("Could not store wallet: " + err.Error())
	}
	return nil
}
func (db *SQLite) WalletGet(publicKey string, blockchain string) (map[string]interface{}, error) {
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
func (db *SQLite) WalletGetDefault(blockchain string) (map[string]interface{}, error) {
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
func (db *SQLite) WalletGetPrivateKey(publicKey string, blockchain string) (string, error) {
	rows, err := db.runParamSQLSelect("SELECT encryptedPrivateKey FROM wallets WHERE publicKey = ? AND blockchain = ?", publicKey, blockchain)
	if err != nil {
		return "", core.LogDebugReturn("Could not get wallet private key: " + err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var encPrivKey string
		err = rows.Scan(&encPrivKey)
		if err != nil {
			return "", core.LogDebugReturn("Could not scan private key row: " + err.Error())
		}
		return encPrivKey, nil
	}
	return "", core.LogDebugReturn("Wallet not found")
}
func (db *SQLite) WalletSetDefault(publicKey string, blockchain string) error {
	// First unset any existing default for this blockchain
	_, err := db.runParamSQLUpdate("UPDATE wallets SET isDefault = 0 WHERE blockchain = ?", blockchain)
	if err != nil {
		return core.LogDebugReturn("Could not unset existing default wallet: " + err.Error())
	}
	// Then set the new default
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
func (db *SQLite) WalletGetAll() ([]map[string]interface{}, error) {
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
