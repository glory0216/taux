package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/glory0216/taux/internal/model"
)

func newLogsCmd(app *App) *cobra.Command {
	var (
		tail    int
		noTools bool
	)

	cmd := &cobra.Command{
		Use:   "logs <session-id>",
		Short: "Show session conversation logs",
		Long:  "Display the conversation history of a session, showing user messages, assistant responses, and tool calls.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, argList []string) error {
			ctx := cmd.Context()

			fullID, err := resolveSessionID(app, ctx, argList[0])
			if err != nil {
				return err
			}

			// Find session file path
			var filePath, sessionProvider string
			sessionList, _ := app.Registry.AllSession(ctx, emptyFilter())
			for _, s := range sessionList {
				if s.ID == fullID {
					filePath = s.FilePath
					sessionProvider = s.Provider
					break
				}
			}
			if filePath == "" {
				return fmt.Errorf("session file not found: %s", argList[0])
			}
			if sessionProvider != "claude" {
				providerLabel := sessionProvider
				if providerLabel == "" {
					providerLabel = "unknown"
				}
				return fmt.Errorf("taux logs is only supported for Claude sessions (this is a %s session)", providerLabel)
			}

			return printLogs(filePath, tail, noTools)
		},
	}

	cmd.Flags().IntVar(&tail, "tail", 0, "Show only the last N conversation turns")
	cmd.Flags().BoolVar(&noTools, "no-tools", false, "Hide tool call details")

	return cmd
}

type logEntry struct {
	timestamp string
	role      string
	content   string
}

func printLogs(filePath string, tail int, noTools bool) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	// Use a ring buffer when tail > 0 so we never hold more than tail entries
	// in memory regardless of how large the session file is.
	useRing := tail > 0
	var entryList []logEntry
	var ring []logEntry
	var ringPos int
	if useRing {
		ring = make([]logEntry, tail)
	}

	addEntry := func(e logEntry) {
		if useRing {
			ring[ringPos%tail] = e
			ringPos++
		} else {
			entryList = append(entryList, e)
		}
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec model.JSONLRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		// Skip sidechain messages
		if rec.IsSidechain {
			continue
		}

		ts := ""
		if !rec.Timestamp.IsZero() {
			ts = rec.Timestamp.Format("15:04:05")
		}

		switch rec.Type {
		case "user":
			text := extractUserText(rec.Message)
			if text == "" {
				continue
			}
			addEntry(logEntry{timestamp: ts, role: "user", content: text})

		case "assistant":
			for _, e := range extractAssistantEntries(rec.Message, noTools) {
				e.timestamp = ts
				addEntry(e)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read session file: %w", err)
	}

	if useRing {
		// Reconstruct ordered slice from ring buffer
		count := ringPos
		if count > tail {
			count = tail
		}
		start := 0
		if ringPos > tail {
			start = ringPos % tail
		}
		entryList = make([]logEntry, count)
		for i := 0; i < count; i++ {
			entryList[i] = ring[(start+i)%tail]
		}
	}

	for _, e := range entryList {
		roleColor := "\033[36m" // cyan for user
		if e.role == "assistant" {
			roleColor = "\033[33m" // yellow for assistant
		} else if e.role == "tool" {
			roleColor = "\033[2m" // dim for tool
		}
		reset := "\033[0m"

		if e.timestamp != "" {
			fmt.Printf("\033[2m[%s]\033[0m %s%s%s: %s\n", e.timestamp, roleColor, e.role, reset, e.content)
		} else {
			fmt.Printf("%s%s%s: %s\n", roleColor, e.role, reset, e.content)
		}
	}

	return nil
}

// extractUserText pulls the text content from a user message.
func extractUserText(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}

	// User messages can be: { "role": "user", "content": [...] }
	var msg struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return ""
	}
	if msg.Role != "user" {
		return ""
	}

	// Content can be a string or array of content blocks
	var text string
	if err := json.Unmarshal(msg.Content, &text); err == nil {
		return truncateLog(stripTags(text), 500)
	}

	var blocks []model.ContentBlock
	if err := json.Unmarshal(msg.Content, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return truncateLog(stripTags(strings.Join(parts, " ")), 500)
	}

	return ""
}

// extractAssistantEntries pulls text and tool_use entries from an assistant message.
func extractAssistantEntries(raw json.RawMessage, noTools bool) []logEntry {
	if raw == nil {
		return nil
	}

	var msg model.AssistantMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}
	if msg.Role != "assistant" {
		return nil
	}

	var blocks []model.ContentBlock
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return nil
	}

	var entryList []logEntry
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				entryList = append(entryList, logEntry{
					role:    "assistant",
					content: truncateLog(b.Text, 500),
				})
			}
		case "tool_use":
			if noTools {
				continue
			}
			summary := b.Name
			if b.Input != nil {
				// Try to extract key param for common tools
				var inputMap map[string]interface{}
				if err := json.Unmarshal(b.Input, &inputMap); err == nil {
					if fp, ok := inputMap["file_path"].(string); ok {
						summary += "(" + fp + ")"
					} else if cmd, ok := inputMap["command"].(string); ok {
						if len(cmd) > 60 {
							cmd = cmd[:57] + "..."
						}
						summary += "(" + cmd + ")"
					} else if pattern, ok := inputMap["pattern"].(string); ok {
						summary += "(" + pattern + ")"
					}
				}
			}
			entryList = append(entryList, logEntry{
				role:    "tool",
				content: summary,
			})
		}
	}

	return entryList
}

// stripTags removes XML-like tags from text (IDE tags, system tags, etc.)
func stripTags(s string) string {
	// Simple approach: remove <tag>content</tag> patterns for known IDE tags
	for _, tag := range []string{"ide_selection", "ide_opened_file", "local-command-caveat", "ide_context", "ide_viewport", "system-reminder"} {
		for {
			start := strings.Index(s, "<"+tag)
			if start < 0 {
				break
			}
			end := strings.Index(s[start:], "</"+tag+">")
			if end < 0 {
				// Try self-closing or just remove opening tag area
				break
			}
			s = s[:start] + s[start+end+len("</"+tag+">"):]
		}
	}
	return strings.TrimSpace(s)
}

func truncateLog(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ") // collapse whitespace
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-3]) + "..."
	}
	return s
}
