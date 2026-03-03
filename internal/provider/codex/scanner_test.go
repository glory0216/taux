package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSession_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sessionList, err := ScanSession(tmpDir)
	if err != nil {
		t.Fatalf("ScanSession failed: %v", err)
	}
	if len(sessionList) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessionList))
	}
}

func TestScanSession_ValidSession(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure: sessions/2025/03/01/rollout-test123.jsonl
	sessionDir := filepath.Join(tmpDir, "sessions", "2025", "03", "01")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Copy test JSONL content
	src := testdataPath("valid_session.jsonl")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(sessionDir, "rollout-test123.jsonl")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}

	sessionList, err := ScanSession(tmpDir)
	if err != nil {
		t.Fatalf("ScanSession failed: %v", err)
	}

	if len(sessionList) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessionList))
	}

	s := sessionList[0]
	if s.ID != "test123" {
		t.Errorf("ID = %q, want %q", s.ID, "test123")
	}
	if s.ShortID != "test12" {
		t.Errorf("ShortID = %q, want %q", s.ShortID, "test12")
	}
	if s.Provider != "codex" {
		t.Errorf("Provider = %q, want %q", s.Provider, "codex")
	}
	if s.Model != "o4-mini" {
		t.Errorf("Model = %q, want %q", s.Model, "o4-mini")
	}
	if s.Description == "" {
		t.Error("Description should not be empty")
	}
}

func TestScanSession_NoSessionsDir(t *testing.T) {
	tmpDir := t.TempDir()
	// No "sessions" subdirectory

	sessionList, err := ScanSession(tmpDir)
	if err != nil {
		t.Fatalf("ScanSession should not error: %v", err)
	}
	if len(sessionList) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessionList))
	}
}

func TestExtractQuickMetadata_ValidFile(t *testing.T) {
	meta := extractQuickMetadata(testdataPath("valid_session.jsonl"))

	if meta.model != "o4-mini" {
		t.Errorf("model = %q, want %q", meta.model, "o4-mini")
	}
	if meta.description == "" {
		t.Error("description should not be empty")
	}
	if meta.firstTime.IsZero() {
		t.Error("firstTime should not be zero")
	}
	if meta.lastTime.IsZero() {
		t.Error("lastTime should not be zero")
	}
}

func TestExtractQuickMetadata_EmptyFile(t *testing.T) {
	meta := extractQuickMetadata(testdataPath("empty.jsonl"))

	if meta.model != "" {
		t.Errorf("model = %q, want empty", meta.model)
	}
	if meta.description != "" {
		t.Errorf("description = %q, want empty", meta.description)
	}
}

func TestExtractTimestamp(t *testing.T) {
	line := []byte(`{"timestamp":1709312400.123}`)
	ts := extractTimestamp(line)
	if ts.IsZero() {
		t.Error("timestamp should not be zero")
	}
	if ts.Unix() != 1709312400 {
		t.Errorf("timestamp unix = %d, want %d", ts.Unix(), 1709312400)
	}
}

func TestExtractTimestamp_CreatedAt(t *testing.T) {
	line := []byte(`{"created_at":1709312400.0}`)
	ts := extractTimestamp(line)
	if ts.IsZero() {
		t.Error("timestamp should not be zero for created_at")
	}
}

func TestExtractTimestamp_NoTimestamp(t *testing.T) {
	line := []byte(`{"type":"something"}`)
	ts := extractTimestamp(line)
	if !ts.IsZero() {
		t.Error("timestamp should be zero when no timestamp field")
	}
}

func TestEstimateLineCount(t *testing.T) {
	if estimateLineCount(0) != 0 {
		t.Error("expected 0 for empty file")
	}
	if estimateLineCount(1000) != 1 {
		t.Errorf("expected 1 for 1000 bytes, got %d", estimateLineCount(1000))
	}
	if estimateLineCount(30000) != 10 {
		t.Errorf("expected 10 for 30000 bytes, got %d", estimateLineCount(30000))
	}
}
