package cursor

import (
	"testing"
	"time"
)

func TestScanSession_WithComposers(t *testing.T) {
	tmpDir, db := createTestDB(t)

	c1 := ComposerData{
		ComposerID:    "comp-aaa-111-222",
		Name:          "First Chat",
		CreatedAt:     1700000000000,
		LastUpdatedAt: 1700001000000,
		Status:        "completed",
	}
	c2 := ComposerData{
		ComposerID:    "comp-bbb-333-444",
		Name:          "Second Chat",
		CreatedAt:     1700002000000,
		LastUpdatedAt: 1700003000000,
		Status:        "active",
	}
	insertComposer(t, db, c1)
	insertComposer(t, db, c2)
	db.Close()

	sessionList, err := ScanSession(tmpDir)
	if err != nil {
		t.Fatalf("ScanSession error: %v", err)
	}
	if len(sessionList) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessionList))
	}

	// Verify provider and environment
	for _, s := range sessionList {
		if s.Provider != "cursor" {
			t.Errorf("expected Provider 'cursor', got %q", s.Provider)
		}
		if s.Environment != "ide" {
			t.Errorf("expected Environment 'ide', got %q", s.Environment)
		}
	}
}

func TestScanSession_NoDB(t *testing.T) {
	tmpDir := t.TempDir()
	// No globalStorage directory

	sessionList, err := ScanSession(tmpDir)
	if err != nil {
		t.Fatalf("ScanSession should not error for missing DB, got: %v", err)
	}
	if sessionList != nil {
		t.Fatalf("expected nil for missing DB, got %d sessions", len(sessionList))
	}
}

func TestScanSession_EmptyDB(t *testing.T) {
	tmpDir, db := createTestDB(t)
	db.Close()

	sessionList, err := ScanSession(tmpDir)
	if err != nil {
		t.Fatalf("ScanSession error: %v", err)
	}
	if len(sessionList) != 0 {
		t.Fatalf("expected 0 sessions for empty DB, got %d", len(sessionList))
	}
}

func TestScanSession_SortedByLastActive(t *testing.T) {
	tmpDir, db := createTestDB(t)

	// Insert 3 composers with different timestamps (not in order)
	c1 := ComposerData{
		ComposerID:    "oldest",
		Name:          "Oldest",
		CreatedAt:     1700000000000,
		LastUpdatedAt: 1700000100000, // oldest
	}
	c2 := ComposerData{
		ComposerID:    "newest",
		Name:          "Newest",
		CreatedAt:     1700000200000,
		LastUpdatedAt: 1700000300000, // newest
	}
	c3 := ComposerData{
		ComposerID:    "middle",
		Name:          "Middle",
		CreatedAt:     1700000150000,
		LastUpdatedAt: 1700000200000, // middle
	}

	insertComposer(t, db, c1)
	insertComposer(t, db, c2)
	insertComposer(t, db, c3)
	db.Close()

	sessionList, err := ScanSession(tmpDir)
	if err != nil {
		t.Fatalf("ScanSession error: %v", err)
	}
	if len(sessionList) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessionList))
	}

	// Verify sorted descending by LastActive
	if sessionList[0].ID != "newest" {
		t.Errorf("expected first session 'newest', got %q", sessionList[0].ID)
	}
	if sessionList[1].ID != "middle" {
		t.Errorf("expected second session 'middle', got %q", sessionList[1].ID)
	}
	if sessionList[2].ID != "oldest" {
		t.Errorf("expected third session 'oldest', got %q", sessionList[2].ID)
	}

	// Verify timestamps are actually descending
	for i := 0; i < len(sessionList)-1; i++ {
		if !sessionList[i].LastActive.After(sessionList[i+1].LastActive) {
			t.Errorf("session[%d].LastActive (%v) should be after session[%d].LastActive (%v)",
				i, sessionList[i].LastActive, i+1, sessionList[i+1].LastActive)
		}
	}
}

func TestComposerToSession_Full(t *testing.T) {
	cd := ComposerData{
		ComposerID:    "abcdef-123456-789",
		Name:          "My Test Session",
		CreatedAt:     1700000000000,
		LastUpdatedAt: 1700001000000,
		Status:        "completed",
		IsAgentic:     true,
		FullConversationHeadersOnly: []BubbleHeader{
			{BubbleID: "b1", Type: 1},
			{BubbleID: "b2", Type: 2},
			{BubbleID: "b3", Type: 1},
		},
	}

	s := composerToSession(cd, "/home/user/project", "project", "/path/to/state.vscdb")

	if s.ID != "abcdef-123456-789" {
		t.Errorf("expected ID 'abcdef-123456-789', got %q", s.ID)
	}
	if s.ShortID != "abcdef" {
		t.Errorf("expected ShortID 'abcdef', got %q", s.ShortID)
	}
	if s.Provider != "cursor" {
		t.Errorf("expected Provider 'cursor', got %q", s.Provider)
	}
	if s.Status != "dead" {
		t.Errorf("expected Status 'dead', got %q", s.Status)
	}
	if s.Project != "project" {
		t.Errorf("expected Project 'project', got %q", s.Project)
	}
	if s.ProjectPath != "/home/user/project" {
		t.Errorf("expected ProjectPath '/home/user/project', got %q", s.ProjectPath)
	}
	if s.Description != "My Test Session" {
		t.Errorf("expected Description 'My Test Session', got %q", s.Description)
	}
	if s.Environment != "ide" {
		t.Errorf("expected Environment 'ide', got %q", s.Environment)
	}
	if s.MessageCount != 3 {
		t.Errorf("expected MessageCount 3, got %d", s.MessageCount)
	}
	if s.FilePath != "/path/to/state.vscdb" {
		t.Errorf("expected FilePath '/path/to/state.vscdb', got %q", s.FilePath)
	}

	expectedStart := time.UnixMilli(1700000000000)
	if !s.StartedAt.Equal(expectedStart) {
		t.Errorf("expected StartedAt %v, got %v", expectedStart, s.StartedAt)
	}
	expectedLast := time.UnixMilli(1700001000000)
	if !s.LastActive.Equal(expectedLast) {
		t.Errorf("expected LastActive %v, got %v", expectedLast, s.LastActive)
	}
}

func TestComposerToSession_ShortID(t *testing.T) {
	cd := ComposerData{
		ComposerID: "abc",
		Name:       "Short ID Session",
	}

	s := composerToSession(cd, "", "Global", "")

	// When ID is shorter than 6, ShortID should be the full ID
	if s.ShortID != "abc" {
		t.Errorf("expected ShortID 'abc' for short ID, got %q", s.ShortID)
	}
}

func TestComposerToSession_ZeroTimestamps(t *testing.T) {
	cd := ComposerData{
		ComposerID:    "zero-ts",
		Name:          "Zero Timestamps",
		CreatedAt:     0,
		LastUpdatedAt: 0,
	}

	s := composerToSession(cd, "", "Global", "")

	if !s.StartedAt.IsZero() {
		t.Errorf("expected zero StartedAt, got %v", s.StartedAt)
	}
	if !s.LastActive.IsZero() {
		t.Errorf("expected zero LastActive, got %v", s.LastActive)
	}
}
