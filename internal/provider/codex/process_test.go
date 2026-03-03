package codex

import "testing"

func TestIsCodexProcess(t *testing.T) {
	testCaseList := []struct {
		name    string
		argLine string
		want    bool
	}{
		{"exact binary", "codex --resume abc123", true},
		{"full path", "/usr/local/bin/codex --resume abc123", true},
		{"home path", "/Users/user/.local/bin/codex", true},
		{"not codex - codex-helper", "codex-helper run", false},
		{"not codex - grep", "grep codex", false},
		{"not codex - empty", "", false},
		{"not codex - node", "node /usr/lib/codex/index.js", false},
		{"bare codex", "codex", true},
	}

	for _, tc := range testCaseList {
		t.Run(tc.name, func(t *testing.T) {
			got := isCodexProcess(tc.argLine)
			if got != tc.want {
				t.Errorf("isCodexProcess(%q) = %v, want %v", tc.argLine, got, tc.want)
			}
		})
	}
}

func TestExtractResumeSessionID(t *testing.T) {
	testCaseList := []struct {
		name    string
		argLine string
		want    string
	}{
		{"with resume", "codex --resume abc-123-def", "abc-123-def"},
		{"resume at end", "codex --resume sessionID", "sessionID"},
		{"no resume", "codex --model o4-mini", ""},
		{"resume without value", "codex --resume", ""},
		{"empty", "", ""},
		{"resume in middle", "codex --resume myid --model o4-mini", "myid"},
	}

	for _, tc := range testCaseList {
		t.Run(tc.name, func(t *testing.T) {
			got := extractResumeSessionID(tc.argLine)
			if got != tc.want {
				t.Errorf("extractResumeSessionID(%q) = %q, want %q", tc.argLine, got, tc.want)
			}
		})
	}
}
