package opencode

import (
	"os"
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

func TestIsChildOf_currentProcess(t *testing.T) {
	// Current process is always a child of PID 1 (init) transitively,
	// but direct parent check: os.Getppid() should be parent of os.Getpid()
	child := os.Getpid()
	parent := os.Getppid()
	if !IsChildOf(child, parent) {
		t.Errorf("IsChildOf(%d, %d) = false, want true", child, parent)
	}
}
