package gemini

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func TestParseSession_ObjectFormat(t *testing.T) {
	detail, err := ParseSession(testdataPath("object_format.json"))
	if err != nil {
		t.Fatalf("ParseSession failed: %v", err)
	}

	// 2 user + 2 model = 4 messages
	if detail.MessageCount != 4 {
		t.Errorf("MessageCount = %d, want %d", detail.MessageCount, 4)
	}

	// Model from usageMetadata
	if detail.Model != "gemini-2.5-pro" {
		t.Errorf("Model = %q, want %q", detail.Model, "gemini-2.5-pro")
	}

	// Token usage (accumulated from 2 model messages)
	// First: prompt=100, candidates=250, cached=50
	// Second: prompt=150, candidates=300, cached=0
	if detail.TokenUsage.InputTokens != 250 {
		t.Errorf("InputTokens = %d, want %d", detail.TokenUsage.InputTokens, 250)
	}
	if detail.TokenUsage.OutputTokens != 550 {
		t.Errorf("OutputTokens = %d, want %d", detail.TokenUsage.OutputTokens, 550)
	}
	if detail.TokenUsage.CacheReadInputTokens != 50 {
		t.Errorf("CacheReadInputTokens = %d, want %d", detail.TokenUsage.CacheReadInputTokens, 50)
	}

	// Description from first user message
	if detail.Description != "Explain how goroutines work in Go" {
		t.Errorf("Description = %q, want %q", detail.Description, "Explain how goroutines work in Go")
	}

	// Tool calls: 1 functionCall (run_code)
	if detail.ToolCallCount != 1 {
		t.Errorf("ToolCallCount = %d, want %d", detail.ToolCallCount, 1)
	}
	if detail.ToolUsage["run_code"] != 1 {
		t.Errorf("ToolUsage[run_code] = %d, want %d", detail.ToolUsage["run_code"], 1)
	}

	if detail.Provider != "gemini" {
		t.Errorf("Provider = %q, want %q", detail.Provider, "gemini")
	}
}

func TestParseSession_ArrayFormat(t *testing.T) {
	detail, err := ParseSession(testdataPath("array_format.json"))
	if err != nil {
		t.Fatalf("ParseSession failed: %v", err)
	}

	if detail.MessageCount != 2 {
		t.Errorf("MessageCount = %d, want %d", detail.MessageCount, 2)
	}
	if detail.Description != "What is 2+2?" {
		t.Errorf("Description = %q, want %q", detail.Description, "What is 2+2?")
	}
}

func TestParseSession_EmptyJSON(t *testing.T) {
	detail, err := ParseSession(testdataPath("empty.json"))
	if err != nil {
		t.Fatalf("ParseSession failed: %v", err)
	}

	if detail.MessageCount != 0 {
		t.Errorf("MessageCount = %d, want 0", detail.MessageCount)
	}
	if detail.ToolCallCount != 0 {
		t.Errorf("ToolCallCount = %d, want 0", detail.ToolCallCount)
	}
}

func TestParseSession_NonexistentFile(t *testing.T) {
	_, err := ParseSession(testdataPath("nonexistent.json"))
	if err == nil {
		t.Error("ParseSession should fail for nonexistent file")
	}
}

func TestParseSession_SnakeCaseTokens(t *testing.T) {
	detail, err := ParseSession(testdataPath("snake_case_tokens.json"))
	if err != nil {
		t.Fatalf("ParseSession failed: %v", err)
	}

	// snake_case variant: prompt_tokens=10, output_tokens=20, cached_tokens=5
	if detail.TokenUsage.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want %d", detail.TokenUsage.InputTokens, 10)
	}
	if detail.TokenUsage.OutputTokens != 20 {
		t.Errorf("OutputTokens = %d, want %d", detail.TokenUsage.OutputTokens, 20)
	}
	if detail.TokenUsage.CacheReadInputTokens != 5 {
		t.Errorf("CacheReadInputTokens = %d, want %d", detail.TokenUsage.CacheReadInputTokens, 5)
	}
}

func TestGeminiUsage_UnmarshalJSON_CamelCase(t *testing.T) {
	data := `{"promptTokenCount":100,"candidatesTokenCount":200,"cachedContentTokenCount":50,"totalTokenCount":350}`
	var u geminiUsage
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if u.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want 100", u.PromptTokens)
	}
	if u.CompletionTokens != 200 {
		t.Errorf("CompletionTokens = %d, want 200", u.CompletionTokens)
	}
	if u.CachedTokens != 50 {
		t.Errorf("CachedTokens = %d, want 50", u.CachedTokens)
	}
}

func TestGeminiUsage_UnmarshalJSON_SnakeCase(t *testing.T) {
	data := `{"prompt_tokens":80,"output_tokens":160,"cached_tokens":30}`
	var u geminiUsage
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if u.PromptTokens != 80 {
		t.Errorf("PromptTokens = %d, want 80", u.PromptTokens)
	}
	if u.CompletionTokens != 160 {
		t.Errorf("CompletionTokens = %d, want 160", u.CompletionTokens)
	}
	if u.CachedTokens != 30 {
		t.Errorf("CachedTokens = %d, want 30", u.CachedTokens)
	}
}

func TestGeminiUsage_UnmarshalJSON_Empty(t *testing.T) {
	data := `{}`
	var u geminiUsage
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if u.PromptTokens != 0 || u.CompletionTokens != 0 || u.CachedTokens != 0 {
		t.Error("all token counts should be 0 for empty object")
	}
}

func TestExtractMessageText_Parts(t *testing.T) {
	msg := geminiMessage{
		Parts: []geminiPart{
			{Text: "Hello from parts"},
		},
	}
	text := extractMessageText(msg)
	if text != "Hello from parts" {
		t.Errorf("text = %q, want %q", text, "Hello from parts")
	}
}

func TestExtractMessageText_ContentString(t *testing.T) {
	msg := geminiMessage{
		Content: json.RawMessage(`"Hello from content"`),
	}
	text := extractMessageText(msg)
	if text != "Hello from content" {
		t.Errorf("text = %q, want %q", text, "Hello from content")
	}
}

func TestExtractMessageText_ContentBlocks(t *testing.T) {
	msg := geminiMessage{
		Content: json.RawMessage(`[{"type":"text","text":"Hello from blocks"}]`),
	}
	text := extractMessageText(msg)
	if text != "Hello from blocks" {
		t.Errorf("text = %q, want %q", text, "Hello from blocks")
	}
}

func TestExtractMessageText_Empty(t *testing.T) {
	msg := geminiMessage{}
	text := extractMessageText(msg)
	if text != "" {
		t.Errorf("text = %q, want empty", text)
	}
}
