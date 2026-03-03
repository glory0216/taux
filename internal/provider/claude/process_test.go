package claude

import "testing"

// ---------------------------------------------------------------------------
// extractResumeSessionID tests
// ---------------------------------------------------------------------------

func TestExtractResumeSessionID_Found(t *testing.T) {
	got := extractResumeSessionID("claude --resume abc-123 --verbose")
	if got != "abc-123" {
		t.Errorf("extractResumeSessionID = %q, want %q", got, "abc-123")
	}
}

func TestExtractResumeSessionID_NotFound(t *testing.T) {
	got := extractResumeSessionID("claude --help")
	if got != "" {
		t.Errorf("extractResumeSessionID = %q, want %q", got, "")
	}
}

func TestExtractResumeSessionID_AtEnd(t *testing.T) {
	got := extractResumeSessionID("claude --resume abc")
	if got != "abc" {
		t.Errorf("extractResumeSessionID = %q, want %q", got, "abc")
	}
}

func TestExtractResumeSessionID_NoValue(t *testing.T) {
	got := extractResumeSessionID("claude --resume")
	if got != "" {
		t.Errorf("extractResumeSessionID = %q, want %q", got, "")
	}
}

func TestExtractResumeSessionID_Empty(t *testing.T) {
	got := extractResumeSessionID("")
	if got != "" {
		t.Errorf("extractResumeSessionID = %q, want %q", got, "")
	}
}
