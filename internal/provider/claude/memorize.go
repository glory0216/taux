package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/glory0216/taux/internal/model"
)

// MemorizeSession reads a session JSONL and exports a compact, AI-readable
// markdown summary. The output is structured for quick consumption:
//   - Metadata table
//   - Task (first user message — the goal)
//   - Files modified (extracted from tool calls)
//   - Tool usage summary
//   - Condensed conversation (tool-only turns skipped, messages truncated)
//
// Returns the output file path.
func (p *Provider) MemorizeSession(id string, outDir string) (string, error) {
	path, err := p.findSessionFile(id)
	if err != nil {
		return "", fmt.Errorf("find session %s: %w", id, err)
	}

	// Parse full session for metadata
	detail, err := ParseSession(path)
	if err != nil {
		return "", fmt.Errorf("parse session %s: %w", id, err)
	}

	// Fix model if synthetic — use quick metadata scan
	if detail.Model == "" || strings.HasPrefix(detail.Model, "<") {
		meta := extractQuickMetadata(path)
		if meta.Model != "" {
			detail.Model = meta.Model
		}
	}

	// Derive project info
	projectsDir := filepath.Join(p.dataDir, "projects")
	rel, _ := filepath.Rel(projectsDir, path)
	if rel != "" {
		dirName := filepath.Dir(rel)
		detail.ProjectPath = decodeProjectPath(dirName)
		detail.Project = filepath.Base(detail.ProjectPath)
	}

	// Extract structured conversation data
	convData, err := extractStructuredConversation(path)
	if err != nil {
		return "", fmt.Errorf("extract conversation: %w", err)
	}

	// Build markdown
	md := buildSmartMD(detail, convData)

	// Ensure output directory
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("create memorize dir %s: %w", outDir, err)
	}

	// Filename: YYYY-MM-DD-<short-id>-<project>.md
	shortID := id
	if len(shortID) > 6 {
		shortID = shortID[:6]
	}
	dateStr := detail.StartedAt.Format("2006-01-02")
	if detail.StartedAt.IsZero() {
		dateStr = time.Now().Format("2006-01-02")
	}
	safeName := sanitizeFilename(detail.Project)
	if safeName == "" {
		safeName = "session"
	}
	filename := fmt.Sprintf("%s-%s-%s.md", dateStr, shortID, safeName)
	outPath := filepath.Join(outDir, filename)

	if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
		return "", fmt.Errorf("write memorize file: %w", err)
	}

	return outPath, nil
}

// structuredConversation holds the parsed and categorized session data.
type structuredConversation struct {
	task         string             // first user message (the goal)
	turnList     []conversationTurn // condensed conversation
	fileModified []string           // unique file paths from Edit/Write
}

// conversationTurn is a single meaningful exchange.
type conversationTurn struct {
	role    string
	content string
	toolUse []string
}

// extractStructuredConversation reads the JSONL and extracts a structured summary.
func extractStructuredConversation(path string) (*structuredConversation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data := &structuredConversation{}
	fileSet := make(map[string]bool)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	const maxTurns = 50

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec model.JSONLRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		switch rec.Type {
		case "user":
			text := extractUserText(rec.Message)
			if text == "" {
				continue
			}

			if data.task == "" {
				// First user message = the task/goal
				data.task = truncateRunes(text, 3000)
			} else if len(data.turnList) < maxTurns {
				data.turnList = append(data.turnList, conversationTurn{
					role:    "user",
					content: truncateRunes(text, 300),
				})
			}

		case "assistant":
			if len(rec.Message) == 0 {
				continue
			}
			var am model.AssistantMessage
			if err := json.Unmarshal(rec.Message, &am); err != nil {
				continue
			}

			var textParts []string
			var toolNameList []string
			if len(am.Content) > 0 {
				var blockList []model.ContentBlock
				if err := json.Unmarshal(am.Content, &blockList); err == nil {
					for _, b := range blockList {
						switch b.Type {
						case "text":
							if b.Text != "" {
								textParts = append(textParts, b.Text)
							}
						case "tool_use":
							toolNameList = append(toolNameList, b.Name)
							// Extract file paths from Edit/Write/Read tool calls
							extractFilePaths(b, fileSet)
						}
					}
				}
			}

			text := strings.TrimSpace(strings.Join(textParts, "\n"))

			// Skip tool-only turns with no text (just noise)
			if text == "" && len(toolNameList) > 0 {
				// Merge tool names into previous assistant turn if possible
				if n := len(data.turnList); n > 0 && data.turnList[n-1].role == "assistant" {
					data.turnList[n-1].toolUse = append(data.turnList[n-1].toolUse, toolNameList...)
				}
				continue
			}
			if text == "" {
				continue
			}

			if len(data.turnList) < maxTurns {
				data.turnList = append(data.turnList, conversationTurn{
					role:    "assistant",
					content: truncateRunes(text, 800),
					toolUse: toolNameList,
				})
			}
		}
	}

	// Convert file set to sorted slice
	for f := range fileSet {
		data.fileModified = append(data.fileModified, f)
	}
	sort.Strings(data.fileModified)

	return data, scanner.Err()
}

// extractFilePaths extracts file paths from Edit/Write/Read tool_use blocks.
func extractFilePaths(block model.ContentBlock, fileSet map[string]bool) {
	if block.Name != "Edit" && block.Name != "Write" && block.Name != "Read" {
		return
	}
	if len(block.Input) == 0 {
		return
	}
	var input struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(block.Input, &input); err == nil && input.FilePath != "" {
		fileSet[input.FilePath] = true
	}
}

// extractUserText gets the text content from a user message.
func extractUserText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var msg struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return ""
	}
	text := extractTextFromContent(msg.Content)
	text = stripLeadingTags(text)
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "<teammate-message") || strings.HasPrefix(text, "<system") {
		return ""
	}
	return text
}

// buildSmartMD generates a compact, structured markdown from session data.
func buildSmartMD(detail *model.SessionDetail, conv *structuredConversation) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# Session %s\n\n", detail.ID))

	// Metadata table
	sb.WriteString("## Metadata\n\n")
	sb.WriteString("| Field | Value |\n")
	sb.WriteString("|-------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Project | %s |\n", detail.Project))
	if detail.ProjectPath != "" {
		sb.WriteString(fmt.Sprintf("| Path | `%s` |\n", detail.ProjectPath))
	}
	sb.WriteString(fmt.Sprintf("| Model | %s |\n", detail.Model))
	if detail.Version != "" {
		sb.WriteString(fmt.Sprintf("| CLI Version | %s |\n", detail.Version))
	}
	if detail.Environment != "" {
		env := "CLI (Terminal)"
		if detail.Environment == "ide" {
			env = "IDE (Cursor/VSCode)"
		}
		sb.WriteString(fmt.Sprintf("| Environment | %s |\n", env))
	}
	if detail.CWD != "" {
		sb.WriteString(fmt.Sprintf("| CWD | `%s` |\n", detail.CWD))
	}
	if detail.GitBranch != "" {
		sb.WriteString(fmt.Sprintf("| Branch | %s |\n", detail.GitBranch))
	}
	sb.WriteString(fmt.Sprintf("| Messages | %d |\n", detail.MessageCount))
	sb.WriteString(fmt.Sprintf("| Tool Calls | %d |\n", detail.ToolCallCount))
	tu := detail.TokenUsage
	totalTokens := tu.InputTokens + tu.OutputTokens + tu.CacheReadInputTokens + tu.CacheCreationInputTokens
	sb.WriteString(fmt.Sprintf("| Total Tokens | %d (in=%d out=%d) |\n", totalTokens, tu.InputTokens, tu.OutputTokens))
	if !detail.StartedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("| Started | %s |\n", detail.StartedAt.Format("2006-01-02 15:04:05")))
	}
	if !detail.LastActive.IsZero() {
		sb.WriteString(fmt.Sprintf("| Last Active | %s |\n", detail.LastActive.Format("2006-01-02 15:04:05")))
	}
	sb.WriteString("\n")

	// Task (first user message — the goal of the session)
	if conv.task != "" {
		sb.WriteString("## Task\n\n")
		sb.WriteString(conv.task)
		sb.WriteString("\n\n")
	}

	// Files modified
	if len(conv.fileModified) > 0 {
		sb.WriteString("## Files Modified\n\n")
		for _, f := range conv.fileModified {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		sb.WriteString("\n")
	}

	// Tool usage summary
	if len(detail.ToolUsage) > 0 {
		sb.WriteString("## Tool Usage\n\n")
		// Sort by count descending
		type toolEntry struct {
			name  string
			count int
		}
		var entryList []toolEntry
		for name, count := range detail.ToolUsage {
			entryList = append(entryList, toolEntry{name, count})
		}
		sort.Slice(entryList, func(i, j int) bool {
			return entryList[i].count > entryList[j].count
		})
		for _, e := range entryList {
			sb.WriteString(fmt.Sprintf("- %s: %d\n", e.name, e.count))
		}
		sb.WriteString("\n")
	}

	// Condensed conversation
	if len(conv.turnList) > 0 {
		sb.WriteString("## Conversation\n\n")
		for _, turn := range conv.turnList {
			switch turn.role {
			case "user":
				sb.WriteString("**User**: ")
				sb.WriteString(strings.ReplaceAll(turn.content, "\n", " "))
				sb.WriteString("\n\n")
			case "assistant":
				sb.WriteString("**Assistant**")
				if len(turn.toolUse) > 0 {
					sb.WriteString(fmt.Sprintf(" [%s]", strings.Join(dedup(turn.toolUse), ", ")))
				}
				sb.WriteString(": ")
				sb.WriteString(strings.ReplaceAll(turn.content, "\n", " "))
				sb.WriteString("\n\n")
			}
		}
	}

	return sb.String()
}

// truncateRunes truncates a string to maxLen runes, adding ellipsis.
func truncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// dedup returns unique strings preserving order.
func dedup(list []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range list {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// sanitizeFilename removes characters not safe for filenames.
func sanitizeFilename(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' || r == ' ' {
			return '-'
		}
		return r
	}, s)
	return strings.Trim(s, "-")
}
