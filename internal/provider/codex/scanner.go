package codex

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

// ScanSession scans the Codex sessions directory for JSONL session files.
// Directory structure: ~/.codex/sessions/YYYY/MM/DD/rollout-{sessionID}.jsonl
func ScanSession(dataDir string) ([]model.Session, error) {
	sessionsDir := filepath.Join(dataDir, "sessions")
	pattern := filepath.Join(sessionsDir, "*", "*", "*", "*.jsonl")
	matchList, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var sessionList []model.Session
	for _, match := range matchList {
		session, err := scanSingleSession(match, sessionsDir)
		if err != nil {
			continue
		}
		sessionList = append(sessionList, *session)
	}

	sort.Slice(sessionList, func(i, j int) bool {
		return sessionList[i].LastActive.After(sessionList[j].LastActive)
	})
	return sessionList, nil
}

// scanSingleSession extracts lightweight session info from a single JSONL file.
func scanSingleSession(path string, sessionsDir string) (*model.Session, error) {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".jsonl") {
		return nil, os.ErrInvalid
	}

	// Extract session ID: strip "rollout-" prefix and ".jsonl" suffix
	sessionID := strings.TrimSuffix(base, ".jsonl")
	sessionID = strings.TrimPrefix(sessionID, "rollout-")
	if sessionID == "" {
		return nil, os.ErrInvalid
	}

	// Extract date from directory path: sessions/YYYY/MM/DD/file.jsonl
	rel, err := filepath.Rel(sessionsDir, path)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Single-pass metadata extraction: timestamps + model + description
	meta := extractQuickMetadata(path)

	firstTime := meta.firstTime
	lastTime := meta.lastTime
	if firstTime.IsZero() {
		firstTime = stat.ModTime()
	}
	if lastTime.IsZero() {
		lastTime = stat.ModTime()
	}

	// Build short ID
	shortID := sessionID
	if len(shortID) > 6 {
		shortID = shortID[:6]
	}

	// Project from date path
	dirPart := filepath.Dir(rel)

	return &model.Session{
		ID:           sessionID,
		ShortID:      shortID,
		Provider:     "codex",
		Status:       model.SessionDead,
		Project:      dirPart, // YYYY/MM/DD as project context
		Model:        meta.model,
		Description:  meta.description,
		Environment:  "cli",
		MessageCount: estimateLineCount(stat.Size()),
		StartedAt:    firstTime,
		LastActive:   lastTime,
		FilePath:     path,
		FileSize:     stat.Size(),
	}, nil
}

// quickMetadata holds metadata extracted from a single pass over a JSONL file.
type quickMetadata struct {
	model       string
	description string
	firstTime   time.Time
	lastTime    time.Time
}

// extractQuickMetadata performs a single-pass scan of a JSONL file to extract:
//   - model name (from first ~30 lines)
//   - description (first user message from first ~30 lines)
//   - first timestamp (from first line)
//   - last timestamp (from last line via tail read)
//
// This replaces 4 separate file opens with 2 (forward scan + tail seek).
func extractQuickMetadata(path string) quickMetadata {
	var meta quickMetadata

	// 1. Forward scan: first ~30 lines for model + description + first timestamp
	f, err := os.Open(path)
	if err != nil {
		return meta
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for i := 0; i < 30 && scanner.Scan(); i++ {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Track first timestamp
		if meta.firstTime.IsZero() {
			meta.firstTime = extractTimestamp(line)
		}

		var rec struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
			Model   string          `json:"model"`
		}
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		// Model: direct field or payload
		if meta.model == "" {
			if rec.Model != "" {
				meta.model = rec.Model
			} else if len(rec.Payload) > 0 {
				var payload struct {
					Model    string `json:"model"`
					Response struct {
						Model string `json:"model"`
					} `json:"response"`
				}
				if err := json.Unmarshal(rec.Payload, &payload); err == nil {
					if payload.Model != "" {
						meta.model = payload.Model
					} else if payload.Response.Model != "" {
						meta.model = payload.Response.Model
					}
				}
			}
		}

		// Description: first user input
		if meta.description == "" && (rec.Type == "item.created" || rec.Type == "input") {
			meta.description = extractUserText(rec.Payload)
		}

		// Early exit if all found
		if meta.model != "" && meta.description != "" {
			break
		}
	}

	// 2. Tail read for last timestamp
	meta.lastTime = extractLastTimestamp(path)
	if meta.lastTime.IsZero() {
		meta.lastTime = meta.firstTime
	}

	return meta
}

// extractUserText extracts the first user text from an event payload.
func extractUserText(payload json.RawMessage) string {
	if len(payload) == 0 {
		return ""
	}

	var p struct {
		Item struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"item"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}

	var text string
	if p.Text != "" {
		text = p.Text
	}
	if text == "" && p.Item.Role == "user" {
		for _, c := range p.Item.Content {
			if c.Type == "input_text" || c.Type == "text" {
				text = c.Text
				break
			}
		}
	}
	if text == "" {
		for _, c := range p.Content {
			if c.Type == "input_text" || c.Type == "text" {
				text = c.Text
				break
			}
		}
	}

	if text == "" {
		return ""
	}

	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\n", " ")
	if len([]rune(text)) > 80 {
		text = string([]rune(text)[:77]) + "..."
	}
	return text
}

// extractTimestamp tries to extract a timestamp from a JSONL line.
func extractTimestamp(line []byte) time.Time {
	var rec struct {
		Timestamp float64 `json:"timestamp"`
		CreatedAt float64 `json:"created_at"`
	}
	if err := json.Unmarshal(line, &rec); err != nil {
		return time.Time{}
	}
	ts := rec.Timestamp
	if ts == 0 {
		ts = rec.CreatedAt
	}
	if ts == 0 {
		return time.Time{}
	}
	sec := int64(ts)
	nsec := int64((ts - float64(sec)) * 1e9)
	return time.Unix(sec, nsec)
}

// extractLastTimestamp reads the last line of a file to get the final timestamp.
func extractLastTimestamp(path string) time.Time {
	lastLine := readLastLine(path)
	if len(lastLine) == 0 {
		return time.Time{}
	}
	return extractTimestamp(lastLine)
}

// estimateLineCount estimates the number of events from file size.
func estimateLineCount(fileSize int64) int {
	const avgLineSize = 3000
	if fileSize <= 0 {
		return 0
	}
	est := int(fileSize / avgLineSize)
	if est < 1 {
		est = 1
	}
	return est
}

// readLastLine reads the last non-empty line from a file by seeking from the end.
func readLastLine(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil || stat.Size() == 0 {
		return nil
	}

	bufSize := int64(8192)
	if bufSize > stat.Size() {
		bufSize = stat.Size()
	}
	buf := make([]byte, bufSize)
	offset := stat.Size() - bufSize
	n, err := f.ReadAt(buf, offset)
	if err != nil && n == 0 {
		return nil
	}
	buf = buf[:n]

	end := len(buf)
	for end > 0 && (buf[end-1] == '\n' || buf[end-1] == '\r' || buf[end-1] == ' ') {
		end--
	}
	if end == 0 {
		return nil
	}

	start := end - 1
	for start > 0 && buf[start-1] != '\n' {
		start--
	}

	result := make([]byte, end-start)
	copy(result, buf[start:end])
	return result
}
