package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glory0216/taux/internal/model"
)

// ParseSession reads the session metadata file and all message files for the
// given sessionID, returning a fully-populated SessionDetail.
func ParseSession(dataDir, sessionID string) (*model.SessionDetail, error) {
	// Load session metadata
	sessionFile, err := findSessionFile(dataDir, sessionID)
	if err != nil {
		return nil, fmt.Errorf("find session file for %s: %w", sessionID, err)
	}

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var sj sessionJSON
	if err := json.Unmarshal(data, &sj); err != nil {
		return nil, fmt.Errorf("parse session JSON: %w", err)
	}

	detail := &model.SessionDetail{
		ToolUsage: make(map[string]int),
	}
	detail.ID = sj.ID
	detail.ShortID = shortID(sj.ID)
	detail.Provider = "opencode"
	detail.Status = model.SessionDead
	detail.Project = filepath.Base(sj.Directory)
	detail.ProjectPath = sj.Directory
	detail.CWD = sj.Directory
	detail.Description = sj.Title
	detail.Environment = "cli"
	detail.StartedAt = parseMillis(sj.Time.Created)
	detail.LastActive = parseMillis(sj.Time.Updated)
	detail.FilePath = sessionFile

	if stat, err := os.Stat(sessionFile); err == nil {
		detail.FileSize = stat.Size()
	}

	// Aggregate token usage from message files
	msgDir := filepath.Join(dataDir, "storage", "message", sessionID)
	if err := aggregateMessages(msgDir, detail); err != nil {
		// Missing message dir is not fatal — return what we have
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("aggregate messages: %w", err)
		}
	}

	return detail, nil
}

// aggregateMessages scans all msg_*.json files in msgDir and accumulates
// token usage, message count, and model name into detail.
func aggregateMessages(msgDir string, detail *model.SessionDetail) error {
	pattern := filepath.Join(msgDir, "msg_*.json")
	matchList, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	if len(matchList) == 0 {
		// Try any .json in case naming differs
		pattern = filepath.Join(msgDir, "*.json")
		matchList, err = filepath.Glob(pattern)
		if err != nil {
			return err
		}
	}

	if len(matchList) == 0 {
		if _, statErr := os.Stat(msgDir); os.IsNotExist(statErr) {
			return os.ErrNotExist
		}
		return nil
	}

	for _, path := range matchList {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var msg messageJSON
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		detail.MessageCount++

		if detail.Model == "" && msg.Model != "" {
			detail.Model = msg.Model
		}

		detail.TokenUsage.InputTokens += msg.Usage.InputTokens
		detail.TokenUsage.OutputTokens += msg.Usage.OutputTokens
		detail.TokenUsage.CacheCreationInputTokens += msg.Usage.CacheCreationTokens
		detail.TokenUsage.CacheReadInputTokens += msg.Usage.CacheReadTokens

		// Collect tool call counts
		for _, part := range msg.Parts {
			if part.Type == "tool-invocation" || part.Type == "tool_use" {
				detail.ToolCallCount++
				detail.ToolUsage[part.Text]++
			}
		}

		// Extract description from first user message if not set
		if detail.Description == "" && msg.Role == "user" {
			text := extractPartText(msg.Parts)
			if text != "" {
				text = strings.TrimSpace(text)
				text = strings.ReplaceAll(text, "\n", " ")
				if len([]rune(text)) > 80 {
					text = string([]rune(text)[:77]) + "..."
				}
				detail.Description = text
			}
		}
	}

	return nil
}

// extractPartText returns the first text content from message parts.
func extractPartText(parts []partJSON) string {
	for _, p := range parts {
		if p.Type == "text" && p.Text != "" {
			return p.Text
		}
	}
	return ""
}

// findSessionFile searches for the session JSON file by ID across all project dirs.
func findSessionFile(dataDir, sessionID string) (string, error) {
	pattern := filepath.Join(dataDir, "storage", "session", "*", sessionID+".json")
	matchList, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matchList) > 0 {
		return matchList[0], nil
	}
	return "", fmt.Errorf("session file not found: %s", sessionID)
}

func shortID(id string) string {
	if len(id) > 6 {
		return id[:6]
	}
	return id
}
