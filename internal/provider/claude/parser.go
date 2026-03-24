package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/glory0216/taux/internal/model"
)

// ParseSession reads a full JSONL file and returns a SessionDetail with
// aggregated metadata: token usage, tool call counts, timestamps, etc.
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
		messageCount int
		firstSet     bool
		envDetected  bool
		descSet      bool
	)

	ideTagList := []string{
		"<ide_selection", "<ide_opened_file", "<local-command-caveat",
		"<ide_context", "<ide_viewport",
	}

	scanner := bufio.NewScanner(f)
	// Increase buffer size for long lines (1 MB)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	detail.Session.Environment = "cli"

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec model.JSONLRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			// Skip malformed lines
			continue
		}

		messageCount++

		// Track first and last timestamps
		if !rec.Timestamp.IsZero() {
			if !firstSet {
				detail.StartedAt = rec.Timestamp
				firstSet = true
			}
			detail.LastActive = rec.Timestamp
		}

		// Extract metadata from any record that has it
		if rec.Version != "" {
			detail.Version = rec.Version
		}
		if rec.CWD != "" {
			detail.CWD = rec.CWD
		}
		if rec.GitBranch != "" {
			detail.GitBranch = rec.GitBranch
		}
		if rec.TeamName != "" {
			detail.TeamName = rec.TeamName
		}
		if rec.AgentName != "" {
			detail.AgentName = rec.AgentName
		}
		if rec.SessionID != "" {
			detail.ID = rec.SessionID
		}

		// Parse assistant messages for token usage and tool calls
		if rec.Type == "assistant" && len(rec.Message) > 0 {
			parseAssistantRecord(rec.Message, detail)
		}

		// Detect environment + description from user messages (inline, no second file open)
		if rec.Type == "user" && len(rec.Message) > 0 {
			if !envDetected {
				msgStr := string(rec.Message)
				for _, tag := range ideTagList {
					if strings.Contains(msgStr, tag) {
						detail.Session.Environment = "ide"
						envDetected = true
						break
					}
				}
			}

			if !descSet {
				var msg struct {
					Content json.RawMessage `json:"content"`
				}
				if err := json.Unmarshal(rec.Message, &msg); err == nil {
					text := extractTextFromContent(msg.Content)
					if text != "" && !strings.HasPrefix(text, "<teammate-message") && !strings.HasPrefix(text, "<system") {
						text = stripLeadingTags(text)
						text = strings.TrimSpace(text)
						text = strings.ReplaceAll(text, "\n", " ")
						if len([]rune(text)) > 80 {
							text = string([]rune(text)[:77]) + "..."
						}
						detail.Description = text
						descSet = true
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Set context window max based on model
	detail.ContextMax = model.MaxContext(detail.Model)

	detail.MessageCount = messageCount
	detail.Provider = "claude"

	if len(detail.ID) >= 6 {
		detail.ShortID = detail.ID[:6]
	}

	return detail, nil
}

// parseAssistantRecord extracts token usage and tool call info from an
// assistant message's raw JSON.
func parseAssistantRecord(raw json.RawMessage, detail *model.SessionDetail) {
	var am model.AssistantMessage
	if err := json.Unmarshal(raw, &am); err != nil {
		return
	}

	// Set model if present
	if am.Model != "" {
		detail.Model = am.Model
	}

	// Accumulate token usage
	if am.Usage != nil {
		detail.TokenUsage.InputTokens += am.Usage.InputTokens
		detail.TokenUsage.OutputTokens += am.Usage.OutputTokens
		detail.TokenUsage.CacheReadInputTokens += am.Usage.CacheReadInputTokens
		detail.TokenUsage.CacheCreationInputTokens += am.Usage.CacheCreationInputTokens
	}

	// Track last assistant's input_tokens for context window usage
	if am.Usage != nil && am.Usage.InputTokens > 0 {
		detail.ContextUsed = am.Usage.InputTokens
	}

	// Parse content blocks for tool_use
	if len(am.Content) == 0 {
		return
	}

	var blockList []model.ContentBlock
	if err := json.Unmarshal(am.Content, &blockList); err != nil {
		return
	}

	for _, block := range blockList {
		if block.Type == "tool_use" {
			detail.ToolCallCount++
			if block.Name != "" {
				detail.ToolUsage[block.Name]++
			}
			// Extract task list from TodoWrite (last call wins)
			if block.Name == "TodoWrite" && block.Input != nil {
				parseTodoWrite(block.Input, detail)
			}
		}
	}
}

// parseTodoWrite extracts the task list from a TodoWrite tool_use input.
// Each call replaces the entire task list (last write wins).
func parseTodoWrite(input json.RawMessage, detail *model.SessionDetail) {
	var todoInput struct {
		Todos []struct {
			ID      string `json:"id"`
			Content string `json:"content"`
			Status  string `json:"status"` // "pending", "in_progress", "completed"
		} `json:"todos"`
	}
	if err := json.Unmarshal(input, &todoInput); err != nil {
		return
	}
	if len(todoInput.Todos) == 0 {
		return
	}

	taskList := make([]model.Task, 0, len(todoInput.Todos))
	for _, t := range todoInput.Todos {
		taskList = append(taskList, model.Task{
			ID:      t.ID,
			Subject: t.Content,
			Status:  t.Status,
		})
	}
	detail.TaskList = taskList
}

// quickMetadata holds metadata extracted from a single pass over early lines.
type quickMetadata struct {
	Model          string
	Description    string
	Environment    string
	CWD            string
	GitBranch      string
	TeamName       string
	AgentName      string
	FirstTimestamp time.Time
}

// extractQuickMetadata scans the first ~40 lines of a JSONL file once to extract
// model, description, and environment. This replaces three separate file opens.
func extractQuickMetadata(path string) quickMetadata {
	f, err := os.Open(path)
	if err != nil {
		return quickMetadata{Environment: "cli"}
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var meta quickMetadata
	meta.Environment = "cli"

	ideTagList := []string{
		"<ide_selection", "<ide_opened_file", "<local-command-caveat",
		"<ide_context", "<ide_viewport",
	}

	for i := 0; i < 40 && scanner.Scan(); i++ {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec model.JSONLRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		// First timestamp
		if meta.FirstTimestamp.IsZero() && !rec.Timestamp.IsZero() {
			meta.FirstTimestamp = rec.Timestamp
		}

		// CWD from first record that has it
		if meta.CWD == "" && rec.CWD != "" {
			meta.CWD = rec.CWD
		}

		// GitBranch from first record that has it
		if meta.GitBranch == "" && rec.GitBranch != "" {
			meta.GitBranch = rec.GitBranch
		}

		// Model: from first assistant message
		if meta.Model == "" && rec.Type == "assistant" && len(rec.Message) > 0 {
			var am model.AssistantMessage
			if err := json.Unmarshal(rec.Message, &am); err == nil && am.Model != "" {
				if !strings.HasPrefix(am.Model, "<") {
					meta.Model = am.Model
				}
			}
		}

		// Description + Environment: from user messages
		if rec.Type == "user" && len(rec.Message) > 0 {
			// Environment check
			if meta.Environment == "cli" {
				msgStr := string(rec.Message)
				for _, tag := range ideTagList {
					if strings.Contains(msgStr, tag) {
						meta.Environment = "ide"
						break
					}
				}
			}

			// Description: first human message
			if meta.Description == "" {
				var msg struct {
					Content json.RawMessage `json:"content"`
				}
				if err := json.Unmarshal(rec.Message, &msg); err == nil {
					text := extractTextFromContent(msg.Content)
					if text != "" && !strings.HasPrefix(text, "<teammate-message") && !strings.HasPrefix(text, "<system") {
						text = stripLeadingTags(text)
						text = strings.TrimSpace(text)
						text = strings.ReplaceAll(text, "\n", " ")
						if len([]rune(text)) > 80 {
							text = string([]rune(text)[:77]) + "..."
						}
						meta.Description = text
					}
				}
			}
		}

		// TeamName / AgentName from any record
		if meta.TeamName == "" && rec.TeamName != "" {
			meta.TeamName = rec.TeamName
		}
		if meta.AgentName == "" && rec.AgentName != "" {
			meta.AgentName = rec.AgentName
		}

		// Early exit if all found
		if meta.Model != "" && meta.Description != "" && meta.Environment == "ide" && meta.GitBranch != "" {
			break
		}
	}
	return meta
}

// findSessionJSONL returns the JSONL file path and the encoded parent directory
// name for a given session ID. Returns ("", "") if not found.
func findSessionJSONL(sessionID, dataDir string) (filePath, dirName string) {
	pattern := filepath.Join(dataDir, "projects", "*", sessionID+".jsonl")
	matchList, err := filepath.Glob(pattern)
	if err != nil || len(matchList) == 0 {
		return "", ""
	}
	return matchList[0], filepath.Base(filepath.Dir(matchList[0]))
}

// QuickBranchAndCWD returns the git branch and CWD for a session ID
// by scanning the first ~40 lines of its JSONL file. This is fast enough
// for status bar use (~5ms).
func QuickBranchAndCWD(sessionID, dataDir string) (branch, cwd string) {
	filePath, _ := findSessionJSONL(sessionID, dataDir)
	if filePath == "" {
		return "", ""
	}
	meta := extractQuickMetadata(filePath)
	return meta.GitBranch, meta.CWD
}

// QuickTeamInfo returns teamName, agentName, and project for a session ID.
func QuickTeamInfo(sessionID, dataDir string) (teamName, agentName, project string) {
	filePath, dirName := findSessionJSONL(sessionID, dataDir)
	if filePath == "" {
		return "", "", ""
	}
	meta := extractQuickMetadata(filePath)
	projectPath := decodeProjectPath(dirName)
	return meta.TeamName, meta.AgentName, filepath.Base(projectPath)
}

// stripLeadingTags removes XML-like tags from the beginning of text.
// For example: "<ide_selection>The user selected..." → "The user selected..."
// Also strips entire tag blocks like <tag>...</tag> at the start.
func stripLeadingTags(text string) string {
	for {
		text = strings.TrimSpace(text)
		if !strings.HasPrefix(text, "<") {
			break
		}
		// Find closing >
		end := strings.Index(text, ">")
		if end < 0 {
			break
		}
		tagContent := text[1:end]
		// Self-closing or opening tag
		if strings.HasSuffix(tagContent, "/") {
			// Self-closing: <tag/> — just strip it
			text = text[end+1:]
			continue
		}
		// Extract tag name
		fieldList := strings.Fields(tagContent)
		if len(fieldList) == 0 {
			// Empty tag like <> — just strip it
			text = text[end+1:]
			continue
		}
		tagName := fieldList[0]
		// Look for closing tag
		closeTag := "</" + tagName + ">"
		closeIdx := strings.Index(text, closeTag)
		if closeIdx >= 0 {
			// Strip entire <tag>...</tag> block
			text = text[closeIdx+len(closeTag):]
			continue
		}
		// No closing tag found — just strip the opening tag
		text = text[end+1:]
	}
	return text
}

// extractTextFromContent extracts plain text from a user message content field.
// Content can be a JSON string or an array of content blocks.
func extractTextFromContent(raw json.RawMessage) string {
	// Try as plain string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Try as array of content blocks
	var blockList []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blockList); err == nil {
		for _, b := range blockList {
			if b.Type == "text" && b.Text != "" {
				return b.Text
			}
		}
	}
	return ""
}

// ParseSessionSummary reads only the first and last lines of a JSONL file
// for quick metadata extraction without a full parse.
func ParseSessionSummary(path string) (*model.JSONLRecord, *model.JSONLRecord, error) {
	firstLine, err := readFirstLine(path)
	if err != nil {
		return nil, nil, err
	}
	lastLine, err := readLastLine(path)
	if err != nil {
		return nil, nil, err
	}

	var first, last model.JSONLRecord
	if err := json.Unmarshal(firstLine, &first); err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(lastLine, &last); err != nil {
		return nil, nil, err
	}

	return &first, &last, nil
}

// estimateLineCount estimates message count from file size.
// Average JSONL line for Claude Code sessions is ~4-6KB.
// This avoids a full file scan which is the biggest bottleneck for large files.
func estimateLineCount(fileSize int64) int {
	const avgLineSize = 5000 // 5KB average
	if fileSize <= 0 {
		return 0
	}
	est := int(fileSize / avgLineSize)
	if est < 1 {
		est = 1
	}
	return est
}

// countLines returns the number of non-empty lines in a file.
func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			count++
		}
	}
	return count, scanner.Err()
}

// readFirstLine reads the first non-empty line from a file.
func readFirstLine(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) > 0 {
			// Return a copy since scanner reuses the buffer
			result := make([]byte, len(line))
			copy(result, line)
			return result, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, os.ErrNotExist
}

// readLastLine reads the last non-empty line from a file by seeking from the end.
// It uses a dynamic buffer that doubles from 8KB up to 1MB to handle large
// assistant messages (which can be 50KB+ with tool calls).
func readLastLine(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := stat.Size()
	if size == 0 {
		return nil, os.ErrNotExist
	}

	// Try increasingly larger buffers until we capture the full last line
	const maxBufSize = 1024 * 1024 // 1MB
	for bufSize := int64(8192); bufSize <= maxBufSize; bufSize *= 2 {
		if bufSize > size {
			bufSize = size
		}
		buf := make([]byte, bufSize)
		offset := size - bufSize
		n, err := f.ReadAt(buf, offset)
		if err != nil && n == 0 {
			return nil, err
		}
		buf = buf[:n]

		// Trim trailing newlines/whitespace
		end := len(buf)
		for end > 0 && (buf[end-1] == '\n' || buf[end-1] == '\r' || buf[end-1] == ' ') {
			end--
		}
		if end == 0 {
			return nil, os.ErrNotExist
		}

		// Find the start of the last line
		start := end - 1
		for start > 0 && buf[start-1] != '\n' {
			start--
		}

		// If start > 0, we found a newline delimiter — the line is complete
		// If start == 0 and offset == 0, we've read from beginning of file — line is complete
		// If start == 0 and offset > 0, the line may be truncated — try larger buffer
		if start == 0 && offset > 0 {
			if bufSize >= size {
				// Already reading the whole file, can't expand further
			} else {
				continue
			}
		}

		result := make([]byte, end-start)
		copy(result, buf[start:end])
		return result, nil
	}

	// Fallback: read from beginning using scanner (for lines > 1MB)
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var lastLine []byte
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) > 0 {
			lastLine = make([]byte, len(line))
			copy(lastLine, line)
		}
	}
	if lastLine == nil {
		return nil, os.ErrNotExist
	}
	return lastLine, scanner.Err()
}

// TodayTokenSummary holds today's token breakdown.
type TodayTokenSummary struct {
	IOTokens   int // input + output
	CacheRead  int
	CacheWrite int
}

// SumTodayTokens scans JSONL files modified today and sums token usage from
// assistant messages. Only files with mtime matching today's date are parsed,
// keeping this fast (~70ms for typical daily usage).
func SumTodayTokens(dataDir string) TodayTokenSummary {
	today := time.Now().Format("2006-01-02")
	projectsDir := filepath.Join(dataDir, "projects")
	pattern := filepath.Join(projectsDir, "*", "*.jsonl")
	matchList, _ := filepath.Glob(pattern)

	var summary TodayTokenSummary
	for _, m := range matchList {
		stat, err := os.Stat(m)
		if err != nil {
			continue
		}
		if stat.ModTime().Format("2006-01-02") != today {
			continue
		}

		s := sumTokensInFile(m)
		summary.IOTokens += s.IOTokens
		summary.CacheRead += s.CacheRead
		summary.CacheWrite += s.CacheWrite
	}
	return summary
}

// sumTokensInFile reads a JSONL file and sums all token usage fields from
// assistant messages. Uses a lightweight approach: only unmarshals the usage
// field from assistant records.
func sumTokensInFile(path string) TodayTokenSummary {
	f, err := os.Open(path)
	if err != nil {
		return TodayTokenSummary{}
	}
	defer f.Close()

	var summary TodayTokenSummary
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // 4MB buffer for large assistant messages

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec model.JSONLRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec.Type != "assistant" || len(rec.Message) == 0 {
			continue
		}

		var am struct {
			Usage *model.MessageUsage `json:"usage"`
		}
		if err := json.Unmarshal(rec.Message, &am); err != nil || am.Usage == nil {
			continue
		}
		summary.IOTokens += am.Usage.InputTokens + am.Usage.OutputTokens
		summary.CacheRead += am.Usage.CacheReadInputTokens
		summary.CacheWrite += am.Usage.CacheCreationInputTokens
	}
	return summary
}

// sortSessionByLastActive sorts sessions by LastActive in descending order.
func sortSessionByLastActive(sessionList []model.Session) {
	sort.Slice(sessionList, func(i, j int) bool {
		return sessionList[i].LastActive.After(sessionList[j].LastActive)
	})
}
