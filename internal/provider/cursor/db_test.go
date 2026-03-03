package cursor

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// createTestDB creates a temporary SQLite database with the cursorDiskKV table.
func createTestDB(t *testing.T) (string, *sql.DB) {
	t.Helper()
	tmpDir := t.TempDir()

	// Create directory structure matching what the provider expects
	gsDir := filepath.Join(tmpDir, "globalStorage")
	if err := os.MkdirAll(gsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(gsDir, "state.vscdb")

	db, err := sql.Open("sqlite", "file:"+dbPath+"?_journal_mode=WAL")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS cursorDiskKV (key TEXT PRIMARY KEY, value BLOB)")
	if err != nil {
		t.Fatal(err)
	}

	return tmpDir, db
}

// insertComposer inserts a ComposerData row into the test DB.
func insertComposer(t *testing.T, db *sql.DB, cd ComposerData) {
	t.Helper()
	data, err := json.Marshal(cd)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO cursorDiskKV (key, value) VALUES (?, ?)",
		"composerData:"+cd.ComposerID, data)
	if err != nil {
		t.Fatal(err)
	}
}

// insertBubble inserts a BubbleData row into the test DB.
func insertBubble(t *testing.T, db *sql.DB, composerID string, bd BubbleData) {
	t.Helper()
	data, err := json.Marshal(bd)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO cursorDiskKV (key, value) VALUES (?, ?)",
		"bubbleId:"+composerID+":"+bd.BubbleID, data)
	if err != nil {
		t.Fatal(err)
	}
}

// --- openDB tests ---

func TestOpenDB_Exists(t *testing.T) {
	tmpDir, setupDB := createTestDB(t)
	setupDB.Close()

	dbPath := filepath.Join(tmpDir, "globalStorage", "state.vscdb")
	db, err := openDB(dbPath)
	if err != nil {
		t.Fatalf("openDB returned error: %v", err)
	}
	if db == nil {
		t.Fatal("openDB returned nil for existing DB")
	}
	defer db.Close()

	// Verify we can query the table
	if !tableExists(db, "cursorDiskKV") {
		t.Fatal("cursorDiskKV table should exist")
	}
}

func TestOpenDB_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nonexistent", "state.vscdb")

	db, err := openDB(dbPath)
	if err != nil {
		t.Fatalf("openDB should not return error for missing file, got: %v", err)
	}
	if db != nil {
		db.Close()
		t.Fatal("openDB should return nil for missing file")
	}
}

// --- tableExists tests ---

func TestTableExists_True(t *testing.T) {
	_, db := createTestDB(t)
	defer db.Close()

	if !tableExists(db, "cursorDiskKV") {
		t.Fatal("tableExists should return true for cursorDiskKV")
	}
}

func TestTableExists_False(t *testing.T) {
	_, db := createTestDB(t)
	defer db.Close()

	if tableExists(db, "nonexistent_table") {
		t.Fatal("tableExists should return false for nonexistent table")
	}
}

// --- queryComposerDataList tests ---

func TestQueryComposerDataList_WithData(t *testing.T) {
	_, db := createTestDB(t)
	defer db.Close()

	c1 := ComposerData{
		ComposerID:    "aaa-111",
		Name:          "First Session",
		CreatedAt:     1700000000000,
		LastUpdatedAt: 1700001000000,
		Status:        "completed",
		IsAgentic:     true,
	}
	c2 := ComposerData{
		ComposerID:    "bbb-222",
		Name:          "Second Session",
		CreatedAt:     1700002000000,
		LastUpdatedAt: 1700003000000,
		Status:        "active",
		IsAgentic:     false,
	}

	insertComposer(t, db, c1)
	insertComposer(t, db, c2)

	result, err := queryComposerDataList(db)
	if err != nil {
		t.Fatalf("queryComposerDataList error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 composers, got %d", len(result))
	}

	// Build map for order-independent verification
	m := make(map[string]ComposerData)
	for _, cd := range result {
		m[cd.ComposerID] = cd
	}

	r1, ok := m["aaa-111"]
	if !ok {
		t.Fatal("missing composer aaa-111")
	}
	if r1.Name != "First Session" {
		t.Errorf("expected Name 'First Session', got %q", r1.Name)
	}
	if r1.CreatedAt != 1700000000000 {
		t.Errorf("expected CreatedAt 1700000000000, got %d", r1.CreatedAt)
	}
	if r1.Status != "completed" {
		t.Errorf("expected Status 'completed', got %q", r1.Status)
	}
	if !r1.IsAgentic {
		t.Error("expected IsAgentic true")
	}

	r2, ok := m["bbb-222"]
	if !ok {
		t.Fatal("missing composer bbb-222")
	}
	if r2.Name != "Second Session" {
		t.Errorf("expected Name 'Second Session', got %q", r2.Name)
	}
	if r2.IsAgentic {
		t.Error("expected IsAgentic false")
	}
}

func TestQueryComposerDataList_Empty(t *testing.T) {
	_, db := createTestDB(t)
	defer db.Close()

	result, err := queryComposerDataList(db)
	if err != nil {
		t.Fatalf("queryComposerDataList error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 composers, got %d", len(result))
	}
}

// --- queryComposerByID tests ---

func TestQueryComposerByID_Found(t *testing.T) {
	_, db := createTestDB(t)
	defer db.Close()

	cd := ComposerData{
		ComposerID:    "target-id-123",
		Name:          "Target Session",
		CreatedAt:     1700000000000,
		LastUpdatedAt: 1700001000000,
		Status:        "active",
		IsAgentic:     true,
		FullConversationHeadersOnly: []BubbleHeader{
			{BubbleID: "b1", Type: 1},
			{BubbleID: "b2", Type: 2},
		},
	}
	insertComposer(t, db, cd)

	// Insert another composer that should not be returned
	other := ComposerData{ComposerID: "other-id", Name: "Other"}
	insertComposer(t, db, other)

	result, err := queryComposerByID(db, "target-id-123")
	if err != nil {
		t.Fatalf("queryComposerByID error: %v", err)
	}
	if result == nil {
		t.Fatal("queryComposerByID returned nil for existing ID")
	}
	if result.ComposerID != "target-id-123" {
		t.Errorf("expected ComposerID 'target-id-123', got %q", result.ComposerID)
	}
	if result.Name != "Target Session" {
		t.Errorf("expected Name 'Target Session', got %q", result.Name)
	}
	if len(result.FullConversationHeadersOnly) != 2 {
		t.Errorf("expected 2 bubble headers, got %d", len(result.FullConversationHeadersOnly))
	}
}

func TestQueryComposerByID_NotFound(t *testing.T) {
	_, db := createTestDB(t)
	defer db.Close()

	result, err := queryComposerByID(db, "nonexistent-id")
	if err != nil {
		t.Fatalf("queryComposerByID error: %v", err)
	}
	if result != nil {
		t.Fatal("queryComposerByID should return nil for nonexistent ID")
	}
}

// --- queryBubbleDataBatch tests ---

func TestQueryBubbleDataBatch_WithData(t *testing.T) {
	_, db := createTestDB(t)
	defer db.Close()

	composerID := "comp-abc"

	b1 := BubbleData{BubbleID: "b1", Type: 1, Text: "Hello", RawText: "Hello raw"}
	b2 := BubbleData{BubbleID: "b2", Type: 2, Text: "Hi there", RawText: "Hi there raw",
		TokenCount: &BubbleTokenCount{InputTokens: 10, OutputTokens: 20}}
	b3 := BubbleData{BubbleID: "b3", Type: 1, Text: "Thanks", RawText: "Thanks raw"}

	insertBubble(t, db, composerID, b1)
	insertBubble(t, db, composerID, b2)
	insertBubble(t, db, composerID, b3)

	// Insert a bubble for a different composer (should not be returned)
	insertBubble(t, db, "other-comp", BubbleData{BubbleID: "b4", Type: 1, Text: "Other"})

	result, err := queryBubbleDataBatch(db, composerID)
	if err != nil {
		t.Fatalf("queryBubbleDataBatch error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 bubbles, got %d", len(result))
	}

	// Build map for order-independent verification
	m := make(map[string]BubbleData)
	for _, bd := range result {
		m[bd.BubbleID] = bd
	}

	r1, ok := m["b1"]
	if !ok {
		t.Fatal("missing bubble b1")
	}
	if r1.Type != 1 {
		t.Errorf("expected Type 1, got %d", r1.Type)
	}
	if r1.Text != "Hello" {
		t.Errorf("expected Text 'Hello', got %q", r1.Text)
	}

	r2, ok := m["b2"]
	if !ok {
		t.Fatal("missing bubble b2")
	}
	if r2.TokenCount == nil {
		t.Fatal("expected non-nil TokenCount for b2")
	}
	if r2.TokenCount.InputTokens != 10 {
		t.Errorf("expected InputTokens 10, got %d", r2.TokenCount.InputTokens)
	}
	if r2.TokenCount.OutputTokens != 20 {
		t.Errorf("expected OutputTokens 20, got %d", r2.TokenCount.OutputTokens)
	}
}

func TestQueryBubbleDataBatch_Empty(t *testing.T) {
	_, db := createTestDB(t)
	defer db.Close()

	result, err := queryBubbleDataBatch(db, "nonexistent-comp")
	if err != nil {
		t.Fatalf("queryBubbleDataBatch error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 bubbles, got %d", len(result))
	}
}

// --- deleteComposerData tests ---

func TestDeleteComposerData_Success(t *testing.T) {
	_, db := createTestDB(t)
	defer db.Close()

	composerID := "del-target"

	// Insert composer + bubbles
	cd := ComposerData{ComposerID: composerID, Name: "To Delete"}
	insertComposer(t, db, cd)
	insertBubble(t, db, composerID, BubbleData{BubbleID: "b1", Type: 1, Text: "msg1"})
	insertBubble(t, db, composerID, BubbleData{BubbleID: "b2", Type: 2, Text: "msg2"})

	// Insert another composer that should survive
	other := ComposerData{ComposerID: "keep-this", Name: "Keep"}
	insertComposer(t, db, other)

	deleted, err := deleteComposerData(db, composerID)
	if err != nil {
		t.Fatalf("deleteComposerData error: %v", err)
	}
	// 1 composerData row + 2 bubbleId rows = 3
	if deleted != 3 {
		t.Errorf("expected 3 deleted rows, got %d", deleted)
	}

	// Verify composer is gone
	result, err := queryComposerByID(db, composerID)
	if err != nil {
		t.Fatalf("queryComposerByID error: %v", err)
	}
	if result != nil {
		t.Fatal("deleted composer should not be found")
	}

	// Verify bubbles are gone
	bubbleList, err := queryBubbleDataBatch(db, composerID)
	if err != nil {
		t.Fatalf("queryBubbleDataBatch error: %v", err)
	}
	if len(bubbleList) != 0 {
		t.Fatalf("expected 0 bubbles after delete, got %d", len(bubbleList))
	}

	// Verify other composer survives
	kept, err := queryComposerByID(db, "keep-this")
	if err != nil {
		t.Fatalf("queryComposerByID error: %v", err)
	}
	if kept == nil {
		t.Fatal("unrelated composer should survive delete")
	}
}
