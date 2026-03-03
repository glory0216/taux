package cursor

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

// openDB opens a state.vscdb file in read-only mode.
// Returns nil, nil if the file does not exist.
func openDB(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	uri := fmt.Sprintf("file:%s?mode=ro&_journal_mode=WAL&_busy_timeout=1000", path)
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// openDBReadWrite opens a state.vscdb file in read-write mode.
// Returns nil, nil if the file does not exist.
func openDBReadWrite(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	uri := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=3000", path)
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// deleteComposerData removes composerData and bubbleId rows for a given composer ID.
func deleteComposerData(db *sql.DB, composerID string) (int64, error) {
	if !tableExists(db, "cursorDiskKV") {
		return 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Delete composerData:<id>
	res1, err := tx.Exec("DELETE FROM cursorDiskKV WHERE key = ?",
		"composerData:"+composerID)
	if err != nil {
		return 0, err
	}
	n1, _ := res1.RowsAffected()

	// Delete bubbleId:<id>:*
	res2, err := tx.Exec("DELETE FROM cursorDiskKV WHERE key LIKE ?",
		fmt.Sprintf("bubbleId:%s:%%", composerID))
	if err != nil {
		return 0, err
	}
	n2, _ := res2.RowsAffected()

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n1 + n2, nil
}

// tableExists checks if a table exists in the database.
func tableExists(db *sql.DB, tableName string) bool {
	var name string
	err := db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
		tableName,
	).Scan(&name)
	return err == nil
}

// queryComposerDataList retrieves all composerData records from cursorDiskKV.
func queryComposerDataList(db *sql.DB) ([]ComposerData, error) {
	if !tableExists(db, "cursorDiskKV") {
		return nil, nil
	}

	rows, err := db.Query(
		"SELECT key, value FROM cursorDiskKV WHERE key LIKE 'composerData:%'",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ComposerData
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}

		var cd ComposerData
		if err := json.Unmarshal(value, &cd); err != nil {
			continue
		}

		// If composerID is empty, extract from key
		if cd.ComposerID == "" {
			cd.ComposerID = strings.TrimPrefix(key, "composerData:")
		}

		result = append(result, cd)
	}
	return result, rows.Err()
}

// queryComposerByID retrieves a single composerData record by ID.
// More efficient than queryComposerDataList when we only need one.
func queryComposerByID(db *sql.DB, composerID string) (*ComposerData, error) {
	if !tableExists(db, "cursorDiskKV") {
		return nil, nil
	}

	key := "composerData:" + composerID
	var value []byte
	err := db.QueryRow("SELECT value FROM cursorDiskKV WHERE key = ?", key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var cd ComposerData
	if err := json.Unmarshal(value, &cd); err != nil {
		return nil, err
	}
	if cd.ComposerID == "" {
		cd.ComposerID = composerID
	}
	return &cd, nil
}

// queryBubbleDataBatch retrieves all bubbles for a composer efficiently.
func queryBubbleDataBatch(db *sql.DB, composerID string) ([]BubbleData, error) {
	if !tableExists(db, "cursorDiskKV") {
		return nil, nil
	}

	prefix := fmt.Sprintf("bubbleId:%s:%%", composerID)
	rows, err := db.Query(
		"SELECT value FROM cursorDiskKV WHERE key LIKE ?", prefix,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []BubbleData
	for rows.Next() {
		var value []byte
		if err := rows.Scan(&value); err != nil {
			continue
		}
		var bd BubbleData
		if err := json.Unmarshal(value, &bd); err != nil {
			continue
		}
		result = append(result, bd)
	}
	return result, rows.Err()
}
