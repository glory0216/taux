package cursor

import (
	"testing"
)

func TestParseSession_Found(t *testing.T) {
	tmpDir, db := createTestDB(t)

	composerID := "parse-target-id-999"

	cd := ComposerData{
		ComposerID:    composerID,
		Name:          "Parsed Session",
		CreatedAt:     1700000000000,
		LastUpdatedAt: 1700001000000,
		Status:        "active",
		IsAgentic:     true,
		FullConversationHeadersOnly: []BubbleHeader{
			{BubbleID: "b1", Type: 1},
			{BubbleID: "b2", Type: 2},
		},
	}
	insertComposer(t, db, cd)

	b1 := BubbleData{
		BubbleID: "b1", Type: 1, Text: "User message",
		TokenCount: &BubbleTokenCount{InputTokens: 100, OutputTokens: 0},
	}
	b2 := BubbleData{
		BubbleID: "b2", Type: 2, Text: "AI response",
		TokenCount: &BubbleTokenCount{InputTokens: 50, OutputTokens: 200},
	}
	insertBubble(t, db, composerID, b1)
	insertBubble(t, db, composerID, b2)
	db.Close()

	detail, err := ParseSession(tmpDir, composerID)
	if err != nil {
		t.Fatalf("ParseSession error: %v", err)
	}
	if detail == nil {
		t.Fatal("ParseSession returned nil for existing composer")
	}

	if detail.ID != composerID {
		t.Errorf("expected ID %q, got %q", composerID, detail.ID)
	}
	if detail.ShortID != "parse-" {
		t.Errorf("expected ShortID 'parse-', got %q", detail.ShortID)
	}
	if detail.Provider != "cursor" {
		t.Errorf("expected Provider 'cursor', got %q", detail.Provider)
	}
	if detail.Description != "Parsed Session" {
		t.Errorf("expected Description 'Parsed Session', got %q", detail.Description)
	}
	if detail.MessageCount != 2 {
		t.Errorf("expected MessageCount 2, got %d", detail.MessageCount)
	}

	// Token usage
	if detail.TokenUsage.InputTokens != 150 {
		t.Errorf("expected InputTokens 150, got %d", detail.TokenUsage.InputTokens)
	}
	if detail.TokenUsage.OutputTokens != 200 {
		t.Errorf("expected OutputTokens 200, got %d", detail.TokenUsage.OutputTokens)
	}

	// Tool call count (AI messages = type 2)
	if detail.ToolCallCount != 1 {
		t.Errorf("expected ToolCallCount 1, got %d", detail.ToolCallCount)
	}
}

func TestParseSession_NotFound(t *testing.T) {
	tmpDir, db := createTestDB(t)
	db.Close()

	detail, err := ParseSession(tmpDir, "nonexistent-id")
	if err != nil {
		t.Fatalf("ParseSession error: %v", err)
	}
	if detail != nil {
		t.Fatal("ParseSession should return nil for nonexistent ID")
	}
}

func TestParseSession_NoDB(t *testing.T) {
	tmpDir := t.TempDir()

	detail, err := ParseSession(tmpDir, "some-id")
	if err != nil {
		t.Fatalf("ParseSession should not error for missing DB, got: %v", err)
	}
	if detail != nil {
		t.Fatal("ParseSession should return nil for missing DB")
	}
}

// --- extractDescription tests ---

func TestExtractDescription_Name(t *testing.T) {
	cd := &ComposerData{
		ComposerID: "test-id",
		Name:       "My Named Session",
		Text:       "Some legacy text",
	}
	bubbleList := []BubbleData{
		{BubbleID: "b1", Type: 1, Text: "User said something"},
	}

	desc := extractDescription(cd, bubbleList)
	if desc != "My Named Session" {
		t.Errorf("expected 'My Named Session', got %q", desc)
	}
}

func TestExtractDescription_FirstUserBubble(t *testing.T) {
	cd := &ComposerData{
		ComposerID: "test-id",
		Name:       "", // No name
		Text:       "Legacy text",
	}
	bubbleList := []BubbleData{
		{BubbleID: "b1", Type: 2, Text: "AI response first"},
		{BubbleID: "b2", Type: 1, Text: "First user message"},
		{BubbleID: "b3", Type: 1, Text: "Second user message"},
	}

	desc := extractDescription(cd, bubbleList)
	if desc != "First user message" {
		t.Errorf("expected 'First user message', got %q", desc)
	}
}

func TestExtractDescription_FirstUserBubble_RawTextFallback(t *testing.T) {
	cd := &ComposerData{
		ComposerID: "test-id",
		Name:       "",
	}
	bubbleList := []BubbleData{
		{BubbleID: "b1", Type: 1, Text: "", RawText: "Raw user message"},
	}

	desc := extractDescription(cd, bubbleList)
	if desc != "Raw user message" {
		t.Errorf("expected 'Raw user message', got %q", desc)
	}
}

func TestExtractDescription_Legacy(t *testing.T) {
	cd := &ComposerData{
		ComposerID: "test-id",
		Name:       "",
		Text:       "Legacy conversation text",
	}
	// No user bubbles
	bubbleList := []BubbleData{
		{BubbleID: "b1", Type: 2, Text: "AI only"},
	}

	desc := extractDescription(cd, bubbleList)
	if desc != "Legacy conversation text" {
		t.Errorf("expected 'Legacy conversation text', got %q", desc)
	}
}

func TestExtractDescription_Fallback(t *testing.T) {
	cd := &ComposerData{
		ComposerID: "path/to/some-composer-id",
		Name:       "",
		Text:       "",
	}
	var bubbleList []BubbleData

	desc := extractDescription(cd, bubbleList)
	// filepath.Base("path/to/some-composer-id") = "some-composer-id"
	if desc != "some-composer-id" {
		t.Errorf("expected 'some-composer-id', got %q", desc)
	}
}

// --- extractTokenUsage tests ---

func TestExtractTokenUsage_WithTokens(t *testing.T) {
	bubbleList := []BubbleData{
		{
			BubbleID:   "b1",
			Type:       1,
			TokenCount: &BubbleTokenCount{InputTokens: 100, OutputTokens: 0},
		},
		{
			BubbleID:   "b2",
			Type:       2,
			TokenCount: &BubbleTokenCount{InputTokens: 50, OutputTokens: 300},
		},
	}

	usage := extractTokenUsage(bubbleList)
	if usage.InputTokens != 150 {
		t.Errorf("expected InputTokens 150, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 300 {
		t.Errorf("expected OutputTokens 300, got %d", usage.OutputTokens)
	}
}

func TestExtractTokenUsage_NilTokenCount(t *testing.T) {
	bubbleList := []BubbleData{
		{BubbleID: "b1", Type: 1, Text: "no tokens"},
		{BubbleID: "b2", Type: 2, Text: "also no tokens"},
	}

	usage := extractTokenUsage(bubbleList)
	if usage.InputTokens != 0 {
		t.Errorf("expected InputTokens 0, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 0 {
		t.Errorf("expected OutputTokens 0, got %d", usage.OutputTokens)
	}
}

// --- truncateUTF8 tests ---

func TestTruncateUTF8_Short(t *testing.T) {
	s := "Hello world"
	result := truncateUTF8(s, 80)
	if result != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", result)
	}
}

func TestTruncateUTF8_Long(t *testing.T) {
	// 90 chars
	s := "This is a very long string that should be truncated because it exceeds the maximum length limit."
	result := truncateUTF8(s, 80)

	runes := []rune(result)
	// maxLen=80, so 77 chars + "..." = 80 runes
	if len(runes) != 80 {
		t.Errorf("expected 80 runes, got %d", len(runes))
	}
	if result[len(result)-3:] != "..." {
		t.Errorf("expected trailing '...', got %q", result[len(result)-3:])
	}
}

func TestTruncateUTF8_Multibyte(t *testing.T) {
	// Korean string: each character is 3 bytes in UTF-8, 1 rune each
	// "안녕하세요 세계입니다 테스트 문자열입니다 매우 긴 한국어 문자열을 잘라야 합니다 이것은 테스트입니다"
	s := "안녕하세요 세계입니다 테스트 문자열입니다 매우 긴 한국어 문자열을 잘라야 합니다 이것은 테스트입니다 추가 문자열"
	result := truncateUTF8(s, 30)

	runes := []rune(result)
	// 27 runes + "..." = 30 runes
	if len(runes) != 30 {
		t.Errorf("expected 30 runes, got %d", len(runes))
	}

	// Ensure no broken bytes: result should be valid UTF-8
	for i, r := range result {
		if r == '\uFFFD' {
			t.Errorf("invalid UTF-8 at byte position %d", i)
		}
	}

	// Should end with "..."
	runeSlice := []rune(result)
	last3 := string(runeSlice[len(runeSlice)-3:])
	if last3 != "..." {
		t.Errorf("expected trailing '...', got %q", last3)
	}
}
