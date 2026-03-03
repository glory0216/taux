package cursor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/glory0216/taux/internal/model"
)

// WorkspaceInfo holds discovered workspace metadata.
type WorkspaceInfo struct {
	Hash        string
	ProjectPath string
	ProjectName string
	DBPath      string
}

// ScanSession scans Cursor databases and returns lightweight sessions.
func ScanSession(dataDir string) ([]model.Session, error) {
	globalDB := globalDBPath(dataDir)
	db, err := openDB(globalDB)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}
	defer db.Close()

	composerDataList, err := queryComposerDataList(db)
	if err != nil {
		return nil, err
	}

	// Build workspace map for project association
	workspaceMap := buildWorkspaceMap(dataDir)

	var sessionList []model.Session
	for _, cd := range composerDataList {
		if cd.ComposerID == "" {
			continue
		}

		// Try to find associated project.
		// NOTE: Cursor's global state.vscdb does not store a per-composer workspace
		// association, so we cannot reliably map each session to its workspace.
		// As a best-effort default, pick the alphabetically-first workspace
		// for deterministic output.  TODO: improve if Cursor adds metadata.
		projectName := "Global"
		projectPath := ""
		if len(workspaceMap) > 0 {
			var wsList []WorkspaceInfo
			for _, ws := range workspaceMap {
				wsList = append(wsList, ws)
			}
			sort.Slice(wsList, func(i, j int) bool {
				return wsList[i].ProjectPath < wsList[j].ProjectPath
			})
			projectName = wsList[0].ProjectName
			projectPath = wsList[0].ProjectPath
		}

		session := composerToSession(cd, projectPath, projectName, globalDB)
		sessionList = append(sessionList, session)
	}

	// Sort by LastActive descending
	sort.Slice(sessionList, func(i, j int) bool {
		return sessionList[i].LastActive.After(sessionList[j].LastActive)
	})

	return sessionList, nil
}

// globalDBPath returns the path to globalStorage/state.vscdb.
func globalDBPath(dataDir string) string {
	return filepath.Join(dataDir, "globalStorage", "state.vscdb")
}

// buildWorkspaceMap discovers workspace directories and their project paths.
func buildWorkspaceMap(dataDir string) map[string]WorkspaceInfo {
	result := make(map[string]WorkspaceInfo)

	wsDir := filepath.Join(dataDir, "workspaceStorage")
	pattern := filepath.Join(wsDir, "*", "workspace.json")
	matchList, err := filepath.Glob(pattern)
	if err != nil {
		return result
	}

	for _, match := range matchList {
		data, err := os.ReadFile(match)
		if err != nil {
			continue
		}

		var ws WorkspaceJSON
		if err := json.Unmarshal(data, &ws); err != nil {
			continue
		}

		if ws.Folder == "" {
			continue
		}

		hash := filepath.Base(filepath.Dir(match))
		result[hash] = WorkspaceInfo{
			Hash:        hash,
			ProjectPath: ws.Folder,
			ProjectName: filepath.Base(ws.Folder),
			DBPath:      filepath.Join(filepath.Dir(match), "state.vscdb"),
		}
	}

	return result
}

// composerToSession converts a ComposerData to a lightweight model.Session.
func composerToSession(cd ComposerData, projectPath, projectName, dbPath string) model.Session {
	shortID := cd.ComposerID
	if len(shortID) > 6 {
		shortID = shortID[:6]
	}

	// Message count from headers or legacy conversation
	msgCount := len(cd.FullConversationHeadersOnly)
	if msgCount == 0 {
		msgCount = len(cd.Conversation)
	}

	// Description from composer name or first text
	desc := cd.Name
	if desc == "" && cd.Text != "" {
		desc = cd.Text
		if len([]rune(desc)) > 80 {
			desc = string([]rune(desc)[:77]) + "..."
		}
	}

	// Timestamps
	var startedAt, lastActive time.Time
	if cd.CreatedAt > 0 {
		startedAt = time.UnixMilli(cd.CreatedAt)
	}
	if cd.LastUpdatedAt > 0 {
		lastActive = time.UnixMilli(cd.LastUpdatedAt)
	} else {
		lastActive = startedAt
	}

	return model.Session{
		ID:           cd.ComposerID,
		ShortID:      shortID,
		Provider:     "cursor",
		Status:       model.SessionDead,
		Project:      projectName,
		ProjectPath:  projectPath,
		Description:  desc,
		Environment:  "ide",
		MessageCount: msgCount,
		StartedAt:    startedAt,
		LastActive:   lastActive,
		FilePath:     dbPath,
	}
}
