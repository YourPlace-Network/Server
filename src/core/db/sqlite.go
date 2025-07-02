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
	_ "github.com/glebarez/go-sqlite"
	"github.com/google/uuid"
	"io"
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
func (db *SQLite) rowCount(query string) (int, error) {
	rows, err := db.runParamSQLSelect(query)
	if err != nil {
		return 0, core.LogErrorReturn("Could not get row count: " + err.Error())
	}
	defer rows.Close()
	rowCount := 0
	for rows.Next() {
		rowCount++
	}
	err = rows.Err()
	if err != nil {
		return 0, core.LogErrorReturn("Could not get row count: " + err.Error())
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
	// Tables schema map
	tables := map[string]string{
		"meta":          "CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT)",
		"settings":      "CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT)",
		"files":         "CREATE TABLE IF NOT EXISTS files (fileUUID TEXT PRIMARY KEY, fileHash TEXT, mimeType TEXT, fileName TEXT, size INTEGER, addedDate INTEGER, cid TEXT, fileURL TEXT, source TEXT)",
		"file_txn_hash": "CREATE TABLE IF NOT EXISTS file_txn_hash (fileUUID TEXT, txHash TEXT)",
		"postsBackfill": "CREATE TABLE IF NOT EXISTS postsBackfill (uuid TEXT PRIMARY KEY, blockchain TEXT, headBlock INTEGER, status TEXT, tailBlock INTEGER, timestamp INTEGER)",
		"authNonce":     "CREATE TABLE IF NOT EXISTS authNonce (nonce TEXT PRIMARY KEY, status TEXT, timestamp INTEGER)",
		"authExpired":   "CREATE TABLE IF NOT EXISTS authExpired (uuid TEXT PRIMARY KEY, status TEXT)",
		"loginNonce":    "CREATE TABLE IF NOT EXISTS loginNonce (nonce TEXT PRIMARY KEY, domain TEXT, expiration INTEGER, nonceHash TEXT)",
		"onchain_post":  "CREATE TABLE IF NOT EXISTS onchain_post (txHash TEXT, blockchain TEXT, fromAddress TEXT DEFAULT '', toAddress TEXT DEFAULT '', parentTxHash TEXT DEFAULT '', amount REAL DEFAULT 0, timestamp INTEGER DEFAULT 0, data TEXT DEFAULT '', PRIMARY KEY(txHash, blockchain))",
		"onchain_meta": "CREATE TABLE IF NOT EXISTS onchain_meta (blockchain TEXT, address TEXT, name TEXT DEFAULT '', avatar TEXT DEFAULT '', description TEXT DEFAULT '', location TEXT DEFAULT '', banner TEXT DEFAULT '', website TEXT DEFAULT '', birthdate INTEGER DEFAULT NULL, server TEXT DEFAULT '', " +
			"blockchainTimestamp INTEGER DEFAULT 0, addressTimestamp INTEGER DEFAULT 0, nameTimestamp INTEGER DEFAULT 0, avatarTimestamp INTEGER DEFAULT 0, descriptionTimestamp INTEGER DEFAULT 0, locationTimestamp INTEGER DEFAULT 0, bannerTimestamp INTEGER DEFAULT 0, websiteTimestamp INTEGER DEFAULT 0, birthdateTimestamp INTEGER DEFAULT 0, serverTimestamp INTEGER DEFAULT 0, PRIMARY KEY(blockchain, address))",
		"onchain_block":        "CREATE TABLE IF NOT EXISTS onchain_block (txHash TEXT, blockchain TEXT, address TEXT, key TEXT, value TEXT, timestamp INTEGER DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"onchain_follow":       "CREATE TABLE IF NOT EXISTS onchain_follow (txHash TEXT, blockchain TEXT, followerAddress TEXT, followerBlockchain TEXT, followeeAddress TEXT, followeeBlockchain TEXT, timestamp INTEGER DEFAULT 0, PRIMARY KEY (txHash, blockchain))",
		"marketplace_listings": "CREATE TABLE IF NOT EXISTS marketplace_listings (id TEXT PRIMARY KEY, sellerAddress TEXT, sellerBlockchain TEXT, title TEXT, description TEXT, price TEXT, priceWei TEXT, currency TEXT, listingType TEXT DEFAULT 'fixed', status TEXT DEFAULT 'active', auctionEndTime INTEGER, reservePrice TEXT, reservePriceWei TEXT, txHash TEXT, createdAt INTEGER, updatedAt INTEGER)",
		"marketplace_offers":   "CREATE TABLE IF NOT EXISTS marketplace_offers (id TEXT PRIMARY KEY, listingId TEXT, offerByAddress TEXT, offerByBlockchain TEXT, offerPrice TEXT, offerPriceWei TEXT, message TEXT, status TEXT DEFAULT 'pending', txHash TEXT, createdAt INTEGER, acceptedAt INTEGER, FOREIGN KEY(listingId) REFERENCES marketplace_listings(id))",
		"marketplace_payments": "CREATE TABLE IF NOT EXISTS marketplace_payments (id TEXT PRIMARY KEY, offerId TEXT, offerAcceptTxHash TEXT, fromAddress TEXT, fromBlockchain TEXT, toAddress TEXT, toBlockchain TEXT, price TEXT, priceWei TEXT, txHash TEXT, status TEXT DEFAULT 'pending', createdAt INTEGER, FOREIGN KEY(offerId) REFERENCES marketplace_offers(id))",
		"marketplace_receipts": "CREATE TABLE IF NOT EXISTS marketplace_receipts (id TEXT PRIMARY KEY, paymentId TEXT, receiptByAddress TEXT, receiptByBlockchain TEXT, txHash TEXT, createdAt INTEGER, FOREIGN KEY(paymentId) REFERENCES marketplace_payments(id))",
		"marketplace_bids":     "CREATE TABLE IF NOT EXISTS marketplace_bids (id TEXT PRIMARY KEY, listingId TEXT, bidderAddress TEXT, bidderBlockchain TEXT, bidAmount TEXT, bidAmountWei TEXT, bidMessage TEXT, status TEXT DEFAULT 'active', txHash TEXT, createdAt INTEGER, outbidAt INTEGER, FOREIGN KEY(listingId) REFERENCES marketplace_listings(id))",
		"marketplace_auctions": "CREATE TABLE IF NOT EXISTS marketplace_auctions (id TEXT PRIMARY KEY, listingId TEXT, startTime INTEGER, endTime INTEGER, currentHighBid TEXT, currentHighBidWei TEXT, currentHighBidder TEXT, currentHighBidderBlockchain TEXT, bidCount INTEGER DEFAULT 0, status TEXT DEFAULT 'active', txHash TEXT, createdAt INTEGER, endedAt INTEGER, FOREIGN KEY(listingId) REFERENCES marketplace_listings(id))",
		"csrf_tokens":          "CREATE TABLE IF NOT EXISTS csrf_tokens (token TEXT PRIMARY KEY, expiration INTEGER)",
	}
	for _, createStatement := range tables {
		err := db.execWithRetry(ctx, createStatement, 3)
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
func (db *SQLite) ExportSnapshot(exportPath string) error {
	if db.database == nil {
		return core.LogErrorReturn("Database connection not initialized")
	}
	core.LogDebug("Exporting SQLite Snapshot to: " + exportPath)
	// Tables to export
	tables := []string{
		"onchain_post",
		"onchain_meta",
		"onchain_block",
		"onchain_follow",
		"postsBackfill",
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
		return core.LogErrorReturn("Could not create export file: " + err.Error())
	}
	defer exportFile.Close()
	// Use a gzip writer for compression
	gzWriter, err := gzip.NewWriterLevel(exportFile, gzip.BestCompression)
	if err != nil {
		return core.LogErrorReturn("Could not create gzip writer: " + err.Error())
	}
	defer gzWriter.Close()
	// First write the metadata
	metaJSON, err := json.Marshal(metaData)
	if err != nil {
		return core.LogErrorReturn("Could not serialize metadata: " + err.Error())
	}
	// Write metadata length as a binary header (4 bytes)
	binary.Write(gzWriter, binary.LittleEndian, uint32(len(metaJSON)))
	// Write metadata
	_, err = gzWriter.Write(metaJSON)
	if err != nil {
		return core.LogErrorReturn("Could not write metadata: " + err.Error())
	}
	// Export each table directly to the compressed stream
	for _, table := range tables {
		core.LogDebug("Exporting table: " + table)
		// Get table schema - use parameterized query to prevent SQL injection
		rows, err := db.runParamSQLSelect("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table)
		if err != nil {
			return core.LogErrorReturn("Could not get table schema: " + err.Error())
		}
		var createStatement string
		if rows.Next() {
			err = rows.Scan(&createStatement)
			if err != nil {
				rows.Close()
				return core.LogErrorReturn("Could not read table schema: " + err.Error())
			}
		}
		rows.Close()
		if createStatement == "" {
			return core.LogErrorReturn("Table not found: " + table)
		}
		// Reset buffer
		buffer.Reset()
		// Write schema to buffer
		err = binary.Write(&buffer, binary.LittleEndian, uint32(len(createStatement)))
		if err != nil {
			return core.LogErrorReturn("Could not write schema length: " + err.Error())
		}
		_, err = buffer.WriteString(createStatement)
		if err != nil {
			return core.LogErrorReturn("Could not write schema: " + err.Error())
		}
		// Get all data from the table - table name comes from predefined list so safe
		dataRows, err := db.runParamSQLSelect("SELECT * FROM " + sanitizeSQLiteTableName(table))
		if err != nil {
			return core.LogErrorReturn("Could not get table data: " + err.Error())
		}
		// Get column information
		columns, err := dataRows.Columns()
		if err != nil {
			dataRows.Close()
			return core.LogErrorReturn("Could not get column information: " + err.Error())
		}
		// Serialize column count and names
		err = binary.Write(&buffer, binary.LittleEndian, uint32(len(columns)))
		if err != nil {
			dataRows.Close()
			return core.LogErrorReturn("Could not write column count: " + err.Error())
		}
		for _, column := range columns {
			err = binary.Write(&buffer, binary.LittleEndian, uint32(len(column)))
			if err != nil {
				dataRows.Close()
				return core.LogErrorReturn("Could not write column name length: " + err.Error())
			}
			_, err = buffer.WriteString(column)
			if err != nil {
				dataRows.Close()
				return core.LogErrorReturn("Could not write column name: " + err.Error())
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
			return core.LogErrorReturn("Could not read rows: " + err.Error())
		}
		dataRows.Close()
		// Write row count
		err = binary.Write(&buffer, binary.LittleEndian, uint32(rowCount))
		if err != nil {
			return core.LogErrorReturn("Could not write row count: " + err.Error())
		}
		// If there are no rows, continue to next table
		if rowCount == 0 {
			// Write this table's buffer to compressed file
			_, err = gzWriter.Write(buffer.Bytes())
			if err != nil {
				return core.LogErrorReturn("Could not write table buffer: " + err.Error())
			}
			continue
		}
		// Get data again for second pass - table name comes from predefined list so safe
		dataRows, err = db.runParamSQLSelect("SELECT * FROM " + sanitizeSQLiteTableName(table))
		if err != nil {
			return core.LogErrorReturn("Could not get table data (second pass): " + err.Error())
		}
		// Write this table's header to compressed file now
		_, err = gzWriter.Write(buffer.Bytes())
		if err != nil {
			dataRows.Close()
			return core.LogErrorReturn("Could not write table header: " + err.Error())
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
				return core.LogErrorReturn("Could not scan row: " + err.Error())
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
						return core.LogErrorReturn("Could not write NULL indicator: " + err.Error())
					}
				} else {
					// Determine the type and serialize accordingly
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
			// Write row length and then row data
			rowData := rowBuffer.Bytes()
			err = binary.Write(&buffer, binary.LittleEndian, uint32(len(rowData)))
			if err != nil {
				dataRows.Close()
				return core.LogErrorReturn("Could not write row length: " + err.Error())
			}
			_, err = buffer.Write(rowData)
			if err != nil {
				dataRows.Close()
				return core.LogErrorReturn("Could not write row data: " + err.Error())
			}
			rowsProcessed++
			// Flush to gzip writer ever 1000 rows to avoid memory buildup
			if buffer.Len() > 1024*1024 || rowsProcessed%1000 == 0 {
				_, err = gzWriter.Write(buffer.Bytes())
				if err != nil {
					dataRows.Close()
					return core.LogErrorReturn("Could not write batch of rows to buffer: " + err.Error())
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
			return core.LogErrorReturn("Could not read rows (second pass): " + err.Error())
		}
		dataRows.Close()
		// Write any remaining data
		if buffer.Len() > 0 {
			_, err = gzWriter.Write(buffer.Bytes())
			if err != nil {
				return core.LogErrorReturn("Could not write remaining rows to buffer: " + err.Error())
			}
		}
		core.LogDebug(fmt.Sprintf("Exported %d rows from table %s", rowsProcessed, table))
	}
	// Close the gzip writer to flush any remaining data
	err = gzWriter.Close()
	if err != nil {
		return core.LogErrorReturn("Could not close gzip writer: " + err.Error())
	}
	core.LogInfo("SQLite Snapshot Exported Successfully To: " + exportPath)
	return nil
}
func (db *SQLite) ImportSnapshot(importPath string) error {
	if db.database == nil {
		return core.LogErrorReturn("Database connection not initialized")
	}
	// Open the import file
	importFile, err := os.Open(importPath)
	if err != nil {
		return core.LogErrorReturn("Could not open import file: " + err.Error())
	}
	defer importFile.Close()
	// Create a gzip reader
	gzReader, err := gzip.NewReader(importFile)
	if err != nil {
		return core.LogErrorReturn("Could not create gzip reader: " + err.Error())
	}
	defer gzReader.Close()
	// Read metadata length
	var metaLength uint32
	err = binary.Read(gzReader, binary.LittleEndian, &metaLength)
	if err != nil {
		return core.LogErrorReturn("Could not read metadata length: " + err.Error())
	}
	// Read metadata
	metaBytes := make([]byte, metaLength)
	_, err = io.ReadFull(gzReader, metaBytes)
	if err != nil {
		return core.LogErrorReturn("Could not read metadata: " + err.Error())
	}
	var metadata map[string]interface{}
	err = json.Unmarshal(metaBytes, &metadata)
	if err != nil {
		return core.LogErrorReturn("Could not parse metadata: " + err.Error())
	}
	// Get tables
	tablesInterface, ok := metadata["tables"]
	if !ok {
		return core.LogErrorReturn("Metadata missing tables")
	}
	tablesArray, ok := tablesInterface.([]interface{})
	if !ok {
		return core.LogErrorReturn("Metadata tables not an array")
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
			return core.LogErrorReturn("Could not read schema length: " + err.Error())
		}
		// Read schema
		schemaBytes := make([]byte, schemaLength)
		_, err = io.ReadFull(gzReader, schemaBytes)
		if err != nil {
			return core.LogErrorReturn("Could not read schema: " + err.Error())
		}
		schema := string(schemaBytes)
		// Extract table name from schema
		tableName := extractTableName(schema)
		if tableName == "" {
			return core.LogErrorReturn("Could not extract table name from schema: " + schema)
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
			return core.LogErrorReturn("Could not read column count: " + err.Error())
		}
		// Read column names
		columns := make([]string, columnCount)
		for i := 0; i < int(columnCount); i++ {
			var nameLength uint32
			err = binary.Read(gzReader, binary.LittleEndian, &nameLength)
			if err != nil {
				return core.LogErrorReturn("Could not read column name length: " + err.Error())
			}
			nameBytes := make([]byte, nameLength)
			_, err = io.ReadFull(gzReader, nameBytes)
			if err != nil {
				return core.LogErrorReturn("Could not read column name: " + err.Error())
			}
			columns[i] = string(nameBytes)
		}
		// Read row count
		var rowCount uint32
		err = binary.Read(gzReader, binary.LittleEndian, &rowCount)
		if err != nil {
			return core.LogErrorReturn("Could not read row count: " + err.Error())
		}
		// If no rows, continue to next table
		if rowCount == 0 {
			core.LogDebug("No rows to import for table: " + tableName)
			continue
		}
		// Start transaction for this table
		tx, err := db.database.Begin()
		if err != nil {
			return core.LogErrorReturn("Could not start transaction: " + err.Error())
		}
		// Prepare insert statement
		placeholders := make([]string, len(columns))
		for i := range columns {
			placeholders[i] = "?"
		}
		var insertSQL string
		if strings.HasPrefix(tableName, "onchain_") || tableName == "postsBackfill" {
			insertSQL = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING", tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))
		} else {
			insertSQL = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName, strings.Join(placeholders, ", "), strings.Join(placeholders, ", "))
		}
		statement, err := tx.Prepare(insertSQL)
		if err != nil {
			tx.Rollback()
			return core.LogErrorReturn("Could not prepare insert statement: " + err.Error())
		}
		// Import each row
		rowsProcessed := 0
		for rowIdx := 0; rowIdx < int(rowCount); rowIdx++ {
			// Read row length
			var rowLength uint32
			err = binary.Read(gzReader, binary.LittleEndian, &rowLength)
			if err != nil {
				statement.Close()
				tx.Rollback()
				return core.LogErrorReturn("Could not read row length: " + err.Error())
			}
			// Read row data
			rowData := make([]byte, rowLength)
			_, err = io.ReadFull(gzReader, rowData)
			if err != nil {
				statement.Close()
				tx.Rollback()
				return core.LogErrorReturn("Could not read row data: " + err.Error())
			}
			// Parse row data
			rowReader := bytes.NewReader(rowData)
			values := make([]interface{}, len(columns))
			for i := range columns {
				// Read type indicator
				typeIndicator, err := rowReader.ReadByte()
				if err != nil {
					statement.Close()
					tx.Rollback()
					return core.LogErrorReturn("Could not read type indicator: " + err.Error())
				}
				// Parse based on type
				switch typeIndicator {
				case 0: // NULL
					values[i] = nil
				case 1: // int64
					var value int64
					binary.Read(rowReader, binary.LittleEndian, &value)
					values[i] = value
				case 2: // float64
					var value float64
					binary.Read(rowReader, binary.LittleEndian, &value)
					values[i] = value
				case 3: // []byte
					var length uint32
					binary.Read(rowReader, binary.LittleEndian, &length)
					bytes := make([]byte, length)
					_, err := io.ReadFull(rowReader, bytes)
					if err != nil {
						statement.Close()
						tx.Rollback()
						return core.LogErrorReturn("Could not read []byte value: " + err.Error())
					}
					values[i] = bytes
				case 4: // string
					var length uint32
					binary.Read(rowReader, binary.LittleEndian, &length)
					bytes := make([]byte, length)
					_, err := io.ReadFull(rowReader, bytes)
					if err != nil {
						statement.Close()
						tx.Rollback()
						return core.LogErrorReturn("Could not read string value: " + err.Error())
					}
					values[i] = string(bytes)
				case 5: // time.Time
					var unixTime int64
					binary.Read(rowReader, binary.LittleEndian, &unixTime)
					values[i] = time.Unix(unixTime, 0)
				default:
					statement.Close()
					tx.Rollback()
					return core.LogErrorReturn("Unknown type indicator: " + string(typeIndicator))
				}
			}
			// Execute insert
			_, err = statement.Exec(values...)
			if err != nil {
				statement.Close()
				tx.Rollback()
				return core.LogErrorReturn("Could not execute insert: " + err.Error())
			}
			rowsProcessed++
			// Log Progress
			if rowsProcessed%10000 == 0 {
				core.LogDebug(fmt.Sprintf("Imported %d/%d rows from table %s", rowsProcessed, rowCount, tableName))
			}
		}
		// Commit transaction
		statement.Close()
		err = tx.Commit()
		if err != nil {
			return core.LogErrorReturn("Could not commit transaction: " + err.Error())
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
		return core.LogErrorReturn("Could not get home directory: " + err.Error())
	}
	dataDir := userDir + string(os.PathSeparator) + "YourPlace" + string(os.PathSeparator)
	// Connecting to the database file to sanitize
	if path == dataDir+"yourplace.db" {
		return core.LogErrorReturn("Cannot sanitize the main database")
	}
	database, err := sql.Open("sqlite", path)
	if err != nil || database == nil {
		return core.LogErrorReturn("Could not open sqlite db: " + err.Error())
	}
	// Scrub it clean
	queries := []string{"TRUNCATE TABLE IF EXISTS authExpired",
		"TRUNCATE TABLE IF EXISTS authNonce",
		"TRUNCATE TABLE IF EXISTS loginNonce",
		"TRUNCATE TABLE IF EXISTS files",
		"TRUNCATE TABLE IF EXISTS meta",
		"TRUNCATE TABLE IF EXISTS settings",
	}
	for _, query := range queries {
		_, err = database.Exec(query)
		if err != nil {
			return core.LogErrorReturn("Could not sanitize sqlite db: " + err.Error())
		}
	}
	return nil
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
func (db *SQLite) SettingsDeleteValue(key string) error {
	_, err := db.runParamSQLUpdate("DELETE FROM settings WHERE key = ?", key)
	if err != nil {
		return core.LogErrorReturn("Could not delete setting: " + err.Error())
	}
	return nil
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
	rowsmeta, err := db.runParamSQLSelect("SELECT COALESCE(MIN(CASE WHEN blockchainTimestamp > 0 THEN blockchainTimestamp WHEN addressTimestamp > 0 THEN addressTimestamp WHEN nameTimestamp > 0 THEN nameTimestamp WHEN avatarTimestamp > 0 THEN avatarTimestamp WHEN descriptionTimestamp > 0 THEN descriptionTimestamp WHEN locationTimestamp > 0 THEN locationTimestamp WHEN bannerTimestamp > 0 THEN bannerTimestamp WHEN websiteTimestamp > 0 THEN websiteTimestamp WHEN birthdateTimestamp > 0 THEN birthdateTimestamp WHEN serverTimestamp > 0 THEN serverTimestamp ELSE 0 END), 0) AS min_timestamp FROM onchain_meta WHERE blockchain = ? AND address = LOWER(?)", blockchain, address)
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
	rowsposts, err := db.runParamSQLSelect("SELECT timestamp FROM onchain_post WHERE fromAddress = LOWER(?) AND blockchain = ?", address, blockchain)
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
	rowsPosts, err := db.runParamSQLSelect("SELECT txHash, COALESCE(parentTxHash, '') as parentTxHash, timestamp, data FROM onchain_post WHERE fromAddress = LOWER (?) AND blockchain = ? ORDER BY timestamp DESC", address, blockchain)
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
		sqlQuery := "SELECT f.mimeType, f.size, f.fileUrl, f.fileName FROM files f INNER JOIN file_txn_hash fth ON f.fileUUID = fth.fileUUID WHERE fth.txHash = ?"
		rowsAttachments, err := db.runParamSQLSelect(sqlQuery, txHash)
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
		core.LogError("Could not get follower count from database: " + err.Error())
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&postCount)
		if err != nil {
			core.LogError("Could not parse database rows for follower count: " + err.Error())
			return nil
		}
	}
	return &postCount
}
func (db *SQLite) ProfileIsFollower(followeeAddress string, followeeBlockchain string, followerAddress string, followerBlockchain string) bool {
	rows, err := db.runParamSQLSelect("SELECT COUNT(*) FROM onchain_follow WHERE followeeAddress = ? AND followeeBlockchain = ? AND followerAddress = ? AND followerBlockchain = ?", followeeAddress, followeeBlockchain, followerAddress, followerBlockchain)
	if err != nil {
		core.LogError("Could not get follower status from database: " + err.Error())
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var count int
		err = rows.Scan(&count)
		if err != nil {
			core.LogError("Could not parse database rows for follower status: " + err.Error())
			return false
		}
		if count > 0 {
			return true
		}
	}
	return false
}

// --- Search Functions --- //
func (db *SQLite) SearchGetPosts(query string) []map[string]interface{} {
	var posts []map[string]interface{}
	search := "%" + query + "%"
	rows, err := db.runParamSQLSelect("SELECT txHash, COALESCE(parentTxHash, '') as parentHash, timestamp, data, fromAddress, blockchain FROM onchain_post WHERE LOWER (data) LIKE LOWER (?)", search)
	if err != nil {
		core.LogError("Could not get searched posts from database: " + err.Error())
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var timestamp uint64
		var txHash, parentHash, payload, blockchain, address string
		var attachments [][]interface{}
		err := rows.Scan(&txHash, &parentHash, &timestamp, &payload, &address, &blockchain)
		if err != nil {
			core.LogError("Could not scan database rows: " + err.Error())
			return nil
		}
		sqlQuery := "SELECT f.mimeType, f.size, f.fileUrl, f.fileName FROM files f INNER JOIN file_txn_hash fth ON f.fileUUID = fth.fileUUID WHERE fth.txHash = ?"
		rowsAttachments, err := db.runParamSQLSelect(sqlQuery, txHash, blockchain)
		if err != nil {
			core.LogError("Could not get attachments for post: " + err.Error()) // No bail because we can still return the text of the post
		}
		defer rowsAttachments.Close()
		for rowsAttachments.Next() {
			var mimeType string
			var size uint64
			var fileURL string
			var fileName string
			err := rowsAttachments.Scan(&mimeType, &size, &fileURL, &fileName)
			if err != nil {
				core.LogError("Could parse rows for post attachment: " + err.Error())
				break // bail rowsAttachments for loop
			}
			attachment := []interface{}{fileURL, mimeType, size, fileName}
			attachments = append(attachments, attachment)
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
		core.LogError("Could not get searched profiles from database: " + err.Error())
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
func (db *SQLite) FileAdd(fileUUID string, fileHash string, mimeType string, fileName string, size int64) {
	query := "INSERT INTO files (fileUUID, fileHash, mimeType, fileName, size, addedDate) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING"
	_, err := db.runParamSQLUpdate(query, fileUUID, fileHash, mimeType, fileName, size, core.GetTimestamp())
	if err != nil {
		core.LogError("Could not add the file to the database: " + err.Error())
	}
}
func (db *SQLite) IPFSAdd(fileUUID string, cid string) {
	fileURL := "ipfs://" + cid
	query := "UPDATE files SET cid = ?, fileURL = ? WHERE fileUUID = ?"
	_, err := db.runParamSQLUpdate(query, cid, fileURL, fileUUID)
	if err != nil {
		core.LogError("Could not add the IPFS CID to the database: " + err.Error())
	}
}
func (db *SQLite) GetFileHashFromUUID(uuid string) string {
	rows, err := db.runParamSQLSelect("SELECT fileHash FROM files WHERE fileUUID = ?", uuid)
	if err != nil {
		core.LogError("Could not get the hash from the UUID: " + err.Error())
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var fileHash string
		err = rows.Scan(&fileHash)
		if err != nil {
			core.LogError("Could not get the hash from the UUID: " + err.Error())
			return ""
		}
		return fileHash
	}
	return ""
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
func (db *SQLite) IndexerAddPost(txHash string, blockchain string, fromAddress string, toAddress string, parentTxHash string, amount uint64, timestamp uint64, data string, blockNumber uint64) {
	query := "INSERT INTO onchain_post (txHash, blockchain, fromAddress, toAddress, parentTxHash, amount, timestamp, data, blockNumber) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddress, toAddress, parentTxHash, amount, timestamp, data, blockNumber)
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
func (db *SQLite) OnchainP(txHash string, blockchain string, fromAddr string, toAddr string, parentTxHash string, amount uint64, timestamp uint64, data string) {
	query := "INSERT INTO onchain_post (txHash, blockchain, fromAddress, toAddress, parentTxHash, amount, timestamp, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, toAddr, parentTxHash, amount, timestamp, data)
	if err != nil {
		core.LogError("Could not tokenize the post in the database: " + err.Error())
	}
}
func (db *SQLite) OnchainPA(txHash string, blockchain string, fromAddr string, toAddr string, parentTxHash string, amount uint64, timestamp uint64, data string, attachments []Attachment) {
	query := "INSERT INTO onchain_post (txHash, blockchain, fromAddress, toAddress, parentTxHash, amount, timestamp, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (txHash, blockchain) DO NOTHING"
	_, err := db.runParamSQLUpdate(query, txHash, blockchain, fromAddr, toAddr, parentTxHash, amount, timestamp, data)
	if err != nil {
		core.LogError("Could not tokenize the post in the database: " + err.Error())
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
				core.LogError("Could not check for existing file: " + err.Error())
				continue
			}
			if rows.Next() {
				err = rows.Scan(&existingFileUUID)
				if err != nil {
					core.LogError("Could not scan existing file UUID: " + err.Error())
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
				core.LogError("Could not insert file record: " + err.Error())
				continue
			}
		}
		fileTxnQuery := "INSERT INTO file_txn_hash (fileUUID, txHash) VALUES (?, ?)"
		_, err = db.runParamSQLUpdate(fileTxnQuery, fileUUID, txHash)
		if err != nil {
			core.LogError("Could not link file to transaction: " + err.Error())
		}
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
func (db *SQLite) OnchainF(txHash string, blockchain string, followerAddress string, followerBlockchain string, followeeAddress string, followeeBlockchain string, timestamp uint64) {
	followQuery := "SELECT 1 FROM onchain_follow WHERE followerAddress = ? AND followerBlockchain = ? AND followeeAddress = ? AND followeeBlockchain = ? LIMIT 1"
	rows, err := db.runParamSQLSelect(followQuery, followerAddress, followerBlockchain, followeeAddress, followeeBlockchain)
	if err != nil {
		core.LogError("Could not check if the follow already exists in the database: " + err.Error())
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
		core.LogError("Could not tokenize the follow in the database: " + err.Error())
	}
}

// --- Marketplace Functions --- //
func (db *SQLite) MarketplaceCreateListing(id string, sellerAddress string, sellerBlockchain string, title string, description string, price string, priceWei string, currency string, txHash string) {
	timestamp := core.GetTimestamp()
	query := "INSERT INTO marketplace_listings (id, sellerAddress, sellerBlockchain, title, description, price, priceWei, currency, txHash, createdAt, updatedAt) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := db.runParamSQLUpdate(query, id, sellerAddress, sellerBlockchain, title, description, price, priceWei, currency, txHash, timestamp, timestamp)
	if err != nil {
		core.LogError("Could not create marketplace listing: " + err.Error())
	}
}
func (db *SQLite) MarketplaceGetListings() []map[string]interface{} {
	query := "SELECT id, sellerAddress, sellerBlockchain, title, description, price, priceWei, currency, status, txHash, createdAt, updatedAt FROM marketplace_listings WHERE status = 'active' ORDER BY createdAt DESC"
	rows, err := db.runParamSQLSelect(query)
	if err != nil {
		core.LogError("Could not get marketplace listings: " + err.Error())
		return nil
	}
	defer rows.Close()
	var listings []map[string]interface{}
	for rows.Next() {
		var id, sellerAddress, sellerBlockchain, title, description, price, priceWei, currency, status, txHash string
		var createdAt, updatedAt int64
		err = rows.Scan(&id, &sellerAddress, &sellerBlockchain, &title, &description, &price, &priceWei, &currency, &status, &txHash, &createdAt, &updatedAt)
		if err != nil {
			core.LogError("Could not scan marketplace listing: " + err.Error())
			continue
		}
		listing := map[string]interface{}{
			"id": id, "sellerAddress": sellerAddress, "sellerBlockchain": sellerBlockchain,
			"title": title, "description": description, "price": price, "priceWei": priceWei,
			"currency": currency, "status": status, "txHash": txHash,
			"createdAt": createdAt, "updatedAt": updatedAt,
		}
		listings = append(listings, listing)
	}
	return listings
}
func (db *SQLite) MarketplaceGetUserListings(sellerAddress string, sellerBlockchain string) []map[string]interface{} {
	query := "SELECT id, sellerAddress, sellerBlockchain, title, description, price, priceWei, currency, status, txHash, createdAt, updatedAt FROM marketplace_listings WHERE sellerAddress = ? AND sellerBlockchain = ? ORDER BY createdAt DESC"
	rows, err := db.runParamSQLSelect(query, sellerAddress, sellerBlockchain)
	if err != nil {
		core.LogError("Could not get user marketplace listings: " + err.Error())
		return nil
	}
	defer rows.Close()
	var listings []map[string]interface{}
	for rows.Next() {
		var id, sellerAddress, sellerBlockchain, title, description, price, priceWei, currency, status, txHash string
		var createdAt, updatedAt int64
		err = rows.Scan(&id, &sellerAddress, &sellerBlockchain, &title, &description, &price, &priceWei, &currency, &status, &txHash, &createdAt, &updatedAt)
		if err != nil {
			core.LogError("Could not scan user marketplace listing: " + err.Error())
			continue
		}
		listing := map[string]interface{}{
			"id": id, "sellerAddress": sellerAddress, "sellerBlockchain": sellerBlockchain,
			"title": title, "description": description, "price": price, "priceWei": priceWei,
			"currency": currency, "status": status, "txHash": txHash,
			"createdAt": createdAt, "updatedAt": updatedAt,
		}
		listings = append(listings, listing)
	}
	return listings
}
func (db *SQLite) MarketplaceGetListing(id string) map[string]interface{} {
	query := "SELECT id, sellerAddress, sellerBlockchain, title, description, price, priceWei, currency, status, txHash, createdAt, updatedAt FROM marketplace_listings WHERE id = ?"
	rows, err := db.runParamSQLSelect(query, id)
	if err != nil {
		core.LogError("Could not get marketplace listing: " + err.Error())
		return nil
	}
	defer rows.Close()
	if rows.Next() {
		var id, sellerAddress, sellerBlockchain, title, description, price, priceWei, currency, status, txHash string
		var createdAt, updatedAt int64
		err = rows.Scan(&id, &sellerAddress, &sellerBlockchain, &title, &description, &price, &priceWei, &currency, &status, &txHash, &createdAt, &updatedAt)
		if err != nil {
			core.LogError("Could not scan marketplace listing: " + err.Error())
			return nil
		}
		return map[string]interface{}{
			"id": id, "sellerAddress": sellerAddress, "sellerBlockchain": sellerBlockchain,
			"title": title, "description": description, "price": price, "priceWei": priceWei,
			"currency": currency, "status": status, "txHash": txHash,
			"createdAt": createdAt, "updatedAt": updatedAt,
		}
	}
	return nil
}
func (db *SQLite) MarketplaceUpdateListingStatus(id string, status string) {
	timestamp := core.GetTimestamp()
	query := "UPDATE marketplace_listings SET status = ?, updatedAt = ? WHERE id = ?"
	_, err := db.runParamSQLUpdate(query, status, timestamp, id)
	if err != nil {
		core.LogError("Could not update marketplace listing status: " + err.Error())
	}
}
func (db *SQLite) MarketplaceCreateTransaction(id string, listingId string, buyerAddress string, buyerBlockchain string, sellerAddress string, sellerBlockchain string, txHash string, price string, priceWei string) {
	timestamp := core.GetTimestamp()
	query := "INSERT INTO marketplace_transactions (id, listingId, buyerAddress, buyerBlockchain, sellerAddress, sellerBlockchain, txHash, price, priceWei, createdAt) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := db.runParamSQLUpdate(query, id, listingId, buyerAddress, buyerBlockchain, sellerAddress, sellerBlockchain, txHash, price, priceWei, timestamp)
	if err != nil {
		core.LogError("Could not create marketplace transaction: " + err.Error())
	}
}
func (db *SQLite) MarketplaceGetTransaction(id string) map[string]interface{} {
	query := "SELECT id, listingId, buyerAddress, buyerBlockchain, sellerAddress, sellerBlockchain, txHash, status, price, priceWei, createdAt, completedAt FROM marketplace_transactions WHERE id = ?"
	rows, err := db.runParamSQLSelect(query, id)
	if err != nil {
		core.LogError("Could not get marketplace transaction: " + err.Error())
		return nil
	}
	defer rows.Close()
	if rows.Next() {
		var id, listingId, buyerAddress, buyerBlockchain, sellerAddress, sellerBlockchain, txHash, status, price, priceWei string
		var createdAt, completedAt sql.NullInt64
		err = rows.Scan(&id, &listingId, &buyerAddress, &buyerBlockchain, &sellerAddress, &sellerBlockchain, &txHash, &status, &price, &priceWei, &createdAt, &completedAt)
		if err != nil {
			core.LogError("Could not scan marketplace transaction: " + err.Error())
			return nil
		}
		result := map[string]interface{}{
			"id": id, "listingId": listingId, "buyerAddress": buyerAddress, "buyerBlockchain": buyerBlockchain,
			"sellerAddress": sellerAddress, "sellerBlockchain": sellerBlockchain, "txHash": txHash,
			"status": status, "price": price, "priceWei": priceWei,
		}
		if createdAt.Valid {
			result["createdAt"] = createdAt.Int64
		}
		if completedAt.Valid {
			result["completedAt"] = completedAt.Int64
		}
		return result
	}
	return nil
}
func (db *SQLite) MarketplaceUpdateTransactionStatus(id string, status string) {
	timestamp := core.GetTimestamp()
	query := "UPDATE marketplace_transactions SET status = ?, completedAt = ? WHERE id = ?"
	_, err := db.runParamSQLUpdate(query, status, timestamp, id)
	if err != nil {
		core.LogError("Could not update marketplace transaction status: " + err.Error())
	}
}
func (db *SQLite) MarketplaceGetUserTransactions(address string, blockchain string) []map[string]interface{} {
	query := "SELECT id, listingId, buyerAddress, buyerBlockchain, sellerAddress, sellerBlockchain, txHash, status, price, priceWei, createdAt, completedAt FROM marketplace_transactions WHERE (buyerAddress = ? AND buyerBlockchain = ?) OR (sellerAddress = ? AND sellerBlockchain = ?) ORDER BY createdAt DESC"
	rows, err := db.runParamSQLSelect(query, address, blockchain, address, blockchain)
	if err != nil {
		core.LogError("Could not get user marketplace transactions: " + err.Error())
		return nil
	}
	defer rows.Close()
	var transactions []map[string]interface{}
	for rows.Next() {
		var id, listingId, buyerAddress, buyerBlockchain, sellerAddress, sellerBlockchain, txHash, status, price, priceWei string
		var createdAt, completedAt sql.NullInt64
		err = rows.Scan(&id, &listingId, &buyerAddress, &buyerBlockchain, &sellerAddress, &sellerBlockchain, &txHash, &status, &price, &priceWei, &createdAt, &completedAt)
		if err != nil {
			core.LogError("Could not scan user marketplace transaction: " + err.Error())
			continue
		}
		transaction := map[string]interface{}{
			"id": id, "listingId": listingId, "buyerAddress": buyerAddress, "buyerBlockchain": buyerBlockchain,
			"sellerAddress": sellerAddress, "sellerBlockchain": sellerBlockchain, "txHash": txHash,
			"status": status, "price": price, "priceWei": priceWei,
		}
		if createdAt.Valid {
			transaction["createdAt"] = createdAt.Int64
		}
		if completedAt.Valid {
			transaction["completedAt"] = completedAt.Int64
		}
		transactions = append(transactions, transaction)
	}
	return transactions
}

// --- Onchain Marketplace Functions --- //
func (db *SQLite) MarketplaceListing(txHash string, blockchain string, fromAddr string, toAddr string, title string, description string, price string, priceSmallUnit string, currencySymbol string, imageUrls []string, timestamp uint64) {
	marketplaceId := uuid.New().String()
	imageUrlsJson, _ := json.Marshal(imageUrls)
	// Check if this transaction already exists with different timestamp
	existingQuery := "SELECT createdAt FROM marketplace_listings WHERE txHash = ? LIMIT 1"
	existingRows, err := db.runParamSQLSelect(existingQuery, txHash)
	if err != nil {
		core.LogError("Could not check for existing marketplace listing: " + err.Error())
		return
	}
	defer existingRows.Close()
	if existingRows.Next() {
		var existingTimestamp uint64
		err = existingRows.Scan(&existingTimestamp)
		if err != nil {
			core.LogError("Could not scan existing listing timestamp: " + err.Error())
			return
		}
		// If existing transaction has earlier timestamp, keep it (first-one-wins by blockchain time)
		if existingTimestamp <= timestamp {
			core.LogInfo("Marketplace listing ignored - earlier timestamp exists: " + txHash)
			return
		}
		// If this transaction has earlier timestamp, replace the existing one
		updateQuery := "UPDATE marketplace_listings SET title = ?, description = ?, price = ?, priceSmallUnit = ?, currency = ?, imageUrls = ?, updatedAt = ?, createdAt = ? WHERE txHash = ?"
		_, err = db.runParamSQLUpdate(updateQuery, title, description, price, priceSmallUnit, currencySymbol, string(imageUrlsJson), timestamp, timestamp, txHash)
		if err != nil {
			core.LogError("Could not update marketplace listing with earlier timestamp: " + err.Error())
		}
		return
	}
	// Insert new listing
	query := "INSERT INTO marketplace_listings (id, sellerAddress, sellerBlockchain, title, description, price, priceSmallUnit, currency, imageUrls, status, txHash, createdAt, updatedAt) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?)"
	_, err = db.runParamSQLUpdate(query, marketplaceId, fromAddr, blockchain, title, description, price, priceSmallUnit, currencySymbol, string(imageUrlsJson), txHash, timestamp, timestamp)
	if err != nil {
		core.LogError("Could not index marketplace listing: " + err.Error())
	} else {
		core.LogInfo("Indexed marketplace listing: " + marketplaceId + " from tx: " + txHash)
	}
}
func (db *SQLite) MarketplaceOffer(txHash string, blockchain string, fromAddr string, toAddr string, listingTxHash string, offerPrice string, offerPriceSmallUnit string, message string, timestamp uint64) {
	// Check if this transaction already exists with different timestamp
	checkQuery := "SELECT createdAt FROM marketplace_offers WHERE id = ? LIMIT 1"
	rows, err := db.runParamSQLSelect(checkQuery, txHash)
	if err != nil {
		core.LogError("Could not check for existing marketplace offer: " + err.Error())
		return
	}
	defer rows.Close()
	if rows.Next() {
		var existingTimestamp uint64
		err = rows.Scan(&existingTimestamp)
		if err != nil {
			core.LogError("Could not scan existing offer timestamp: " + err.Error())
			return
		}
		// If existing transaction has earlier timestamp, keep it (first-one-wins by blockchain time)
		if existingTimestamp <= timestamp {
			core.LogInfo("Marketplace offer ignored - earlier timestamp exists: " + txHash)
			return
		}
		// If this transaction has earlier timestamp, replace the existing one
		updateQuery := "UPDATE marketplace_offers SET listingId = ?, offerByAddress = ?, offerByBlockchain = ?, offerPrice = ?, offerPriceSmallUnit = ?, message = ?, status = 'pending', txHash = ?, createdAt = ? WHERE id = ?"
		_, err = db.runParamSQLUpdate(updateQuery, listingTxHash, fromAddr, blockchain, offerPrice, offerPriceSmallUnit, message, txHash, timestamp, txHash)
		if err != nil {
			core.LogError("Could not update marketplace offer with earlier timestamp: " + err.Error())
		}
		return
	}
	// Insert new offer
	query := "INSERT INTO marketplace_offers (id, listingId, offerByAddress, offerByBlockchain, offerPrice, offerPriceSmallUnit, message, status, txHash, createdAt) VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?)"
	_, err = db.runParamSQLUpdate(query, txHash, listingTxHash, fromAddr, blockchain, offerPrice, offerPriceSmallUnit, message, txHash, timestamp)
	if err != nil {
		core.LogError("Could not index marketplace offer: " + err.Error())
		return
	}
	// Check if this offer is for an auction listing and update high bid if needed
	listingQuery := "SELECT listingType, id FROM marketplace_listings WHERE txHash = ? LIMIT 1"
	listingRows, err := db.runParamSQLSelect(listingQuery, listingTxHash)
	if err != nil {
		core.LogError("Could not check listing type for offer: " + err.Error())
		return
	}
	defer listingRows.Close()
	if listingRows.Next() {
		var listingType, listingId string
		err = listingRows.Scan(&listingType, &listingId)
		if err != nil {
			core.LogError("Could not scan listing type: " + err.Error())
			return
		}
		// If this is an auction listing, update the high bid
		if listingType == "auction" {
			db.MarketplaceUpdateHighBid(listingId, offerPrice, offerPriceSmallUnit, fromAddr, blockchain)
		}
	}
}
func (db *SQLite) MarketplaceOfferAccept(txHash string, blockchain string, fromAddr string, toAddr string, offerTxHash string, timestamp uint64) {
	// Check if offer exists and is still pending
	checkQuery := "SELECT status FROM marketplace_offers WHERE id = ? LIMIT 1"
	rows, err := db.runParamSQLSelect(checkQuery, offerTxHash)
	if err != nil {
		core.LogError("Could not check offer status: " + err.Error())
		return
	}
	defer rows.Close()
	var currentStatus string
	if !rows.Next() {
		core.LogError("Offer not found for acceptance: " + offerTxHash)
		return
	}
	err = rows.Scan(&currentStatus)
	if err != nil {
		core.LogError("Could not scan offer status: " + err.Error())
		return
	}
	if currentStatus != "pending" {
		core.LogWarn("Attempted to accept non-pending offer: " + offerTxHash + " (status: " + currentStatus + ")")
		return
	}
	// Only accept if still pending
	query := "UPDATE marketplace_offers SET status = 'accepted', acceptedAt = ? WHERE id = ? AND status = 'pending'"
	result, err := db.runParamSQLUpdate(query, timestamp, offerTxHash)
	if err != nil {
		core.LogError("Could not accept marketplace offer: " + err.Error())
		return
	}
	// Check if update actually happened
	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		core.LogWarn("Offer acceptance failed - possibly already accepted: " + offerTxHash)
	}
}
func (db *SQLite) MarketplacePayment(txHash string, buyerBlockchain string, fromAddr string, toAddr string, offerAcceptTxHash string, price string, priceSmallUnit string, timestamp uint64) {
	offerQuery := "SELECT id, offerPriceSmallUnit, offerByAddress, offerByBlockchain, listingId FROM marketplace_offers WHERE status = 'accepted' AND acceptedAt <= ? ORDER BY acceptedAt DESC LIMIT 1"
	rows, err := db.runParamSQLSelect(offerQuery, timestamp+60)
	if err != nil {
		core.LogError("Could not find accepted offer for payment: " + err.Error())
		return
	}
	defer rows.Close()
	var offerId, expectedPriceSmallUnit, offerByAddress, offerByBlockchain, listingId string
	if rows.Next() {
		err = rows.Scan(&offerId, &expectedPriceSmallUnit, &offerByAddress, &offerByBlockchain, &listingId)
		if err != nil {
			core.LogError("Could not scan offer details: " + err.Error())
			return
		}
	} else {
		core.LogError("No accepted offer found for payment transaction")
		return
	}
	if priceSmallUnit != expectedPriceSmallUnit {
		core.LogWarn("Payment amount mismatch - Expected: " + expectedPriceSmallUnit + ", Got: " + priceSmallUnit + " for offer: " + offerId)
	}
	if fromAddr != offerByAddress {
		core.LogWarn("Payment from unexpected address - Expected: " + offerByAddress + ", Got: " + fromAddr + " for offer: " + offerId)
	}
	sellerQuery := "SELECT sellerAddress, sellerBlockchain FROM marketplace_listings WHERE id = ? LIMIT 1"
	sellerRows, err := db.runParamSQLSelect(sellerQuery, listingId)
	if err != nil {
		core.LogError("Could not find seller details: " + err.Error())
		return
	}
	defer sellerRows.Close()
	var sellerAddress, sellerBlockchain string
	if sellerRows.Next() {
		err = sellerRows.Scan(&sellerAddress, &sellerBlockchain)
		if err != nil {
			core.LogError("Could not scan seller details: " + err.Error())
			return
		}
	}
	query := "INSERT INTO marketplace_payments (id, offerId, offerAcceptTxHash, fromAddress, fromBlockchain, toAddress, toBlockchain, price, priceSmallUnit, txHash, status, createdAt) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?) ON CONFLICT (id) DO NOTHING"
	_, err = db.runParamSQLUpdate(query, txHash, offerId, offerAcceptTxHash, fromAddr, buyerBlockchain, sellerAddress, sellerBlockchain, price, priceSmallUnit, txHash, timestamp)
	if err != nil {
		core.LogError("Could not index marketplace payment: " + err.Error())
	}
}
func (db *SQLite) MarketplaceReceipt(txHash string, blockchain string, fromAddr string, toAddr string, paymentTxHash string, timestamp uint64) {
	// Get payment ID
	paymentQuery := "SELECT id FROM marketplace_payments WHERE txHash = ? LIMIT 1"
	rows, err := db.runParamSQLSelect(paymentQuery, paymentTxHash)
	if err != nil {
		core.LogError("Could not find payment for receipt: " + err.Error())
		return
	}
	defer rows.Close()
	var paymentId string
	if rows.Next() {
		err = rows.Scan(&paymentId)
		if err != nil {
			core.LogError("Could not scan payment ID: " + err.Error())
			return
		}
	} else {
		core.LogError("No payment found for receipt transaction")
		return
	}
	// Create receipt record
	query := "INSERT INTO marketplace_receipts (id, paymentId, receiptByAddress, receiptByBlockchain, txHash, createdAt) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT (id) DO NOTHING"
	_, err = db.runParamSQLUpdate(query, txHash, paymentId, fromAddr, blockchain, txHash, timestamp)
	if err != nil {
		core.LogError("Could not index marketplace receipt: " + err.Error())
		return
	}
	// Update payment status to completed
	updateQuery := "UPDATE marketplace_payments SET status = 'completed' WHERE id = ?"
	_, err = db.runParamSQLUpdate(updateQuery, paymentId)
	if err != nil {
		core.LogError("Could not complete marketplace payment: " + err.Error())
	}
}

// --- Marketplace Offer Functions --- //
func (db *SQLite) MarketplaceCreateOffer(id string, listingId string, offerByAddress string, offerByBlockchain string, offerPrice string, offerPriceWei string, message string, txHash string) {
	timestamp := core.GetTimestamp()
	query := "INSERT INTO marketplace_offers (id, listingId, offerByAddress, offerByBlockchain, offerPrice, offerPriceWei, message, status, txHash, createdAt) VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?)"
	_, err := db.runParamSQLUpdate(query, id, listingId, offerByAddress, offerByBlockchain, offerPrice, offerPriceWei, message, txHash, timestamp)
	if err != nil {
		core.LogError("Could not create marketplace offer: " + err.Error())
	}
}
func (db *SQLite) MarketplaceGetOffers(listingId string) []map[string]interface{} {
	query := "SELECT id, listingId, offerByAddress, offerByBlockchain, offerPrice, offerPriceWei, message, status, txHash, createdAt, acceptedAt FROM marketplace_offers WHERE listingId = ? ORDER BY createdAt DESC"
	rows, err := db.runParamSQLSelect(query, listingId)
	if err != nil {
		core.LogError("Could not get marketplace offers: " + err.Error())
		return nil
	}
	defer rows.Close()
	var offers []map[string]interface{}
	for rows.Next() {
		var id, listingId, offerByAddress, offerByBlockchain, offerPrice, offerPriceWei, message, status, txHash string
		var createdAt, acceptedAt sql.NullInt64
		err = rows.Scan(&id, &listingId, &offerByAddress, &offerByBlockchain, &offerPrice, &offerPriceWei, &message, &status, &txHash, &createdAt, &acceptedAt)
		if err != nil {
			core.LogError("Could not scan marketplace offer: " + err.Error())
			continue
		}
		offer := map[string]interface{}{
			"id": id, "listingId": listingId, "offerByAddress": offerByAddress, "offerByBlockchain": offerByBlockchain,
			"offerPrice": offerPrice, "offerPriceWei": offerPriceWei, "message": message, "status": status, "txHash": txHash,
		}
		if createdAt.Valid {
			offer["createdAt"] = createdAt.Int64
		}
		if acceptedAt.Valid {
			offer["acceptedAt"] = acceptedAt.Int64
		}
		offers = append(offers, offer)
	}
	return offers
}
func (db *SQLite) MarketplaceGetOffer(id string) map[string]interface{} {
	query := "SELECT id, listingId, offerByAddress, offerByBlockchain, offerPrice, offerPriceWei, message, status, txHash, createdAt, acceptedAt FROM marketplace_offers WHERE id = ?"
	rows, err := db.runParamSQLSelect(query, id)
	if err != nil {
		core.LogError("Could not get marketplace offer: " + err.Error())
		return nil
	}
	defer rows.Close()
	if rows.Next() {
		var id, listingId, offerByAddress, offerByBlockchain, offerPrice, offerPriceWei, message, status, txHash string
		var createdAt, acceptedAt sql.NullInt64
		err = rows.Scan(&id, &listingId, &offerByAddress, &offerByBlockchain, &offerPrice, &offerPriceWei, &message, &status, &txHash, &createdAt, &acceptedAt)
		if err != nil {
			core.LogError("Could not scan marketplace offer: " + err.Error())
			return nil
		}
		result := map[string]interface{}{
			"id": id, "listingId": listingId, "offerByAddress": offerByAddress, "offerByBlockchain": offerByBlockchain,
			"offerPrice": offerPrice, "offerPriceWei": offerPriceWei, "message": message, "status": status, "txHash": txHash,
		}
		if createdAt.Valid {
			result["createdAt"] = createdAt.Int64
		}
		if acceptedAt.Valid {
			result["acceptedAt"] = acceptedAt.Int64
		}
		return result
	}
	return nil
}
func (db *SQLite) MarketplaceAcceptOffer(offerId string, acceptedAt uint64) {
	query := "UPDATE marketplace_offers SET status = 'accepted', acceptedAt = ? WHERE id = ?"
	_, err := db.runParamSQLUpdate(query, acceptedAt, offerId)
	if err != nil {
		core.LogError("Could not accept marketplace offer: " + err.Error())
	}
}
func (db *SQLite) MarketplaceCreatePayment(id string, offerId string, offerAcceptTxHash string, fromAddress string, fromBlockchain string, toAddress string, toBlockchain string, price string, priceWei string, txHash string) {
	timestamp := core.GetTimestamp()
	query := "INSERT INTO marketplace_payments (id, offerId, offerAcceptTxHash, fromAddress, fromBlockchain, toAddress, toBlockchain, price, priceWei, txHash, status, createdAt) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)"
	_, err := db.runParamSQLUpdate(query, id, offerId, offerAcceptTxHash, fromAddress, fromBlockchain, toAddress, toBlockchain, price, priceWei, txHash, timestamp)
	if err != nil {
		core.LogError("Could not create marketplace payment: " + err.Error())
	}
}
func (db *SQLite) MarketplaceCreateReceipt(id string, paymentId string, receiptByAddress string, receiptByBlockchain string, txHash string) {
	timestamp := core.GetTimestamp()
	query := "INSERT INTO marketplace_receipts (id, paymentId, receiptByAddress, receiptByBlockchain, txHash, createdAt) VALUES (?, ?, ?, ?, ?, ?)"
	_, err := db.runParamSQLUpdate(query, id, paymentId, receiptByAddress, receiptByBlockchain, txHash, timestamp)
	if err != nil {
		core.LogError("Could not create marketplace receipt: " + err.Error())
	}
}

// --- Auction Functions (Using Offer Transaction Type) --- //
func (db *SQLite) MarketplaceCreateAuction(id string, listingId string, startTime uint64, endTime uint64, txHash string) {
	timestamp := core.GetTimestamp()
	// Check if this auction already exists with different timestamp
	checkQuery := "SELECT createdAt FROM marketplace_auctions WHERE id = ? LIMIT 1"
	rows, err := db.runParamSQLSelect(checkQuery, id)
	if err != nil {
		core.LogError("Could not check for existing marketplace auction: " + err.Error())
		return
	}
	defer rows.Close()
	if rows.Next() {
		var existingTimestamp uint64
		err = rows.Scan(&existingTimestamp)
		if err != nil {
			core.LogError("Could not scan existing auction timestamp: " + err.Error())
			return
		}
		// If existing transaction has earlier timestamp, keep it (first-one-wins by blockchain time)
		if existingTimestamp <= timestamp {
			core.LogInfo("Marketplace auction ignored - earlier timestamp exists: " + id)
			return
		}
		// If this transaction has earlier timestamp, replace the existing one
		updateQuery := "UPDATE marketplace_auctions SET listingId = ?, startTime = ?, endTime = ?, status = 'active', txHash = ?, createdAt = ? WHERE id = ?"
		_, err = db.runParamSQLUpdate(updateQuery, listingId, startTime, endTime, txHash, timestamp, id)
		if err != nil {
			core.LogError("Could not update marketplace auction with earlier timestamp: " + err.Error())
		}
		return
	}
	// Insert new auction
	query := "INSERT INTO marketplace_auctions (id, listingId, startTime, endTime, status, txHash, createdAt) VALUES (?, ?, ?, ?, 'active', ?, ?)"
	_, err = db.runParamSQLUpdate(query, id, listingId, startTime, endTime, txHash, timestamp)
	if err != nil {
		core.LogError("Could not create marketplace auction: " + err.Error())
	}
}
func (db *SQLite) MarketplaceGetAuction(listingId string) map[string]interface{} {
	query := "SELECT id, listingId, startTime, endTime, currentHighBid, currentHighBidWei, currentHighBidder, currentHighBidderBlockchain, bidCount, status, txHash, createdAt, endedAt FROM marketplace_auctions WHERE listingId = ?"
	rows, err := db.runParamSQLSelect(query, listingId)
	if err != nil {
		core.LogError("Could not get marketplace auction: " + err.Error())
		return nil
	}
	defer rows.Close()
	if rows.Next() {
		var id, listingId, currentHighBid, currentHighBidWei, currentHighBidder, currentHighBidderBlockchain, status, txHash string
		var startTime, endTime, bidCount, createdAt, endedAt sql.NullInt64
		err = rows.Scan(&id, &listingId, &startTime, &endTime, &currentHighBid, &currentHighBidWei, &currentHighBidder, &currentHighBidderBlockchain, &bidCount, &status, &txHash, &createdAt, &endedAt)
		if err != nil {
			core.LogError("Could not scan marketplace auction: " + err.Error())
			return nil
		}
		result := map[string]interface{}{
			"id": id, "listingId": listingId, "currentHighBid": currentHighBid, "currentHighBidWei": currentHighBidWei,
			"currentHighBidder": currentHighBidder, "currentHighBidderBlockchain": currentHighBidderBlockchain,
			"status": status, "txHash": txHash,
		}
		if startTime.Valid {
			result["startTime"] = startTime.Int64
		}
		if endTime.Valid {
			result["endTime"] = endTime.Int64
		}
		if bidCount.Valid {
			result["bidCount"] = bidCount.Int64
		}
		if createdAt.Valid {
			result["createdAt"] = createdAt.Int64
		}
		if endedAt.Valid {
			result["endedAt"] = endedAt.Int64
		}
		return result
	}
	return nil
}
func (db *SQLite) MarketplaceUpdateHighBid(listingId string, bidAmount string, bidAmountWei string, bidderAddress string, bidderBlockchain string) {
	query := "UPDATE marketplace_auctions SET currentHighBid = ?, currentHighBidWei = ?, currentHighBidder = ?, currentHighBidderBlockchain = ?, bidCount = bidCount + 1 WHERE listingId = ?"
	_, err := db.runParamSQLUpdate(query, bidAmount, bidAmountWei, bidderAddress, bidderBlockchain, listingId)
	if err != nil {
		core.LogError("Could not update marketplace auction high bid: " + err.Error())
	}
}

// --- Onchain Auction Functions --- //
func (db *SQLite) OnchainMAL(txHash string, blockchain string, fromAddr string, toAddr string, title string, description string, startPrice string, startPriceWei string, reservePrice string, reservePriceWei string, currency string, duration uint64, timestamp uint64) {
	endTime := timestamp + duration
	marketplaceId := uuid.New().String()
	// Check if this auction listing already exists with different timestamp
	existingQuery := "SELECT createdAt FROM marketplace_listings WHERE txHash = ? LIMIT 1"
	existingRows, err := db.runParamSQLSelect(existingQuery, txHash)
	if err != nil {
		core.LogError("Could not check for existing marketplace auction listing: " + err.Error())
		return
	}
	defer existingRows.Close()
	if existingRows.Next() {
		var existingTimestamp uint64
		err = existingRows.Scan(&existingTimestamp)
		if err != nil {
			core.LogError("Could not scan existing auction listing timestamp: " + err.Error())
			return
		}
		// If existing transaction has earlier timestamp, keep it (first-one-wins by blockchain time)
		if existingTimestamp <= timestamp {
			core.LogInfo("Marketplace auction listing ignored - earlier timestamp exists: " + txHash)
			return
		}
		// If this transaction has earlier timestamp, replace the existing one
		updateQuery := "UPDATE marketplace_listings SET title = ?, description = ?, price = ?, priceWei = ?, currency = ?, listingType = 'auction', reservePrice = ?, reservePriceWei = ?, auctionEndTime = ?, updatedAt = ?, createdAt = ? WHERE txHash = ?"
		_, err = db.runParamSQLUpdate(updateQuery, title, description, startPrice, startPriceWei, currency, reservePrice, reservePriceWei, endTime, timestamp, timestamp, txHash)
		if err != nil {
			core.LogError("Could not update marketplace auction listing with earlier timestamp: " + err.Error())
		}
		return
	}
	// Create auction listing
	listingQuery := "INSERT INTO marketplace_listings (id, sellerAddress, sellerBlockchain, title, description, price, priceWei, currency, listingType, status, reservePrice, reservePriceWei, auctionEndTime, txHash, createdAt, updatedAt) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'auction', 'active', ?, ?, ?, ?, ?, ?)"
	_, err = db.runParamSQLUpdate(listingQuery, marketplaceId, fromAddr, blockchain, title, description, startPrice, startPriceWei, currency, reservePrice, reservePriceWei, endTime, txHash, timestamp, timestamp)
	if err != nil {
		core.LogError("Could not index marketplace auction listing: " + err.Error())
		return
	}
	// Create auction record
	auctionQuery := "INSERT INTO marketplace_auctions (id, listingId, startTime, endTime, status, txHash, createdAt) VALUES (?, ?, ?, ?, 'active', ?, ?)"
	_, err = db.runParamSQLUpdate(auctionQuery, txHash+"_auction", marketplaceId, timestamp, endTime, txHash, timestamp)
	if err != nil {
		core.LogError("Could not create auction record: " + err.Error())
	}
}
func (db *SQLite) MarketplaceListingCancel(txHash string, blockchain string, fromAddr string, toAddr string, listingTxHash string, reason string, timestamp uint64) {
	if len(reason) > 500 {
		core.LogError("Reason too long in marketplace listing cancel: " + string(rune(len(reason))))
		return
	}
	checkQuery := "SELECT status FROM marketplace_listings WHERE txHash = ? LIMIT 1"
	rows, err := db.runParamSQLSelect(checkQuery, listingTxHash)
	if err != nil {
		core.LogError("Could not check listing status for cancellation: " + err.Error())
		return
	}
	defer rows.Close()
	var currentStatus string
	if !rows.Next() {
		core.LogError("Listing not found for cancellation: " + listingTxHash)
		return
	}
	err = rows.Scan(&currentStatus)
	if err != nil {
		core.LogError("Could not scan listing status: " + err.Error())
		return
	}
	if currentStatus != "active" {
		core.LogWarn("Attempted to cancel non-active listing: " + listingTxHash + " (status: " + currentStatus + ")")
		return
	}
	query := "UPDATE marketplace_listings SET status = 'cancelled', updatedAt = ? WHERE txHash = ? AND status = 'active'"
	result, err := db.runParamSQLUpdate(query, timestamp, listingTxHash)
	if err != nil {
		core.LogError("Could not cancel marketplace listing: " + err.Error())
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		core.LogWarn("Listing cancellation failed - possibly already cancelled: " + listingTxHash)
	} else {
		core.LogInfo("Cancelled marketplace listing: " + listingTxHash)
	}
}
func (db *SQLite) MarketplaceOfferCancel(txHash string, blockchain string, fromAddr string, toAddr string, offerTxHash string, reason string, timestamp uint64) {
	if len(reason) > 500 {
		core.LogError("Reason too long in marketplace offer cancel: " + string(rune(len(reason))))
		return
	}
	checkQuery := "SELECT status FROM marketplace_offers WHERE id = ? LIMIT 1"
	rows, err := db.runParamSQLSelect(checkQuery, offerTxHash)
	if err != nil {
		core.LogError("Could not check offer status for cancellation: " + err.Error())
		return
	}
	defer rows.Close()
	var currentStatus string
	if !rows.Next() {
		core.LogError("Offer not found for cancellation: " + offerTxHash)
		return
	}
	err = rows.Scan(&currentStatus)
	if err != nil {
		core.LogError("Could not scan offer status: " + err.Error())
		return
	}
	if currentStatus != "pending" {
		core.LogWarn("Attempted to cancel non-pending offer: " + offerTxHash + " (status: " + currentStatus + ")")
		return
	}
	query := "UPDATE marketplace_offers SET status = 'cancelled' WHERE id = ? AND status = 'pending'"
	result, err := db.runParamSQLUpdate(query, offerTxHash)
	if err != nil {
		core.LogError("Could not cancel marketplace offer: " + err.Error())
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		core.LogWarn("Offer cancellation failed - possibly already cancelled: " + offerTxHash)
	} else {
		core.LogInfo("Cancelled marketplace offer: " + offerTxHash)
	}
}
