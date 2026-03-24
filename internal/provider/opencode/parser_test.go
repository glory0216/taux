package opencode

import (
	"path/filepath"
	"testing"
)

func TestParseSession_aggregatesTokenUsage(t *testing.T) {
	dataDir := testdataDir()
	detail, err := ParseSession(dataDir, "ses_def456")
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}

	// msg_002: in=1200, out=850, cacheCreate=300, cacheRead=200
	// msg_004: in=2000, out=1200, cacheCreate=0,  cacheRead=900
	wantInput := 1200 + 2000
	wantOutput := 850 + 1200
	wantCacheCreate := 300 + 0
	wantCacheRead := 200 + 900

	if detail.TokenUsage.InputTokens != wantInput {
		t.Errorf("InputTokens = %d, want %d", detail.TokenUsage.InputTokens, wantInput)
	}
	if detail.TokenUsage.OutputTokens != wantOutput {
		t.Errorf("OutputTokens = %d, want %d", detail.TokenUsage.OutputTokens, wantOutput)
	}
	if detail.TokenUsage.CacheCreationInputTokens != wantCacheCreate {
		t.Errorf("CacheCreationInputTokens = %d, want %d", detail.TokenUsage.CacheCreationInputTokens, wantCacheCreate)
	}
	if detail.TokenUsage.CacheReadInputTokens != wantCacheRead {
		t.Errorf("CacheReadInputTokens = %d, want %d", detail.TokenUsage.CacheReadInputTokens, wantCacheRead)
	}
}

func TestParseSession_extractsModel(t *testing.T) {
	detail, err := ParseSession(testdataDir(), "ses_def456")
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}
	if detail.Model != "claude-sonnet-4-5" {
		t.Errorf("Model = %q, want %q", detail.Model, "claude-sonnet-4-5")
	}
}

func TestParseSession_countsMessages(t *testing.T) {
	detail, err := ParseSession(testdataDir(), "ses_def456")
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}
	// 4 messages total (2 user + 2 assistant)
	if detail.MessageCount != 4 {
		t.Errorf("MessageCount = %d, want %d", detail.MessageCount, 4)
	}
}

func TestParseSession_nonexistentSession(t *testing.T) {
	_, err := ParseSession(testdataDir(), "ses_notexist")
	if err == nil {
		t.Error("expected error for nonexistent session, got nil")
	}
}

func TestParseSession_usesSessionFileForMetadata(t *testing.T) {
	detail, err := ParseSession(testdataDir(), "ses_def456")
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}
	// title from session JSON
	if detail.Description != "Add authentication feature" {
		t.Errorf("Description = %q, want title from session file", detail.Description)
	}
	// directory from session JSON
	if detail.CWD != "/Users/user/projects/myapp" {
		t.Errorf("CWD = %q, want %q", detail.CWD, "/Users/user/projects/myapp")
	}
}

func TestParseSession_missingMessageDir(t *testing.T) {
	// ses_ghi789 has no message files — should still return valid (empty) detail
	detail, err := ParseSession(testdataDir(), "ses_ghi789")
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}
	if detail == nil {
		t.Fatal("expected non-nil detail")
	}
	if detail.TokenUsage.InputTokens != 0 {
		t.Errorf("expected 0 input tokens for session with no messages, got %d", detail.TokenUsage.InputTokens)
	}
}

func init() {
	// Ensure testdata path is available
	_ = filepath.Join("testdata", "storage")
}
