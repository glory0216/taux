package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/glory0216/taux/internal/model"
)

// ScanSession scans the OpenCode storage directory for session JSON files.
//
// Directory structure:
//
//	{dataDir}/storage/session/{projectID}/{sessionID}.json
func ScanSession(dataDir string) ([]model.Session, error) {
	pattern := filepath.Join(dataDir, "storage", "session", "*", "*.json")
	matchList, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var sessionList []model.Session
	for _, match := range matchList {
		s, err := scanSingleSession(match)
		if err != nil {
			continue
		}
		sessionList = append(sessionList, *s)
	}

	sort.Slice(sessionList, func(i, j int) bool {
		ti := sessionList[i].LastActive
		if ti.IsZero() {
			ti = sessionList[i].StartedAt
		}
		tj := sessionList[j].LastActive
		if tj.IsZero() {
			tj = sessionList[j].StartedAt
		}
		return ti.After(tj)
	})

	return sessionList, nil
}

// scanSingleSession parses one session JSON file into a lightweight model.Session.
func scanSingleSession(path string) (*model.Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var sj sessionJSON
	if err := json.Unmarshal(data, &sj); err != nil {
		return nil, err
	}
	if sj.ID == "" {
		return nil, os.ErrInvalid
	}

	stat, _ := os.Stat(path)
	var fileSize int64
	if stat != nil {
		fileSize = stat.Size()
	}

	shortID := sj.ID
	if len(shortID) > 6 {
		shortID = shortID[:6]
	}

	projectName := filepath.Base(sj.Directory)

	startedAt := parseMillis(sj.Time.Created)
	lastActive := parseMillis(sj.Time.Updated)
	if lastActive.IsZero() {
		lastActive = startedAt
	}
	if startedAt.IsZero() && stat != nil {
		startedAt = stat.ModTime()
		lastActive = stat.ModTime()
	}

	return &model.Session{
		ID:          sj.ID,
		ShortID:     shortID,
		Provider:    "opencode",
		Status:      model.SessionDead,
		Project:     projectName,
		ProjectPath: sj.Directory,
		Description: sj.Title,
		Environment: "cli",
		StartedAt:   startedAt,
		LastActive:  lastActive,
		FilePath:    path,
		FileSize:    fileSize,
	}, nil
}

// parseMillis converts an interface{} millisecond timestamp to time.Time.
// OpenCode stores timestamps as JSON numbers (float64 after unmarshal).
func parseMillis(v interface{}) time.Time {
	if v == nil {
		return time.Time{}
	}
	switch t := v.(type) {
	case float64:
		if t == 0 {
			return time.Time{}
		}
		return time.UnixMilli(int64(t))
	case int64:
		if t == 0 {
			return time.Time{}
		}
		return time.UnixMilli(t)
	case string:
		// Some versions may store as string
		parsed, err := time.Parse(time.RFC3339, t)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}
