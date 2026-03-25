package opencode

import (
	"testing"
)

func TestFindActiveProcess_returnsSlice(t *testing.T) {
	// Just verify it doesn't error — opencode may or may not be running
	procs, err := FindActiveProcess()
	if err != nil {
		t.Fatalf("FindActiveProcess: %v", err)
	}
	// Result is a slice (possibly empty)
	_ = procs
}

func TestFindProcessBySession_unknownSessionReturnsZero(t *testing.T) {
	pid, err := FindProcessBySession("ses_notexist_xyz")
	if err != nil {
		t.Fatalf("FindProcessBySession: %v", err)
	}
	if pid != 0 {
		t.Errorf("expected pid=0 for unknown session, got %d", pid)
	}
}

