package codex

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/glory0216/taux/internal/model"
)

// ParseSession reads a full JSONL file and returns a SessionDetail with
// aggregated metadata: token usage, tool call counts, timestamps, etc.
//
// Codex JSONL events have a {type, payload} structure. Token counts in
// token_count events are cumulative — the last event's values are the totals.
func ParseSession(path string) (*model.SessionDetail, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	detail := &model.SessionDetail{
		ToolUsage: make(map[string]int),
	}

	var (
		firstTimestamp  float64
		lastTimestamp   float64
		lastTokenCount tokenCountPayload
		userMsgCount   int
		assistMsgCount int
	)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event codexEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		// Track timestamps
		ts := event.Timestamp
		if ts == 0 {
			ts = event.CreatedAt
		}
		if ts > 0 {
			if firstTimestamp == 0 {
				firstTimestamp = ts
			}
			lastTimestamp = ts
		}

		// Extract model from response events
		if len(event.Payload) > 0 {
			var payload struct {
				Model    string `json:"model"`
				Response struct {
					Model string `json:"model"`
				} `json:"response"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err == nil {
				if payload.Model != "" {
					detail.Model = payload.Model
				}
				if payload.Response.Model != "" {
					detail.Model = payload.Response.Model
				}
			}
		}
		if event.Model != "" {
			detail.Model = event.Model
		}

		switch {
		case event.Type == "token_count" || strings.HasSuffix(event.Type, ".token_count"):
			// Token counts are cumulative — just keep the latest
			var tc tokenCountPayload
			if err := json.Unmarshal(event.Payload, &tc); err == nil {
				lastTokenCount = tc
			}

		case strings.Contains(event.Type, "item.created") || event.Type == "input":
			// Count user/assistant messages and tool calls from item creation
			var payload struct {
				Item struct {
					Role    string `json:"role"`
					Type    string `json:"type"`
					Name    string `json:"name"`
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"item"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err == nil {
				switch payload.Item.Role {
				case "user":
					userMsgCount++
					// First user message → description
					if detail.Description == "" {
						for _, c := range payload.Item.Content {
							if (c.Type == "input_text" || c.Type == "text") && c.Text != "" {
								text := strings.TrimSpace(c.Text)
								text = strings.ReplaceAll(text, "\n", " ")
								if len([]rune(text)) > 80 {
									text = string([]rune(text)[:77]) + "..."
								}
								detail.Description = text
								break
							}
						}
					}
				case "assistant":
					assistMsgCount++
				}

				// Tool use detection from item type
				if payload.Item.Type == "function_call" || payload.Item.Type == "tool_use" {
					detail.ToolCallCount++
					if payload.Item.Name != "" {
						detail.ToolUsage[payload.Item.Name]++
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Apply cumulative token counts (last event = totals).
	// Reasoning tokens (o1/o3/o4 series) are priced at the output rate by OpenAI,
	// so they are added to OutputTokens for accurate cost calculation.
	detail.TokenUsage = model.TokenUsage{
		InputTokens:          lastTokenCount.Input,
		OutputTokens:         lastTokenCount.Output + lastTokenCount.Reasoning,
		CacheReadInputTokens: lastTokenCount.CachedInput,
	}

	detail.MessageCount = userMsgCount + assistMsgCount

	// Convert timestamps
	if firstTimestamp > 0 {
		detail.StartedAt = floatToTime(firstTimestamp)
	}
	if lastTimestamp > 0 {
		detail.LastActive = floatToTime(lastTimestamp)
	}

	detail.Provider = "codex"
	detail.Environment = "cli"

	return detail, nil
}

// codexEvent represents a single event line in Codex JSONL.
type codexEvent struct {
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Model     string          `json:"model,omitempty"`
	Timestamp float64         `json:"timestamp,omitempty"`
	CreatedAt float64         `json:"created_at,omitempty"`
}

// tokenCountPayload represents the cumulative token counts.
type tokenCountPayload struct {
	Input       int `json:"input"`
	CachedInput int `json:"cached_input"`
	Output      int `json:"output"`
	Reasoning   int `json:"reasoning"`
	Total       int `json:"total"`
}

// floatToTime converts a Unix timestamp (float64 seconds) to time.Time.
func floatToTime(ts float64) (t time.Time) {
	if ts <= 0 {
		return
	}
	sec := int64(ts)
	nsec := int64((ts - float64(sec)) * 1e9)
	return time.Unix(sec, nsec)
}
