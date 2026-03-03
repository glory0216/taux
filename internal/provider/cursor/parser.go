package cursor

import (
	"path/filepath"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/glory0216/taux/internal/model"
)

// ParseSession reads full detail for a specific composer from the DB.
func ParseSession(dataDir string, composerID string) (*model.SessionDetail, error) {
	globalDB := globalDBPath(dataDir)
	db, err := openDB(globalDB)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}
	defer db.Close()

	// Targeted single-row query instead of loading all composers
	composer, err := queryComposerByID(db, composerID)
	if err != nil {
		return nil, err
	}
	if composer == nil {
		return nil, nil
	}

	// Get all bubbles for detail view
	bubbleList, _ := queryBubbleDataBatch(db, composerID)

	// Build workspace map for project info — use sorted list for determinism
	workspaceMap := buildWorkspaceMap(dataDir)
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

	shortID := composerID
	if len(shortID) > 6 {
		shortID = shortID[:6]
	}

	// Description
	desc := extractDescription(composer, bubbleList)

	// Timestamps
	var startedAt, lastActive time.Time
	if composer.CreatedAt > 0 {
		startedAt = time.UnixMilli(composer.CreatedAt)
	}
	if composer.LastUpdatedAt > 0 {
		lastActive = time.UnixMilli(composer.LastUpdatedAt)
	} else {
		lastActive = startedAt
	}

	// Token usage
	tokenUsage := extractTokenUsage(bubbleList)

	// Message count
	msgCount := len(bubbleList)
	if msgCount == 0 {
		msgCount = len(composer.FullConversationHeadersOnly)
		if msgCount == 0 {
			msgCount = len(composer.Conversation)
		}
	}

	// Tool usage from bubbles (count by type)
	toolUsage := make(map[string]int)
	toolCallCount := 0
	for _, b := range bubbleList {
		if b.Type == 2 { // AI messages
			toolCallCount++
			toolUsage["ai_response"]++
		}
	}

	detail := &model.SessionDetail{
		Session: model.Session{
			ID:           composerID,
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
			FilePath:     globalDB,
		},
		TokenUsage:    tokenUsage,
		ToolCallCount: toolCallCount,
		ToolUsage:     toolUsage,
	}

	// Check if active — single ps call instead of two
	procList, _ := FindCursorProcess()
	if len(procList) > 0 {
		detail.Status = model.SessionActive
		detail.PID = procList[0].PID
		detail.RSS = procList[0].RSS
		detail.CPUPercent = procList[0].CPUPercent
	}

	return detail, nil
}

// extractDescription extracts a human-readable description.
func extractDescription(cd *ComposerData, bubbleList []BubbleData) string {
	if cd.Name != "" {
		return cd.Name
	}

	// Find first user bubble
	for _, b := range bubbleList {
		if b.Type == 1 {
			text := b.Text
			if text == "" {
				text = b.RawText
			}
			return truncateUTF8(text, 80)
		}
	}

	// Legacy fallback
	if cd.Text != "" {
		return truncateUTF8(cd.Text, 80)
	}

	return filepath.Base(cd.ComposerID)
}

// truncateUTF8 truncates a string to maxLen runes without splitting multi-byte characters.
func truncateUTF8(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen-3]) + "..."
}

// extractTokenUsage sums token counts across all AI bubbles.
func extractTokenUsage(bubbleList []BubbleData) model.TokenUsage {
	var usage model.TokenUsage
	for _, b := range bubbleList {
		if b.TokenCount != nil {
			usage.InputTokens += b.TokenCount.InputTokens
			usage.OutputTokens += b.TokenCount.OutputTokens
		}
	}
	return usage
}
