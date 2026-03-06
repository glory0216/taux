package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

// SearchResult holds a single search match within a session file.
type SearchResult struct {
	Snippet string
}

// SearchInSession checks if a JSONL file contains the query string
// in any user/assistant message content. Returns true on first match.
func SearchInSession(filePath, query string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()

	lower := strings.ToLower(query)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), lower) {
			return true
		}
	}
	return false
}

// SearchSession searches a JSONL file and returns matching snippets.
// Extracts text content from user/assistant messages and returns
// surrounding context (50 chars each side) around each match.
func SearchSession(filePath, query string, maxResult int) []SearchResult {
	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	lower := strings.ToLower(query)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var resultList []SearchResult

	for scanner.Scan() {
		if len(resultList) >= maxResult {
			break
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Quick check before JSON parse
		if !strings.Contains(strings.ToLower(string(line)), lower) {
			continue
		}

		// Extract text content from the record
		text := extractSearchableText(line)
		if text == "" {
			continue
		}

		textLower := strings.ToLower(text)
		idx := strings.Index(textLower, lower)
		if idx < 0 {
			continue
		}

		snippet := extractSnippet(text, idx, len(query), 50)
		resultList = append(resultList, SearchResult{Snippet: snippet})
	}

	return resultList
}

// extractSearchableText pulls human-readable text from a JSONL record.
func extractSearchableText(line []byte) string {
	var rec struct {
		Type    string          `json:"type"`
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(line, &rec); err != nil {
		return ""
	}
	if rec.Type != "user" && rec.Type != "assistant" {
		return ""
	}
	if len(rec.Message) == 0 {
		return ""
	}

	// Try to extract content field
	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(rec.Message, &msg); err != nil {
		return ""
	}

	// Content can be a string
	var s string
	if err := json.Unmarshal(msg.Content, &s); err == nil {
		return s
	}

	// Or an array of blocks
	var blockList []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msg.Content, &blockList); err == nil {
		var parts []string
		for _, b := range blockList {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, " ")
	}

	return ""
}

// extractSnippet extracts text around a match position with context.
func extractSnippet(text string, matchIdx, matchLen, contextLen int) string {
	runes := []rune(text)
	// Convert byte index to approximate rune index
	runeIdx := len([]rune(text[:matchIdx]))

	start := runeIdx - contextLen
	if start < 0 {
		start = 0
	}
	end := runeIdx + matchLen + contextLen
	if end > len(runes) {
		end = len(runes)
	}

	snippet := string(runes[start:end])
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	snippet = strings.TrimSpace(snippet)

	prefix := ""
	suffix := ""
	if start > 0 {
		prefix = "..."
	}
	if end < len(runes) {
		suffix = "..."
	}

	return prefix + snippet + suffix
}
