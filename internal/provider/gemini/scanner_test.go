package gemini

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSession_EmptyTmpDir(t *testing.T) {
	tmpDir := t.TempDir()
	geminiTmp := filepath.Join(tmpDir, "tmp")
	if err := os.MkdirAll(geminiTmp, 0o755); err != nil {
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

	// Create directory structure: tmp/<hash>/chats/session-test.json
	chatsDir := filepath.Join(tmpDir, "tmp", "abc123hash", "chats")
	if err := os.MkdirAll(chatsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Copy test JSON
	src := testdataPath("object_format.json")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(chatsDir, "session-test.json")
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
	if s.ID != "session-test" {
		t.Errorf("ID = %q, want %q", s.ID, "session-test")
	}
	if s.Provider != "gemini" {
		t.Errorf("Provider = %q, want %q", s.Provider, "gemini")
	}
	if s.Model != "gemini-2.5-pro" {
		t.Errorf("Model = %q, want %q", s.Model, "gemini-2.5-pro")
	}
	if s.Description == "" {
		t.Error("Description should not be empty")
	}
	if s.MessageCount != 4 {
		t.Errorf("MessageCount = %d, want 4", s.MessageCount)
	}
}

func TestScanSession_NoTmpDir(t *testing.T) {
	tmpDir := t.TempDir()

	sessionList, err := ScanSession(tmpDir)
	if err != nil {
		t.Fatalf("ScanSession should not error: %v", err)
	}
	if len(sessionList) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessionList))
	}
}

func TestParseSessionTimestamp(t *testing.T) {
	testCaseList := []struct {
		name      string
		sessionID string
		wantZero  bool
	}{
		{"valid timestamp", "session-2025-03-01T14-30-00Z", false},
		{"no prefix", "2025-03-01T14-30-00Z", false},
		{"invalid format", "some-random-id", true},
		{"short string", "abc", true},
		{"empty", "", true},
	}

	for _, tc := range testCaseList {
		t.Run(tc.name, func(t *testing.T) {
			ts := parseSessionTimestamp(tc.sessionID)
			if tc.wantZero && !ts.IsZero() {
				t.Errorf("parseSessionTimestamp(%q) should be zero", tc.sessionID)
			}
			if !tc.wantZero && ts.IsZero() {
				t.Errorf("parseSessionTimestamp(%q) should not be zero", tc.sessionID)
			}
		})
	}
}

func TestResolveProjectName_NoMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	hashDir := filepath.Join(tmpDir, "longhashvalue123456")
	if err := os.MkdirAll(hashDir, 0o755); err != nil {
		t.Fatal(err)
	}

	name := resolveProjectName(tmpDir, "longhashvalue123456")
	// Should use first 8 chars of hash as fallback
	if name != "longhash" {
		t.Errorf("resolveProjectName = %q, want %q", name, "longhash")
	}
}

func TestResolveProjectName_Empty(t *testing.T) {
	name := resolveProjectName(t.TempDir(), "")
	if name != "" {
		t.Errorf("resolveProjectName = %q, want empty", name)
	}
}

func TestExtractQuickMetadata_ObjectFormat(t *testing.T) {
	meta := extractQuickMetadata(testdataPath("object_format.json"))

	if meta.model != "gemini-2.5-pro" {
		t.Errorf("model = %q, want %q", meta.model, "gemini-2.5-pro")
	}
	if meta.description != "Explain how goroutines work in Go" {
		t.Errorf("description = %q, want %q", meta.description, "Explain how goroutines work in Go")
	}
	if meta.messageCount != 4 {
		t.Errorf("messageCount = %d, want 4", meta.messageCount)
	}
}

func TestExtractQuickMetadata_EmptyFile(t *testing.T) {
	meta := extractQuickMetadata(testdataPath("empty.json"))

	if meta.model != "" {
		t.Errorf("model = %q, want empty", meta.model)
	}
	if meta.messageCount != 0 {
		t.Errorf("messageCount = %d, want 0", meta.messageCount)
	}
}
