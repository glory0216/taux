package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

// ---------------------------------------------------------------------------
// ParseSession tests
// ---------------------------------------------------------------------------

func TestParseSession_ValidFile(t *testing.T) {
	detail, err := ParseSession(testdataPath("valid_session.jsonl"))
	if err != nil {
		t.Fatalf("ParseSession returned error: %v", err)
	}

	// Message count: 5 lines (2 user + 3 assistant)
	if detail.MessageCount != 5 {
		t.Errorf("MessageCount = %d, want 5", detail.MessageCount)
	}

	// Model
	if detail.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", detail.Model, "claude-sonnet-4-6")
	}

	// Token usage: sum of two assistant messages with usage
	// First assistant:  input=100, output=50, cache_read=30, cache_creation=10
	// Second assistant: input=200, output=100, cache_read=50, cache_creation=20
	// Third assistant:  no usage field
	if detail.TokenUsage.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", detail.TokenUsage.InputTokens)
	}
	if detail.TokenUsage.OutputTokens != 150 {
		t.Errorf("OutputTokens = %d, want 150", detail.TokenUsage.OutputTokens)
	}
	if detail.TokenUsage.CacheReadInputTokens != 80 {
		t.Errorf("CacheReadInputTokens = %d, want 80", detail.TokenUsage.CacheReadInputTokens)
	}
	if detail.TokenUsage.CacheCreationInputTokens != 30 {
		t.Errorf("CacheCreationInputTokens = %d, want 30", detail.TokenUsage.CacheCreationInputTokens)
	}

	// Tool calls: 1 Edit + 1 Read + 1 Write = 3
	if detail.ToolCallCount != 3 {
		t.Errorf("ToolCallCount = %d, want 3", detail.ToolCallCount)
	}
	if detail.ToolUsage["Edit"] != 1 {
		t.Errorf("ToolUsage[Edit] = %d, want 1", detail.ToolUsage["Edit"])
	}
	if detail.ToolUsage["Read"] != 1 {
		t.Errorf("ToolUsage[Read] = %d, want 1", detail.ToolUsage["Read"])
	}
	if detail.ToolUsage["Write"] != 1 {
		t.Errorf("ToolUsage[Write] = %d, want 1", detail.ToolUsage["Write"])
	}

	// Session ID and ShortID
	if detail.ID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("ID = %q, want %q", detail.ID, "a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	}
	if detail.ShortID != "a1b2c3" {
		t.Errorf("ShortID = %q, want %q", detail.ShortID, "a1b2c3")
	}

	// Metadata
	if detail.Version != "1.0.5" {
		t.Errorf("Version = %q, want %q", detail.Version, "1.0.5")
	}
	if detail.CWD != "/Users/test/project" {
		t.Errorf("CWD = %q, want %q", detail.CWD, "/Users/test/project")
	}
	if detail.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want %q", detail.GitBranch, "main")
	}
	if detail.Provider != "claude" {
		t.Errorf("Provider = %q, want %q", detail.Provider, "claude")
	}

	// Timestamps
	if detail.StartedAt.IsZero() {
		t.Error("StartedAt is zero, want non-zero")
	}
	if detail.LastActive.IsZero() {
		t.Error("LastActive is zero, want non-zero")
	}
	if !detail.LastActive.After(detail.StartedAt) {
		t.Errorf("LastActive (%v) should be after StartedAt (%v)", detail.LastActive, detail.StartedAt)
	}

	// Environment: no IDE tags in this session
	if detail.Session.Environment != "cli" {
		t.Errorf("Environment = %q, want %q", detail.Session.Environment, "cli")
	}

	// Description: extracted from first user message
	if detail.Description == "" {
		t.Error("Description is empty, want non-empty")
	}
}

func TestParseSession_IDEEnvironment(t *testing.T) {
	detail, err := ParseSession(testdataPath("ide_session.jsonl"))
	if err != nil {
		t.Fatalf("ParseSession returned error: %v", err)
	}

	if detail.Session.Environment != "ide" {
		t.Errorf("Environment = %q, want %q", detail.Session.Environment, "ide")
	}
}

func TestParseSession_EmptyFile(t *testing.T) {
	detail, err := ParseSession(testdataPath("empty.jsonl"))
	if err != nil {
		t.Fatalf("ParseSession returned error: %v", err)
	}

	if detail.MessageCount != 0 {
		t.Errorf("MessageCount = %d, want 0", detail.MessageCount)
	}
	if detail.TokenUsage.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", detail.TokenUsage.InputTokens)
	}
	if detail.TokenUsage.OutputTokens != 0 {
		t.Errorf("OutputTokens = %d, want 0", detail.TokenUsage.OutputTokens)
	}
	if detail.TokenUsage.CacheReadInputTokens != 0 {
		t.Errorf("CacheReadInputTokens = %d, want 0", detail.TokenUsage.CacheReadInputTokens)
	}
	if detail.TokenUsage.CacheCreationInputTokens != 0 {
		t.Errorf("CacheCreationInputTokens = %d, want 0", detail.TokenUsage.CacheCreationInputTokens)
	}
}

func TestParseSession_MalformedLines(t *testing.T) {
	detail, err := ParseSession(testdataPath("malformed.jsonl"))
	if err != nil {
		t.Fatalf("ParseSession returned error: %v", err)
	}

	// Should parse 2 valid lines (skip the malformed one)
	if detail.MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2", detail.MessageCount)
	}
}

func TestParseSession_NonexistentFile(t *testing.T) {
	_, err := ParseSession(testdataPath("does_not_exist.jsonl"))
	if err == nil {
		t.Error("ParseSession should return error for nonexistent file")
	}
}

// ---------------------------------------------------------------------------
// stripLeadingTags tests
// ---------------------------------------------------------------------------

func TestStripLeadingTags_Simple(t *testing.T) {
	input := "<tag>content</tag>rest"
	got := stripLeadingTags(input)
	if got != "rest" {
		t.Errorf("stripLeadingTags(%q) = %q, want %q", input, got, "rest")
	}
}

func TestStripLeadingTags_SelfClosing(t *testing.T) {
	input := "<br/>text"
	got := stripLeadingTags(input)
	if got != "text" {
		t.Errorf("stripLeadingTags(%q) = %q, want %q", input, got, "text")
	}
}

func TestStripLeadingTags_EmptyTag(t *testing.T) {
	input := "<>text"
	// Must NOT panic
	got := stripLeadingTags(input)
	if got != "text" {
		t.Errorf("stripLeadingTags(%q) = %q, want %q", input, got, "text")
	}
}

func TestStripLeadingTags_NoTags(t *testing.T) {
	input := "plain text"
	got := stripLeadingTags(input)
	if got != "plain text" {
		t.Errorf("stripLeadingTags(%q) = %q, want %q", input, got, "plain text")
	}
}

func TestStripLeadingTags_NestedTags(t *testing.T) {
	input := "<a>x</a><b>y</b>rest"
	got := stripLeadingTags(input)
	if got != "rest" {
		t.Errorf("stripLeadingTags(%q) = %q, want %q", input, got, "rest")
	}
}

func TestStripLeadingTags_UnclosedTag(t *testing.T) {
	input := "<tag"
	got := stripLeadingTags(input)
	// No closing '>' found, so the function should return as-is
	if got != "<tag" {
		t.Errorf("stripLeadingTags(%q) = %q, want %q", input, got, "<tag")
	}
}

// ---------------------------------------------------------------------------
// extractTextFromContent tests
// ---------------------------------------------------------------------------

func TestExtractTextFromContent_String(t *testing.T) {
	raw := json.RawMessage(`"hello"`)
	got := extractTextFromContent(raw)
	if got != "hello" {
		t.Errorf("extractTextFromContent(string) = %q, want %q", got, "hello")
	}
}

func TestExtractTextFromContent_Array(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"hello"}]`)
	got := extractTextFromContent(raw)
	if got != "hello" {
		t.Errorf("extractTextFromContent(array) = %q, want %q", got, "hello")
	}
}

func TestExtractTextFromContent_Empty(t *testing.T) {
	raw := json.RawMessage(`null`)
	got := extractTextFromContent(raw)
	if got != "" {
		t.Errorf("extractTextFromContent(null) = %q, want %q", got, "")
	}
}

// ---------------------------------------------------------------------------
// readFirstLine / readLastLine tests
// ---------------------------------------------------------------------------

func TestReadFirstLine_Normal(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(path, []byte("first\nsecond\nthird\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readFirstLine(path)
	if err != nil {
		t.Fatalf("readFirstLine error: %v", err)
	}
	if string(got) != "first" {
		t.Errorf("readFirstLine = %q, want %q", string(got), "first")
	}
}

func TestReadFirstLine_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := readFirstLine(path)
	if err != os.ErrNotExist {
		t.Errorf("readFirstLine(empty) error = %v, want os.ErrNotExist", err)
	}
}

func TestReadLastLine_Normal(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(path, []byte("first\nsecond\nthird\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readLastLine(path)
	if err != nil {
		t.Fatalf("readLastLine error: %v", err)
	}
	if string(got) != "third" {
		t.Errorf("readLastLine = %q, want %q", string(got), "third")
	}
}

func TestReadLastLine_LargeLastLine(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "large.txt")

	// Create a file where the last line is over 100KB
	largeLine := strings.Repeat("x", 110*1024) // 110KB
	content := "first line\n" + largeLine + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readLastLine(path)
	if err != nil {
		t.Fatalf("readLastLine error: %v", err)
	}
	if string(got) != largeLine {
		t.Errorf("readLastLine returned %d bytes, want %d", len(got), len(largeLine))
	}
}

func TestReadLastLine_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := readLastLine(path)
	if err != os.ErrNotExist {
		t.Errorf("readLastLine(empty) error = %v, want os.ErrNotExist", err)
	}
}

// ---------------------------------------------------------------------------
// estimateLineCount tests
// ---------------------------------------------------------------------------

func TestEstimateLineCount(t *testing.T) {
	testCaseList := []struct {
		fileSize int64
		want     int
	}{
		{0, 0},
		{5000, 1},
		{50000, 10},
	}
	for _, tc := range testCaseList {
		got := estimateLineCount(tc.fileSize)
		if got != tc.want {
			t.Errorf("estimateLineCount(%d) = %d, want %d", tc.fileSize, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// extractQuickMetadata tests
// ---------------------------------------------------------------------------

func TestExtractQuickMetadata_ValidFile(t *testing.T) {
	meta := extractQuickMetadata(testdataPath("valid_session.jsonl"))

	if meta.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", meta.Model, "claude-sonnet-4-6")
	}
	if meta.Description == "" {
		t.Error("Description is empty, want non-empty")
	}
	if meta.Environment != "cli" {
		t.Errorf("Environment = %q, want %q", meta.Environment, "cli")
	}
	if meta.CWD != "/Users/test/project" {
		t.Errorf("CWD = %q, want %q", meta.CWD, "/Users/test/project")
	}
	if meta.FirstTimestamp.IsZero() {
		t.Error("FirstTimestamp is zero, want non-zero")
	}
}

func TestExtractQuickMetadata_IDEFile(t *testing.T) {
	meta := extractQuickMetadata(testdataPath("ide_session.jsonl"))

	if meta.Environment != "ide" {
		t.Errorf("Environment = %q, want %q", meta.Environment, "ide")
	}
}
