package claude

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// ScanSession tests
// ---------------------------------------------------------------------------

func TestScanSession_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	// Create an empty projects directory
	if err := os.MkdirAll(filepath.Join(tmpDir, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}

	sessionList, err := ScanSession(tmpDir)
	if err != nil {
		t.Fatalf("ScanSession error: %v", err)
	}
	if len(sessionList) != 0 {
		t.Errorf("got %d sessions, want 0", len(sessionList))
	}
}

func TestScanSession_ValidSession(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure: projects/-Users-test-project/<uuid>.jsonl
	projectDir := filepath.Join(tmpDir, "projects", "-Users-test-project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Copy valid_session.jsonl into the temp directory with a UUID-style filename
	srcData, err := os.ReadFile(testdataPath("valid_session.jsonl"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	sessionFile := filepath.Join(projectDir, "a1b2c3d4-e5f6-7890-abcd-ef1234567890.jsonl")
	if err := os.WriteFile(sessionFile, srcData, 0o644); err != nil {
		t.Fatal(err)
	}

	sessionList, err := ScanSession(tmpDir)
	if err != nil {
		t.Fatalf("ScanSession error: %v", err)
	}
	if len(sessionList) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessionList))
	}

	s := sessionList[0]
	if s.ID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("ID = %q, want %q", s.ID, "a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	}
	if s.ShortID != "a1b2c3" {
		t.Errorf("ShortID = %q, want %q", s.ShortID, "a1b2c3")
	}
	if s.Provider != "claude" {
		t.Errorf("Provider = %q, want %q", s.Provider, "claude")
	}
	if s.Project != "project" {
		t.Errorf("Project = %q, want %q", s.Project, "project")
	}
	if s.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", s.Model, "claude-sonnet-4-6")
	}
	if s.Environment != "cli" {
		t.Errorf("Environment = %q, want %q", s.Environment, "cli")
	}
	if s.CWD != "/Users/test/project" {
		t.Errorf("CWD = %q, want %q", s.CWD, "/Users/test/project")
	}
	if s.FileSize <= 0 {
		t.Errorf("FileSize = %d, want > 0", s.FileSize)
	}
	if s.Description == "" {
		t.Error("Description is empty, want non-empty")
	}
}

func TestScanSession_NoProjectsDir(t *testing.T) {
	tmpDir := t.TempDir()
	// No "projects" subdirectory exists

	sessionList, err := ScanSession(tmpDir)
	if err != nil {
		t.Fatalf("ScanSession error: %v", err)
	}
	if len(sessionList) != 0 {
		t.Errorf("got %d sessions, want 0", len(sessionList))
	}
}

// ---------------------------------------------------------------------------
// decodeProjectPath tests
// ---------------------------------------------------------------------------

func TestDecodeProjectPath_Normal(t *testing.T) {
	got := decodeProjectPath("-Users-user-project")
	want := "/Users/user/project"
	if got != want {
		t.Errorf("decodeProjectPath(%q) = %q, want %q", "-Users-user-project", got, want)
	}
}

func TestDecodeProjectPath_Empty(t *testing.T) {
	got := decodeProjectPath("")
	if got != "" {
		t.Errorf("decodeProjectPath(%q) = %q, want %q", "", got, "")
	}
}
