package opencode

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/glory0216/taux/internal/model"
)

func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata")
}

func TestScanSession_returnsAllSessions(t *testing.T) {
	sessions, err := ScanSession(testdataDir())
	if err != nil {
		t.Fatalf("ScanSession: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestScanSession_sortedByLastActiveDescending(t *testing.T) {
	sessions, err := ScanSession(testdataDir())
	if err != nil {
		t.Fatalf("ScanSession: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatal("need at least 2 sessions")
	}
	if sessions[0].LastActive.Before(sessions[1].LastActive) {
		t.Error("sessions not sorted by LastActive descending")
	}
}

func TestScanSession_parsesFields(t *testing.T) {
	sessions, err := ScanSession(testdataDir())
	if err != nil {
		t.Fatalf("ScanSession: %v", err)
	}

	var s *model.Session
	for i := range sessions {
		if sessions[i].ID == "ses_def456" {
			s = &sessions[i]
			break
		}
	}
	if s == nil {
		t.Fatal("ses_def456 not found")
	}

	if s.Provider != "opencode" {
		t.Errorf("Provider = %q, want %q", s.Provider, "opencode")
	}
	if s.Project != "myapp" {
		t.Errorf("Project = %q, want %q", s.Project, "myapp")
	}
	if s.Description != "Add authentication feature" {
		t.Errorf("Description = %q, want %q", s.Description, "Add authentication feature")
	}
	if s.ProjectPath != "/Users/user/projects/myapp" {
		t.Errorf("ProjectPath = %q, want %q", s.ProjectPath, "/Users/user/projects/myapp")
	}

	wantStarted := time.UnixMilli(1740000000000)
	if !s.StartedAt.Equal(wantStarted) {
		t.Errorf("StartedAt = %v, want %v", s.StartedAt, wantStarted)
	}
	wantLast := time.UnixMilli(1740003600000)
	if !s.LastActive.Equal(wantLast) {
		t.Errorf("LastActive = %v, want %v", s.LastActive, wantLast)
	}
	if s.ShortID != "ses_de" {
		t.Errorf("ShortID = %q, want %q", s.ShortID, "ses_de")
	}
	if s.Environment != "cli" {
		t.Errorf("Environment = %q, want %q", s.Environment, "cli")
	}
}

func TestScanSession_emptyDir(t *testing.T) {
	sessions, err := ScanSession(t.TempDir())
	if err != nil {
		t.Fatalf("expected no error on empty dir, got %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}
