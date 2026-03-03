package gemini

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/glory0216/taux/internal/model"
)

// ParseSession reads a Gemini session JSON file and returns a SessionDetail
// with aggregated metadata: token usage, tool call counts, etc.
//
// Uses json.NewDecoder for streaming to avoid loading the entire file into memory.
func ParseSession(path string) (*model.SessionDetail, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	detail := &model.SessionDetail{
		ToolUsage: make(map[string]int),
	}

	messageList, err := streamMessageList(f)
	if err != nil {
		// File exists but has no parseable content — return empty detail
		return detail, nil
	}

	var (
		userMsgCount   int
		assistMsgCount int
	)

	for _, raw := range messageList {
		var msg geminiMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		switch msg.Role {
		case "user":
			userMsgCount++
			if detail.Description == "" {
				text := extractMessageText(msg)
				if text != "" {
					text = strings.TrimSpace(text)
					text = strings.ReplaceAll(text, "\n", " ")
					if len([]rune(text)) > 80 {
						text = string([]rune(text)[:77]) + "..."
					}
					detail.Description = text
				}
			}
		case "model", "assistant":
			assistMsgCount++
		}

		// Model extraction
		if msg.Model != "" {
			detail.Model = msg.Model
		}
		if msg.ModelID != "" {
			detail.Model = msg.ModelID
		}

		// Token usage (accumulate from each message)
		if msg.UsageMetadata != nil {
			detail.TokenUsage.InputTokens += msg.UsageMetadata.PromptTokens
			detail.TokenUsage.OutputTokens += msg.UsageMetadata.CompletionTokens
			detail.TokenUsage.CacheReadInputTokens += msg.UsageMetadata.CachedTokens

			if msg.UsageMetadata.Model != "" {
				detail.Model = msg.UsageMetadata.Model
			}
		}

		// Tool calls from parts
		for _, part := range msg.Parts {
			if part.FunctionCall != nil {
				detail.ToolCallCount++
				if part.FunctionCall.Name != "" {
					detail.ToolUsage[part.FunctionCall.Name]++
				}
			}
		}

		// Tool calls from tool_calls field
		for _, tc := range msg.ToolCallList {
			detail.ToolCallCount++
			if tc.Function.Name != "" {
				detail.ToolUsage[tc.Function.Name]++
			}
		}
	}

	detail.MessageCount = userMsgCount + assistMsgCount
	detail.Provider = "gemini"
	detail.Environment = "cli"

	return detail, nil
}

// streamMessageList reads messages from a JSON file using streaming decoder.
// Supports two formats:
//   - Object with "messages" key: {"messages": [...]}
//   - Direct array: [...]
func streamMessageList(r io.ReadSeeker) ([]json.RawMessage, error) {
	dec := json.NewDecoder(r)

	// Peek at first token to determine format
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	switch tok {
	case json.Delim('{'):
		// Object format: find "messages" key and stream its array
		return streamObjectMessages(dec)
	case json.Delim('['):
		// Array format: stream elements directly
		return streamArray(dec)
	default:
		return nil, io.ErrUnexpectedEOF
	}
}

// streamObjectMessages reads through an object to find "messages" array and streams it.
func streamObjectMessages(dec *json.Decoder) ([]json.RawMessage, error) {
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			continue
		}
		if key == "messages" {
			// Next token should be '[' — start of messages array
			tok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			if tok != json.Delim('[') {
				// Not an array, skip
				continue
			}
			return streamArray(dec)
		}
		// Skip values for other keys
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// streamArray reads array elements one at a time from a decoder
// that has already consumed the opening '['.
func streamArray(dec *json.Decoder) ([]json.RawMessage, error) {
	var result []json.RawMessage
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			break
		}
		result = append(result, raw)
	}
	return result, nil
}

// geminiMessage represents a message in the Gemini session format.
type geminiMessage struct {
	Role          string          `json:"role"`
	Content       json.RawMessage `json:"content"`
	Model         string          `json:"model,omitempty"`
	ModelID       string          `json:"model_id,omitempty"`
	Parts         []geminiPart    `json:"parts,omitempty"`
	ToolCallList  []geminiToolCall `json:"tool_calls,omitempty"`
	UsageMetadata *geminiUsage    `json:"usageMetadata,omitempty"`
}

type geminiPart struct {
	Text         string              `json:"text,omitempty"`
	FunctionCall *geminiFunctionCall `json:"functionCall,omitempty"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type geminiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

// geminiUsage tracks token usage with defensive multi-field support.
// Gemini API uses camelCase (promptTokenCount), but CLI storage format
// may differ. We support both variants via custom UnmarshalJSON.
type geminiUsage struct {
	PromptTokens     int    // input tokens
	CompletionTokens int    // output tokens
	CachedTokens     int    // cache read tokens
	TotalTokens      int
	Model            string
}

func (u *geminiUsage) UnmarshalJSON(data []byte) error {
	// Decode into a flexible map to handle multiple field name variants
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Helper: try multiple keys, return first non-zero int
	tryInt := func(keyList ...string) int {
		for _, k := range keyList {
			if v, ok := raw[k]; ok {
				var n int
				if json.Unmarshal(v, &n) == nil && n > 0 {
					return n
				}
			}
		}
		return 0
	}

	tryString := func(keyList ...string) string {
		for _, k := range keyList {
			if v, ok := raw[k]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil && s != "" {
					return s
				}
			}
		}
		return ""
	}

	u.PromptTokens = tryInt("promptTokenCount", "prompt_tokens", "prompt_token_count", "inputTokens", "input_tokens")
	u.CompletionTokens = tryInt("candidatesTokenCount", "completion_tokens", "candidates_token_count", "outputTokens", "output_tokens", "generationTokenCount")
	u.CachedTokens = tryInt("cachedContentTokenCount", "cached_tokens", "cached_content_token_count", "cachedInputTokenCount")
	u.TotalTokens = tryInt("totalTokenCount", "total_tokens", "total_token_count")
	u.Model = tryString("model")

	return nil
}

// extractMessageText extracts plain text content from a Gemini message.
func extractMessageText(msg geminiMessage) string {
	// Try parts first (Gemini native format)
	for _, p := range msg.Parts {
		if p.Text != "" {
			return p.Text
		}
	}

	// Try content as string
	var s string
	if err := json.Unmarshal(msg.Content, &s); err == nil && s != "" {
		return s
	}

	// Try content as array of objects with text
	var blockList []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msg.Content, &blockList); err == nil {
		for _, b := range blockList {
			if b.Text != "" {
				return b.Text
			}
		}
	}

	return ""
}
