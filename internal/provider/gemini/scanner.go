package gemini

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/glory0216/taux/internal/model"
)

// ScanSession scans the Gemini tmp directory for session JSON files.
// Directory structure: ~/.gemini/tmp/<project_hash>/chats/<session>.json
func ScanSession(dataDir string) ([]model.Session, error) {
	tmpDir := filepath.Join(dataDir, "tmp")
	pattern := filepath.Join(tmpDir, "*", "chats", "*.json")
	matchList, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var sessionList []model.Session
	for _, match := range matchList {
		session, err := scanSingleSession(match, tmpDir)
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

// scanSingleSession extracts lightweight session info from a single JSON file.
func scanSingleSession(path string, tmpDir string) (*model.Session, error) {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".json") {
		return nil, os.ErrInvalid
	}

	// Session ID from filename (strip .json)
	sessionID := strings.TrimSuffix(base, ".json")
	if sessionID == "" {
		return nil, os.ErrInvalid
	}

	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Extract project hash from path: tmp/<hash>/chats/file.json
	rel, err := filepath.Rel(tmpDir, path)
	if err != nil {
		return nil, err
	}
	dirParts := strings.Split(filepath.Dir(rel), string(filepath.Separator))
	projectHash := ""
	if len(dirParts) >= 1 {
		projectHash = dirParts[0]
	}

	// Try to resolve project path from metadata
	projectName := resolveProjectName(tmpDir, projectHash)

	// Build short ID: use first 6 chars of session ID
	shortID := sessionID
	if len(shortID) > 6 {
		shortID = shortID[:6]
	}

	// Quick metadata from file (reads only first ~64KB via streaming)
	meta := extractQuickMetadata(path)

	// Use file mod time as fallback for timestamps
	startedAt := stat.ModTime()
	lastActive := stat.ModTime()

	// Try to extract timestamp from session filename (session-YYYY-MM-DDTHH-MM-SSZ)
	if ts := parseSessionTimestamp(sessionID); !ts.IsZero() {
		startedAt = ts
	}

	return &model.Session{
		ID:           sessionID,
		ShortID:      shortID,
		Provider:     "gemini",
		Status:       model.SessionDead,
		Project:      projectName,
		ProjectPath:  projectHash,
		Model:        meta.model,
		Description:  meta.description,
		Environment:  "cli",
		MessageCount: meta.messageCount,
		StartedAt:    startedAt,
		LastActive:   lastActive,
		FilePath:     path,
		FileSize:     stat.Size(),
	}, nil
}

// resolveProjectName tries to find a human-readable project name for a hash directory.
func resolveProjectName(tmpDir string, projectHash string) string {
	if projectHash == "" {
		return ""
	}

	// Check for metadata files that might contain the project path
	hashDir := filepath.Join(tmpDir, projectHash)

	// Try reading a metadata/config file
	for _, name := range []string{"metadata.json", "config.json", ".project"} {
		data, err := os.ReadFile(filepath.Join(hashDir, name))
		if err != nil {
			continue
		}
		var meta struct {
			ProjectPath string `json:"project_path"`
			Path        string `json:"path"`
			Name        string `json:"name"`
		}
		if err := json.Unmarshal(data, &meta); err == nil {
			if meta.Name != "" {
				return meta.Name
			}
			if meta.ProjectPath != "" {
				return filepath.Base(meta.ProjectPath)
			}
			if meta.Path != "" {
				return filepath.Base(meta.Path)
			}
		}
	}

	// Fallback: use hash prefix as project name
	if len(projectHash) > 8 {
		return projectHash[:8]
	}
	return projectHash
}

// parseSessionTimestamp extracts a timestamp from session filenames like
// "session-2025-03-01T14-30-00Z" or similar patterns.
func parseSessionTimestamp(sessionID string) time.Time {
	// Try: session-YYYY-MM-DDTHH-MM-SSZ
	sessionID = strings.TrimPrefix(sessionID, "session-")

	// Replace dashes in time portion back to colons for parsing
	// Pattern: 2025-03-01T14-30-00Z → 2025-03-01T14:30:00Z
	if len(sessionID) >= 19 {
		candidate := sessionID[:10] + "T"
		timePart := sessionID[11:]
		if len(timePart) >= 8 {
			candidate += strings.Replace(timePart[:8], "-", ":", 2)
			if len(timePart) > 8 {
				candidate += timePart[8:]
			}
		}
		if t, err := time.Parse(time.RFC3339, candidate); err == nil {
			return t
		}
	}

	return time.Time{}
}

type quickMeta struct {
	model        string
	description  string
	messageCount int
}

// extractQuickMetadata reads a Gemini session JSON file using streaming decoder
// and extracts model, first user message, and message count.
// Only processes the first few messages — stops early once metadata is found.
func extractQuickMetadata(path string) quickMeta {
	f, err := os.Open(path)
	if err != nil {
		return quickMeta{}
	}
	defer f.Close()

	// Limit read to 64KB for quick scan — wrap in a LimitReader
	lr := io.LimitReader(f, 65536)
	dec := json.NewDecoder(lr)

	var meta quickMeta

	// Peek at first token
	tok, err := dec.Token()
	if err != nil {
		return meta
	}

	switch tok {
	case json.Delim('{'):
		// Object: find "messages" key
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return meta
			}
			key, ok := keyTok.(string)
			if !ok {
				continue
			}
			if key == "messages" {
				// Expect '['
				arrTok, err := dec.Token()
				if err != nil || arrTok != json.Delim('[') {
					return meta
				}
				meta = scanMessagesForMeta(dec)
				return meta
			}
			// Skip other keys
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return meta
			}
		}
	case json.Delim('['):
		// Direct array
		meta = scanMessagesForMeta(dec)
	}

	return meta
}

// scanMessagesForMeta iterates messages from a decoder (inside an array)
// and extracts model + description. Counts all messages.
func scanMessagesForMeta(dec *json.Decoder) quickMeta {
	var meta quickMeta

	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			break
		}
		meta.messageCount++

		// Only parse details if we still need model or description
		if meta.model != "" && meta.description != "" {
			continue
		}

		var msg struct {
			Role          string `json:"role"`
			Model         string `json:"model"`
			Content       string `json:"content"`
			UsageMetadata *struct {
				Model string `json:"model"`
			} `json:"usageMetadata"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		if meta.model == "" {
			if msg.Model != "" {
				meta.model = msg.Model
			}
			if msg.UsageMetadata != nil && msg.UsageMetadata.Model != "" {
				meta.model = msg.UsageMetadata.Model
			}
		}

		if meta.description == "" && msg.Role == "user" {
			text := msg.Content
			if text == "" {
				for _, p := range msg.Parts {
					if p.Text != "" {
						text = p.Text
						break
					}
				}
			}
			if text != "" {
				text = strings.TrimSpace(text)
				text = strings.ReplaceAll(text, "\n", " ")
				if len([]rune(text)) > 80 {
					text = string([]rune(text)[:77]) + "..."
				}
				meta.description = text
			}
		}
	}

	return meta
}
