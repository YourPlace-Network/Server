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

const (
	burnAddressETH      = "0x0000000000000000000000000000000000000000"
	burnAddressShortETH = "0x0"
	schemaVersion       = 1
)

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
	// Check and run migrations
	db.checkAndRunMigrations()
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
	// Tables schema map
	tables := map[string]string{
		"meta":          "CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT)",
		"settings":      "CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT)",
		"files":         "CREATE TABLE IF NOT EXISTS files (fileUUID TEXT PRIMARY KEY, fileHash TEXT, mimeType TEXT, fileName TEXT, size INTEGER, addedDate INTEGER, cid TEXT, fileURL TEXT, source TEXT)",
		"file_txn_hash": "CREATE TABLE IF NOT EXISTS file_txn_hash (fileUUID TEXT, txHash TEXT, blockchain TEXT, PRIMARY KEY (fileUUID, txHash, blockchain))",
		"indexer_jobs":  "CREATE TABLE IF NOT EXISTS indexer_jobs (uuid TEXT PRIMARY KEY, blockchain TEXT, headBlock INTEGER, status TEXT, tailBlock INTEGER, timestamp INTEGER, rps INTEGER DEFAULT 0)",
		"auth_nonce":    "CREATE TABLE IF NOT EXISTS auth_nonce (nonce TEXT PRIMARY KEY, status TEXT, timestamp INTEGER)",
		"auth_expired":  "CREATE TABLE IF NOT EXISTS auth_expired (uuid TEXT PRIMARY KEY, status TEXT)",
		"login_nonce":   "CREATE TABLE IF NOT EXISTS login_nonce (nonce TEXT PRIMARY KEY, domain TEXT, expiration INTEGER, nonceHash TEXT)",
		"onchain_post":  "CREATE TABLE IF NOT EXISTS onchain_post (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', parentTxHash TEXT DEFAULT '', amount REAL DEFAULT 0, timestamp INTEGER DEFAULT 0, data TEXT DEFAULT '', PRIMARY KEY(txHash, blockchain))",
		"onchain_meta": "CREATE TABLE IF NOT EXISTS onchain_meta (blockchain TEXT, address TEXT, name TEXT DEFAULT '', avatar TEXT DEFAULT '', description TEXT DEFAULT '', location TEXT DEFAULT '', banner TEXT DEFAULT '', website TEXT DEFAULT '', birthdate INTEGER DEFAULT NULL, server TEXT DEFAULT '', " +
			"blockchainTimestamp INTEGER DEFAULT 0, addressTimestamp INTEGER DEFAULT 0, nameTimestamp INTEGER DEFAULT 0, avatarTimestamp INTEGER DEFAULT 0, descriptionTimestamp INTEGER DEFAULT 0, locationTimestamp INTEGER DEFAULT 0, bannerTimestamp INTEGER DEFAULT 0, websiteTimestamp INTEGER DEFAULT 0, birthdateTimestamp INTEGER DEFAULT 0, serverTimestamp INTEGER DEFAULT 0, PRIMARY KEY(blockchain, address))",
		"onchain_block":  "CREATE TABLE IF NOT EXISTS onchain_block (txHash TEXT, blockchain TEXT, blockerAddress TEXT, blockerBlockchain TEXT, blockeeAddress TEXT, blockeeAddress TEXT, key TEXT, value TEXT, timestamp INTEGER DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"onchain_follow": "CREATE TABLE IF NOT EXISTS onchain_follow (txHash TEXT, blockchain TEXT, followerAddress TEXT, followerBlockchain TEXT, followeeAddress TEXT, followeeBlockchain TEXT, timestamp INTEGER DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"csrf_tokens":    "CREATE TABLE IF NOT EXISTS csrf_tokens (token TEXT PRIMARY KEY, expiration INTEGER)",
		"notifications":  "CREATE TABLE IF NOT EXISTS notifications (uid TEXT PRIMARY KEY, message TEXT, timestamp INTEGER DEFAULT 0)",
		"wallets":        "CREATE TABLE IF NOT EXISTS wallets (publicKey TEXT, blockchain TEXT, address TEXT, encryptedPrivateKey TEXT, isDefault INTEGER DEFAULT 0, PRIMARY KEY (publicKey, blockchain))",
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
func (db *SQLite) checkAndRunMigrations() {
	dbVersion := db.getSchemaVersion()
	if dbVersion > schemaVersion {
		core.LogFatal(fmt.Sprintf("Database schema version (%d) is ahead of binary schema version (%d). Please upgrade the binary.", dbVersion, schemaVersion))
		return
	}
	if dbVersion < schemaVersion {
		core.LogDebug(fmt.Sprintf("Running database migrations from version %d to %d", dbVersion, schemaVersion))
		for version := dbVersion + 1; version <= schemaVersion; version++ {
			core.LogDebug(fmt.Sprintf("Running migration to version %d", version))
			switch version {
			case 1:
				// Migration 1: Add blockchain column to file_txn_hash if missing
			}
			db.setSchemaVersion(version)
		}
		core.LogDebug("Database migrations completed successfully")
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
		"onchain_post",
		"onchain_meta",
		"onchain_block",
		"onchain_follow",
		"indexer_jobs",
		"file_txn_hash",
		"files",
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
		if noQuoteTableName == "indexer_jobs" {
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
		if noQuoteTableName == "indexer_jobs" {
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
func SanitizeSQLiteDatabase(path string) error {
	// This function resets certain elements in the YourPlace database snapshot to get it back to a clean and updated state. Useful for "catch-up" jobs

	// Resolve the user data directory without creating import cycles with the host package name
	userDir, err := os.UserHomeDir()
	if err != nil {
		return core.LogDebugReturn("Could not get home directory: " + err.Error())
	}
	dataDir := userDir + string(os.PathSeparator) + "YourPlace" + string(os.PathSeparator)
	// Connecting to the database file to sanitize
	if path == dataDir+"yourplace.db" {
		return core.LogDebugReturn("Cannot sanitize the main database")
	}
	database, err := sql.Open("sqlite", path)
	if err != nil || database == nil {
		return core.LogDebugReturn("Could not open sqlite db to sanitize it")
	}
	// Scrub it clean
	queries := []string{"TRUNCATE TABLE IF EXISTS auth_expired",
		"TRUNCATE TABLE IF EXISTS auth_nonce",
		"TRUNCATE TABLE IF EXISTS login_nonce",
		"TRUNCATE TABLE IF EXISTS files",
		"TRUNCATE TABLE IF EXISTS meta",
		"TRUNCATE TABLE IF EXISTS settings",
	}
	for _, query := range queries {
		_, err = database.Exec(query)
		if err != nil {
			return core.LogDebugReturn("Could not sanitize sqlite db: " + err.Error())
		}
	}
	return nil
}
func truncateBurnAddress(address string) string {
	if strings.ToLower(address) == burnAddressETH {
		return burnAddressShortETH
	}
	return address
}
func expandBurnAddress(address string) string {
	if strings.ToLower(address) == burnAddressShortETH {
		return burnAddressETH
	}
	return address
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
	rows, err := db.runParamSQLSelect("SELECT name FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
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
	rows, err := db.runParamSQLSelect("SELECT avatar FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
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
	rows, err := db.runParamSQLSelect("SELECT banner FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
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
	rows, err := db.runParamSQLSelect("SELECT description FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
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
	rows, err := db.runParamSQLSelect("SELECT location FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
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
	rows, err := db.runParamSQLSelect("SELECT website FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
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
func (db *SQLite) ProfileGetBirthDate(address string, blockchain string) *int64 {
	rows, err := db.runParamSQLSelect("SELECT birthdate FROM onchain_meta WHERE address = LOWER(?) AND blockchain = ?", address, blockchain)
	if err != nil {
		core.LogDebug("Could not get profile birthdate from database: " + err.Error())
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
			core.LogDebug("Could not parse database rows for profile birthdate: " + err.Error())
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
	rowsmeta, err := db.runParamSQLSelect("SELECT COALESCE(MIN(CASE WHEN blockchainTimestamp > 0 THEN blockchainTimestamp WHEN addressTimestamp > 0 THEN addressTimestamp WHEN nameTimestamp > 0 THEN nameTimestamp WHEN avatarTimestamp > 0 THEN avatarTimestamp WHEN descriptionTimestamp > 0 THEN descriptionTimestamp WHEN locationTimestamp > 0 THEN locationTimestamp WHEN bannerTimestamp > 0 THEN bannerTimestamp WHEN websiteTimestamp > 0 THEN websiteTimestamp WHEN birthdateTimestamp > 0 THEN birthdateTimestamp WHEN serverTimestamp > 0 THEN serverTimestamp ELSE 0 END), 0) AS min_timestamp FROM onchain_meta WHERE blockchain = ? AND address = LOWER(?)", blockchain, address)
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
	rowsposts, err := db.runParamSQLSelect("SELECT timestamp FROM onchain_post WHERE fromAddress = LOWER(?) AND blockchain = ?", address, blockchain)
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
	rowsPosts, err := db.runParamSQLSelect("SELECT txHash, COALESCE(parentTxHash, '') as parentTxHash, timestamp, data FROM onchain_post WHERE fromAddress = LOWER (?) AND blockchain = ? AND data IS NOT NULL ORDER BY timestamp DESC", address, blockchain)
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
		post := map[string]interface{}{
			"resultType": "profile post",
			"txHash":     txHash,
			"parent":     parent,
			"timestamp":  timestamp,
			"payload":    payload,
			"blockchain": blockchain,
			"address":    address,
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
	rows, err := db.runParamSQLSelect("SELECT COUNT(*) FROM onchain_follow WHERE followeeAddress = ? AND followeeBlockchain = ?", address, blockchain)
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
	rows, err := db.runParamSQLSelect("SELECT COUNT(*) FROM onchain_follow WHERE followerAddress = ? AND followerBlockchain = ?", address, blockchain)
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
	rows, err := db.runParamSQLSelect("SELECT COUNT(*) FROM onchain_follow WHERE followeeAddress = ? AND followeeBlockchain = ? AND followerAddress = ? AND followerBlockchain = ?", followeeAddress, followeeBlockchain, followerAddress, followerBlockchain)
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
	search := "%" + query + "%"
	rows, err := db.runParamSQLSelect("SELECT txHash, COALESCE(parentTxHash, '') as parentHash, timestamp, data, fromAddress, blockchain FROM onchain_post WHERE LOWER (data) LIKE LOWER (?)", search)
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
	return posts
}
func (db *SQLite) SearchGetProfiles(query string) []map[string]interface{} {
	var profiles []map[string]interface{}
	search := "%" + query + "%"
	rows, err := db.runParamSQLSelect("SELECT address, blockchain FROM onchain_meta WHERE address LIKE ? OR name LIKE ?", search, search)
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
	query := "INSERT INTO indexer_jobs (uuid, blockchain, headBlock, status, tailBlock, timestamp, rps) VALUES (?, ?, 0, 'pending', 0, ?, 0) ON CONFLICT (uuid) DO UPDATE SET status = excluded.status, tailBlock = excluded.tailBlock, timestamp = excluded.timestamp"
	core.LogDebug("IndexerCreateJob(): " + query)
	_, err := db.runParamSQLUpdate(query, uuid, blockchain, timestamp)
	if err != nil {
		core.LogDebug("Could not create indexer job in the database: " + err.Error())
	}
}
func (db *SQLite) IndexerGetJobUUID(blockchain string) string {
	rows, err := db.runParamSQLSelect("SELECT uuid FROM indexer_jobs WHERE blockchain = ?", blockchain)
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
	rows, err := db.runParamSQLSelect("SELECT status FROM indexer_jobs WHERE uuid = ?", uuid)
	if err != nil {
		core.LogDebug("Could not get the indexer job status from the database: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			core.LogDebug("Could not get the rows for the indexer job status from the database: " + err.Error())
			return ""
		}
		return value
	}
	return ""
}
func (db *SQLite) IndexerGetHeadBlock(uuid string) uint64 {
	rows, err := db.runParamSQLSelect("SELECT headBlock FROM indexer_jobs WHERE uuid = ?", uuid)
	if err != nil {
		core.LogDebug("Could not get the indexer head block from the database: " + err.Error())
		return 0
	}
	defer rows.Close()
	for rows.Next() {
		var value uint64
		err = rows.Scan(&value)
		if err != nil {
			core.LogDebug("Could not get the rows for the indexer head block from the database: " + err.Error())
			return 0
		}
		return value
	}
	return 0
}
func (db *SQLite) IndexerGetTailBlock(uuid string) uint64 {
	rows, err := db.runParamSQLSelect("SELECT tailBlock FROM indexer_jobs WHERE uuid = ?", uuid)
	if err != nil {
		core.LogDebug("Could not get the indexer tail block from the database: " + err.Error())
		return 0
	}
	defer rows.Close()
	for rows.Next() {
		var value uint64
		err = rows.Scan(&value)
		if err != nil {
			core.LogDebug("Could not get the rows for the indexer tail block from the database: " + err.Error())
			return 0
		}
		return value
	}
	return 0
}
func (db *SQLite) IndexerGetRunningJobsUUIDs() []string {
	rows, err := db.getRows("SELECT uuid FROM indexer_jobs WHERE status = 'running'")
	if err != nil {
		core.LogDebug("Could not find the running indexer job UUIDs from the database: " + err.Error())
		return []string{}
	}
	defer rows.Close()
	var uuids []string
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			core.LogDebug("Could not find the rows for the running indexer job UUIDs from the database: " + err.Error())
			return []string{}
		}
		uuids = append(uuids, value)
	}
	return uuids
}
func (db *SQLite) IndexerUpdateJobStatus(uuid string, status string) {
	timestamp := core.GetTimestamp()
	_, err := db.runParamSQLUpdate("UPDATE indexer_jobs SET status = ?, timestamp = ? WHERE uuid = ?", status, timestamp, uuid)
	db.runSQL("PRAGMA wal_checkpoint(FULL)")
	if err != nil {
		core.LogDebug("Could not update the indexer job status in the database: " + err.Error())
	}
}
func (db *SQLite) IndexerUpdateHeadBlock(uuid string, headBlock uint64) {
	_, err := db.runParamSQLUpdate("UPDATE indexer_jobs SET headBlock = ?, timestamp = ? WHERE uuid = ?", headBlock, core.GetTimestamp(), uuid)
	if err != nil {
		core.LogDebug("Could not update the indexer head block in the database: " + err.Error())
	}
}
func (db *SQLite) IndexerUpdateTailBlock(uuid string, tailBlock uint64) {
	_, err := db.runParamSQLUpdate("UPDATE indexer_jobs SET tailBlock = ?, timestamp = ? WHERE uuid = ?", tailBlock, core.GetTimestamp(), uuid)
	if err != nil {
		core.LogDebug("Could not update the indexer tail block in the database: " + err.Error())
	}
}
func (db *SQLite) IndexerUpdateJobSpeed(uuid string, speed uint64) {
	_, err := db.runParamSQLUpdate("UPDATE indexer_jobs SET rps = ? WHERE uuid = ?", speed, uuid)
	if err != nil {
		// Silently skip SQLITE_BUSY errors since RPS is non-critical and will be updated again soon
		if strings.Contains(err.Error(), "SQLITE_BUSY") || strings.Contains(err.Error(), "database is locked") {
			return
		}
		core.LogDebug("Could not update the indexer job speed in the database: " + err.Error())
	}
}
func (db *SQLite) IndexerAddPost(txHash string, blockchain string, fromAddress string, toAddress string, parentTxHash string, amount uint64, timestamp uint64, data string, blockNumber uint64) {
	query := "INSERT INTO onchain_post (txHash, blockchain, fromAddress, toAddress, parentTxHash, amount, timestamp, data, blockNumber) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddress, toAddress, parentTxHash, amount, timestamp, data, blockNumber)
	if err != nil {
		core.LogDebug("Could not add a post from the indexer into the database: " + err.Error())
	}
}
func (db *SQLite) IndexerResetJobs(blockchain string) {
	_, err := db.runParamSQLUpdate("UPDATE indexer_jobs SET status = 'pending', headBlock = 0, tailBlock = 0, timestamp = ? WHERE blockchain = ?", core.GetTimestamp(), blockchain)
	if err != nil {
		core.LogDebug("Could not reset the indexer in the database: " + err.Error())
	}
}

// --- Onchain Tokenized --- //
func (db *SQLite) OnchainP(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string) {
	query := "INSERT INTO onchain_post (txHash, blockchain, fromAddress, parentTxHash, amount, timestamp, data) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, parentTxHash, amount, timestamp, data)
	if err != nil {
		core.LogDebug("Could not tokenize the post in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainPA(txHash string, blockchain string, fromAddr string, parentTxHash string, amount uint64, timestamp uint64, data string, attachments []Attachment) {
	query := "INSERT INTO onchain_post (txHash, blockchain, fromAddress, parentTxHash, amount, timestamp, data) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
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
	query := "INSERT INTO onchain_meta (blockchain, address, name, nameTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET name = excluded.name, nameTimestamp = excluded.nameTimestamp WHERE excluded.nameTimestamp > nameTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, name, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMA(blockchain string, address string, avatar string, timestamp uint64) {
	query := "INSERT INTO onchain_meta (blockchain, address, avatar, avatarTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET avatar = excluded.avatar, avatarTimestamp = excluded.avatarTimestamp WHERE excluded.avatarTimestamp > avatarTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, avatar, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMB(blockchain string, address string, banner string, timestamp uint64) {
	query := "INSERT INTO onchain_meta (blockchain, address, banner, bannerTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET banner = excluded.banner, bannerTimestamp = excluded.bannerTimestamp WHERE excluded.bannerTimestamp > bannerTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, banner, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMBD(blockchain string, address string, birthdate uint64, timestamp uint64) {
	query := "INSERT INTO onchain_meta (blockchain, address, birthdate, birthdateTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET birthdate = excluded.birthdate, birthdateTimestamp = excluded.birthdateTimestamp WHERE excluded.birthdateTimestamp > birthdateTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, birthdate, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainML(blockchain string, address string, location string, timestamp uint64) {
	query := "INSERT INTO onchain_meta (blockchain, address, location, locationTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET location = excluded.location, locationTimestamp = excluded.locationTimestamp WHERE excluded.locationTimestamp > locationTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, location, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMW(blockchain string, address string, website string, timestamp uint64) {
	query := "INSERT INTO onchain_meta (blockchain, address, website, websiteTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET website = excluded.website, websiteTimestamp = excluded.websiteTimestamp WHERE excluded.websiteTimestamp > websiteTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, website, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainMD(blockchain string, address string, description string, timestamp uint64) {
	query := "INSERT INTO onchain_meta (blockchain, address, description, descriptionTimestamp) VALUES (?, ?, ?, ?) ON CONFLICT (blockchain, address) DO UPDATE SET description = excluded.description, descriptionTimestamp = excluded.descriptionTimestamp WHERE excluded.descriptionTimestamp > descriptionTimestamp"
	_, err := db.runParamSQLUpdate(query, blockchain, address, description, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the meta in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainF(txHash string, blockchain string, followerAddress string, followerBlockchain string, followeeAddress string, followeeBlockchain string, timestamp uint64) {
	followQuery := "SELECT 1 FROM onchain_follow WHERE followerAddress = ? AND followerBlockchain = ? AND followeeAddress = ? AND followeeBlockchain = ? LIMIT 1"
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
	query := "INSERT INTO onchain_follow (txHash, blockchain, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	_, err = db.runParamSQLUpdate(query, txHash, blockchain, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain, timestamp)
	if err != nil {
		core.LogDebug("Could not tokenize the follow in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainFU(txHash string, blockchain string, followerAddress string, followerBlockchain string, followeeAddress string, followeeBlockchain string, timestamp uint64) {
	// Check if the follow relationship exists before attempting to remove it
	followQuery := "SELECT 1 FROM onchain_follow WHERE followerAddress = ? AND followerBlockchain = ? AND followeeAddress = ? AND followeeBlockchain = ? LIMIT 1"
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
	query := "DELETE FROM onchain_follow WHERE followerAddress = ? AND followerBlockchain = ? AND followeeAddress = ? AND followeeBlockchain = ?"
	_, err = db.runParamSQLUpdate(query, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain)
	if err != nil {
		core.LogDebug("Could not remove the follow relationship from the database: " + err.Error())
	}
}

// --- Followers Feed --- //
func (db *SQLite) GetFollowersFeed(followerAddress string, followerBlockchain string, limit int) []map[string]interface{} {
	var posts []map[string]interface{}
	query := `SELECT p.txHash, COALESCE(p.parentTxHash, '') as parentTxHash, p.timestamp, p.data, p.fromAddress, p.blockchain 
			  FROM onchain_post p 
			  INNER JOIN onchain_follow f ON p.fromAddress = f.followeeAddress AND p.blockchain = f.followeeBlockchain 
			  WHERE f.followerAddress = LOWER(?) AND f.followerBlockchain = ? AND p.data IS NOT NULL 
			  ORDER BY p.timestamp DESC 
			  LIMIT ?`

	rows, err := db.runParamSQLSelect(query, followerAddress, followerBlockchain, limit)
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

		post := map[string]interface{}{
			"resultType": "post",
			"txHash":     txHash,
			"parentHash": parentTxHash,
			"timestamp":  timestamp,
			"payload":    payload,
			"blockchain": blockchain,
			"address":    address,
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

// --- Snapshot Service Functions --- //
func (db *SQLite) exportSnapshots(exportDir string, blockchain string, headBlock uint64, tailBlock uint64) error { // Exports multiple compressed snapshot files for different post history lengths
	if db.database == nil {
		return core.LogDebugReturn("Database connection not initialized")
	}
	ageThresholds := []int{30, 60, 90, 180, 240, 365, 0}
	currentTime := core.GetTimestamp()
	for _, ageDays := range ageThresholds {
		var exportPath string
		var metadataPath string
		var cutoffTimestamp uint64
		if ageDays > 0 {
			cutoffTimestamp = currentTime - uint64(ageDays*24*60*60)
		}
		if ageDays == 0 {
			exportPath = filepath.Join(exportDir, blockchain+"-snapshot-complete.db.gz")
			metadataPath = filepath.Join(exportDir, blockchain+"-snapshot-complete.json")
			core.LogDebug("Exporting SQLite Snapshot (all data) to: " + exportPath)
		} else {
			exportPath = filepath.Join(exportDir, fmt.Sprintf("%s-snapshot-%dd.db.gz", blockchain, ageDays))
			metadataPath = filepath.Join(exportDir, fmt.Sprintf("%s-snapshot-%dd.json", blockchain, ageDays))
			core.LogDebug(fmt.Sprintf("Exporting SQLite Snapshot (%d days) to: %s", ageDays, exportPath))
		}
		tables := []string{
			"onchain_post",
			"onchain_meta",
			"onchain_block",
			"onchain_follow",
			"indexer_jobs",
			"file_txn_hash",
			"files",
		}
		var buffer bytes.Buffer
		metaData := map[string]interface{}{
			"timestamp":  currentTime,
			"version":    "1.0",
			"tables":     tables,
			"age_days":   ageDays,
			"age_cutoff": cutoffTimestamp,
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
			var dataRows *sql.Rows
			if table == "onchain_post" && ageDays > 0 {
				dataRows, err = db.runParamSQLSelect(`
					SELECT txHash, blockchain, fromAddress, toAddress, parentTxHash, amount, timestamp,
					CASE WHEN timestamp >= ? THEN data ELSE '' END as data
					FROM onchain_post`, cutoffTimestamp)
			} else {
				dataRows, err = db.runParamSQLSelect("SELECT * FROM " + sanitizeSQLiteTableName(table))
			}
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
			if table == "onchain_post" && ageDays > 0 {
				dataRows, err = db.runParamSQLSelect(`
					SELECT txHash, blockchain, fromAddress, toAddress, parentTxHash, amount, timestamp,
					CASE WHEN timestamp >= ? THEN data ELSE NULL END as data
					FROM onchain_post`, cutoffTimestamp)
			} else {
				dataRows, err = db.runParamSQLSelect("SELECT * FROM " + sanitizeSQLiteTableName(table))
			}
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
		if ageDays == 0 {
			core.LogInfo("SQLite Snapshot (all data) Exported Successfully To: " + exportPath)
		} else {
			core.LogInfo(fmt.Sprintf("SQLite Snapshot (%d days) Exported Successfully To: %s", ageDays, exportPath))
		}
	}
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
