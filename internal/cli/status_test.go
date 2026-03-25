package cli

import (
	"testing"

	"github.com/glory0216/taux/internal/model"
)

// --- buildProviderSnapshot ---

func TestBuildProviderSnapshot_usesPIDAndShortID(t *testing.T) {
	s := model.Session{ID: "abcdef123456", ShortID: "abcdef", Project: "myapp", PID: 42}

	snap := buildProviderSnapshot(s)

	if snap.PID != 42 || snap.ShortID != "abcdef" || snap.Project != "myapp" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}

func TestBuildProviderSnapshot_fallsBackToIDPrefix(t *testing.T) {
	// ShortID empty — should truncate ID
	s := model.Session{ID: "abcdef123456", ShortID: "", Project: "proj", PID: 7}

	snap := buildProviderSnapshot(s)

	if snap.ShortID != "abcdef" {
		t.Fatalf("expected ShortID 'abcdef', got %q", snap.ShortID)
	}
}

// --- currentPIDSet ---

func TestCurrentPIDSet_excludesZeroPID(t *testing.T) {
	sessions := []model.Session{
		{PID: 0, ID: "aaa"},
		{PID: 5, ID: "bbb"},
	}

	set := currentPIDSet(sessions)

	if set[0] {
		t.Fatal("PID 0 should not be in set")
	}
	if !set[5] {
		t.Fatal("PID 5 should be in set")
	}
}

// --- detectGoneSnapshots ---

func TestDetectGoneSnapshots_reportsGonePID(t *testing.T) {
	prev := []providerSessionSnapshot{{PID: 100, ShortID: "abc123", Project: "myapp"}}
	current := map[int]bool{200: true}

	gone := detectGoneSnapshots(prev, current)

	if len(gone) != 1 || gone[0].PID != 100 {
		t.Fatalf("expected 1 gone snapshot with PID 100, got %v", gone)
	}
}

func TestDetectGoneSnapshots_ignoresActivePID(t *testing.T) {
	prev := []providerSessionSnapshot{{PID: 100, ShortID: "abc123", Project: "myapp"}}
	current := map[int]bool{100: true}

	gone := detectGoneSnapshots(prev, current)

	if len(gone) != 0 {
		t.Fatalf("expected no gone snapshots, got %v", gone)
	}
}

func TestDetectGoneSnapshots_emptyPrev(t *testing.T) {
	gone := detectGoneSnapshots(nil, map[int]bool{100: true})

	if len(gone) != 0 {
		t.Fatalf("expected no gone snapshots, got %v", gone)
	}
}

func TestDetectGoneSnapshots_allGone(t *testing.T) {
	prev := []providerSessionSnapshot{
		{PID: 1, ShortID: "aaa", Project: "p1"},
		{PID: 2, ShortID: "bbb", Project: "p2"},
	}

	gone := detectGoneSnapshots(prev, map[int]bool{})

	if len(gone) != 2 {
		t.Fatalf("expected 2 gone snapshots, got %v", gone)
	}
}

func TestDetectGoneSnapshots_partiallyGone(t *testing.T) {
	prev := []providerSessionSnapshot{
		{PID: 1, ShortID: "aaa", Project: "p1"},
		{PID: 2, ShortID: "bbb", Project: "p2"},
		{PID: 3, ShortID: "ccc", Project: "p3"},
	}

	gone := detectGoneSnapshots(prev, map[int]bool{2: true})

	if len(gone) != 2 {
		t.Fatalf("expected 2 gone snapshots, got %v", gone)
	}
	for _, g := range gone {
		if g.PID == 2 {
			t.Fatalf("PID 2 should not be in gone list")
		}
	}
}
