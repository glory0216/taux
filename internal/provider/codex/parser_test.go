package codex

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func TestParseSession_ValidJSONL(t *testing.T) {
	detail, err := ParseSession(testdataPath("valid_session.jsonl"))
	if err != nil {
		t.Fatalf("ParseSession failed: %v", err)
	}

	// Model should be extracted
	if detail.Model != "o4-mini" {
		t.Errorf("Model = %q, want %q", detail.Model, "o4-mini")
	}

	// Token counts should use the LAST cumulative event (not sum)
	// Last token_count: input=1000, cached_input=200, output=400, reasoning=100
	// Reasoning tokens are priced at the output rate (OpenAI o-series billing),
	// so OutputTokens = output + reasoning = 400 + 100 = 500.
	if detail.TokenUsage.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want %d", detail.TokenUsage.InputTokens, 1000)
	}
	if detail.TokenUsage.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want %d", detail.TokenUsage.OutputTokens, 500)
	}
	if detail.TokenUsage.CacheReadInputTokens != 200 {
		t.Errorf("CacheReadInputTokens = %d, want %d", detail.TokenUsage.CacheReadInputTokens, 200)
	}

	// Message count: 1 user + 4 assistant (1 text message + 3 function_call items)
	// All item.created events with role contribute to message count
	if detail.MessageCount != 5 {
		t.Errorf("MessageCount = %d, want %d", detail.MessageCount, 5)
	}

	// Tool calls: 3 function_call items (edit_file x2 + read_file x1)
	if detail.ToolCallCount != 3 {
		t.Errorf("ToolCallCount = %d, want %d", detail.ToolCallCount, 3)
	}
	if detail.ToolUsage["edit_file"] != 2 {
		t.Errorf("ToolUsage[edit_file] = %d, want %d", detail.ToolUsage["edit_file"], 2)
	}
	if detail.ToolUsage["read_file"] != 1 {
		t.Errorf("ToolUsage[read_file] = %d, want %d", detail.ToolUsage["read_file"], 1)
	}

	// Description from first user message
	if detail.Description != "Fix the login bug in auth.go" {
		t.Errorf("Description = %q, want %q", detail.Description, "Fix the login bug in auth.go")
	}

	// Timestamps
	if detail.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
	if detail.LastActive.IsZero() {
		t.Error("LastActive should not be zero")
	}
	if !detail.LastActive.After(detail.StartedAt) {
		t.Error("LastActive should be after StartedAt")
	}

	// Provider
	if detail.Provider != "codex" {
		t.Errorf("Provider = %q, want %q", detail.Provider, "codex")
	}
}

func TestParseSession_EmptyFile(t *testing.T) {
	detail, err := ParseSession(testdataPath("empty.jsonl"))
	if err != nil {
		t.Fatalf("ParseSession failed: %v", err)
	}

	if detail.MessageCount != 0 {
		t.Errorf("MessageCount = %d, want 0", detail.MessageCount)
	}
	if detail.ToolCallCount != 0 {
		t.Errorf("ToolCallCount = %d, want 0", detail.ToolCallCount)
	}
	if detail.TokenUsage.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", detail.TokenUsage.InputTokens)
	}
}

func TestParseSession_MalformedJSONL(t *testing.T) {
	// Should not panic, should parse what it can
	detail, err := ParseSession(testdataPath("malformed.jsonl"))
	if err != nil {
		t.Fatalf("ParseSession failed: %v", err)
	}

	// Only the valid line should contribute
	if detail.Provider != "codex" {
		t.Errorf("Provider = %q, want %q", detail.Provider, "codex")
	}
}

func TestParseSession_NonexistentFile(t *testing.T) {
	_, err := ParseSession(testdataPath("nonexistent.jsonl"))
	if err == nil {
		t.Error("ParseSession should fail for nonexistent file")
	}
}
