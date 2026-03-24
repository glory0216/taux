package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glory0216/taux/internal/model"
)

// ScanSession scans all project directories under dataDir/projects/ for JSONL
// session files. Returns a lightweight []model.Session without full parsing.
//
// Directory structure: ~/.claude/projects/<encoded-path>/<uuid>.jsonl
// Encoded path example: -Users-alice-Projects-myapp -> /Users/alice/Projects/myapp
func ScanSession(dataDir string) ([]model.Session, error) {
	projectsDir := filepath.Join(dataDir, "projects")
	pattern := filepath.Join(projectsDir, "*", "*.jsonl")
	matchList, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var sessionList []model.Session
	for _, match := range matchList {
		session, err := scanSingleSession(match, projectsDir)
		if err != nil {
			// Skip files that can't be parsed
			continue
		}
		sessionList = append(sessionList, *session)
	}

	sortSessionByLastActive(sessionList)
	return sessionList, nil
}

// scanSingleSession extracts lightweight session info from a single JSONL file.
// It uses 3 file opens total: stat + extractQuickMetadata (forward scan) + readLastLine (tail seek).
// This avoids the previous approach of 5 separate opens and a full-file countLines.
func scanSingleSession(path string, projectsDir string) (*model.Session, error) {
	// Extract session ID from filename: <uuid>.jsonl
	base := filepath.Base(path)
	sessionID := strings.TrimSuffix(base, ".jsonl")
	if sessionID == base {
		return nil, os.ErrInvalid
	}

	// Extract project path from parent directory name
	dirName := filepath.Base(filepath.Dir(path))
	projectPath := decodeProjectPath(dirName)
	projectName := filepath.Base(projectPath)

	// File stat for size and modtime
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Extract model, description, environment, CWD, and first timestamp in a single forward scan
	meta := extractQuickMetadata(path)

	// Read last line for last timestamp (tail seek — fast for any file size)
	lastLine, _ := readLastLine(path)
	var lastTimestamp time.Time
	if lastLine != nil {
		var rec model.JSONLRecord
		if err := json.Unmarshal(lastLine, &rec); err == nil {
			lastTimestamp = rec.Timestamp
		}
	}

	// Estimate message count from file size instead of reading the entire file
	msgCount := estimateLineCount(stat.Size())

	// Build short ID
	shortID := sessionID
	if len(shortID) > 6 {
		shortID = shortID[:6]
	}

	session := &model.Session{
		ID:           sessionID,
		ShortID:      shortID,
		Provider:     "claude",
		Status:       model.SessionDead, // Will be updated by caller with process info
		Project:      projectName,
		ProjectPath:  projectPath,
		Model:        meta.Model,
		Description:  meta.Description,
		Environment:  meta.Environment,
		CWD:          meta.CWD,
		GitBranch:    meta.GitBranch,
		MessageCount: msgCount,
		StartedAt:    meta.FirstTimestamp,
		LastActive:   lastTimestamp,
		FilePath:     path,
		FileSize:     stat.Size(),
		TeamName:     meta.TeamName,
		AgentName:    meta.AgentName,
	}

	return session, nil
}

// decodeProjectPath converts a directory name back to a filesystem path.
// Example: "-Users-alice-Projects-myapp" -> "/Users/alice/Projects/myapp"
//
// The encoding replaces "/" with "-" and prepends "-" for the leading "/".
// So we replace the leading "-" with "/" and then all remaining "-" with "/".
// However, directory/file names may legitimately contain "-", so we use a
// heuristic: the Claude Code directory name is the full absolute path with
// path separators replaced by "-".
func decodeProjectPath(encoded string) string {
	if encoded == "" {
		return ""
	}
	// The encoded path starts with "-" which represents the leading "/"
	// Each "-" in the encoded path represents a "/"
	// Example: -Users-alice-Projects-myapp -> /Users/alice/Projects/myapp
	decoded := strings.ReplaceAll(encoded, "-", string(filepath.Separator))
	return decoded
}
