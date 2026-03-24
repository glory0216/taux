package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/glory0216/taux/internal/config"
	"github.com/glory0216/taux/internal/notify"
	"github.com/glory0216/taux/internal/provider/claude"
	"github.com/glory0216/taux/internal/tmux"
)

// watchState persists active session IDs and alerted states between invocations.
type watchState struct {
	ActiveIDList  []string `json:"active_ids"`
	WaitingIDList []string `json:"waiting_ids"` // sessions already alerted as waiting
}

// sessionTeamInfo caches per-session team metadata to avoid repeated file reads.
type sessionTeamInfo struct {
	teamName  string
	agentName string
	project   string
}

func newStatusCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Status bar output for tmux",
		Long:  "Single-line output for tmux status-right. Designed to complete in <50ms.",
		RunE: func(_ *cobra.Command, _ []string) error {
			claudeDataDir := config.ExpandPath(app.Config.Providers.Claude.DataDir)

			// Claude-specific: active process list for pane detection and notifications
			claudeActiveList, _ := claude.FindActiveProcess()

			// Notification: completion + input waiting (Claude-specific state detection)
			if app.Config.General.NotifyDesktop {
				notifySessionEvent(claudeActiveList, claudeDataDir, app)
			}

			// Count active sessions across ALL providers via registry
			activeCount := countAllActiveProcesses(app)

			// Output: active count + current pane's session branch (Claude-specific)
			if activeCount == 0 {
				fmt.Print("#[fg=colour242]\u25cb#[fg=default]")
			} else {
				branchStr := currentPaneBranch(claudeActiveList, claudeDataDir)
				if branchStr != "" {
					fmt.Printf("#[fg=green]\u25cf %d#[fg=default]  %s", activeCount, branchStr)
				} else {
					fmt.Printf("#[fg=green]\u25cf %d#[fg=default]", activeCount)
				}
			}

			return nil
		},
	}

	return cmd
}

// notifySessionEvent detects completed sessions and sessions waiting for input,
// then alerts the corresponding tmux window tab.
func notifySessionEvent(activeList []claude.ProcessInfo, claudeDataDir string, app *App) {
	configDir := filepath.Dir(config.ConfigPath())
	statePath := filepath.Join(configDir, ".watch-state.json")

	// Build current active state
	currentIDSet := make(map[string]bool, len(activeList))
	var currentIDList []string
	for _, proc := range activeList {
		if proc.SessionID != "" {
			currentIDSet[proc.SessionID] = true
			currentIDList = append(currentIDList, proc.SessionID)
		}
	}

	// Read previous state
	var prev watchState
	if data, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(data, &prev)
	}
	prevWaitingSet := make(map[string]bool)
	for _, id := range prev.WaitingIDList {
		prevWaitingSet[id] = true
	}

	aliasMap := config.LoadAlias(configDir)
	prevActiveSet := make(map[string]bool, len(prev.ActiveIDList))
	for _, id := range prev.ActiveIDList {
		prevActiveSet[id] = true
	}

	// 0) Auto-alias new child sessions (teamName present = agent-spawned)
	autoAliasNewSession(currentIDList, prevActiveSet, aliasMap, claudeDataDir, configDir)

	// 1) Detect gone sessions (were active, now gone)
	// Scan once to build a project-name lookup — avoids N repeated full scans inside the loop.
	sessionProjectMap := make(map[string]string)
	if sessionList, scanErr := claude.ScanSession(claudeDataDir); scanErr == nil {
		for _, s := range sessionList {
			sessionProjectMap[s.ID] = s.Project
		}
	}
	for _, prevID := range prev.ActiveIDList {
		if currentIDSet[prevID] {
			continue
		}
		shortID := prevID
		if len(shortID) > 6 {
			shortID = shortID[:6]
		}
		project := sessionProjectMap[prevID]
		alias := config.GetAlias(aliasMap, prevID)
		// Check JSONL now: if last state was end_turn → user closed it
		// Otherwise → agent was interrupted or session ended mid-work
		lastState := claude.DetectSessionState(prevID, claudeDataDir)
		if lastState == claude.StateWaitingInput {
			msg := formatClosedMessage(shortID, project, alias)
			_ = notify.Send("taux", msg)
		} else {
			msg := formatCompletionMessage(shortID, project, alias)
			_ = notify.Send("taux", msg)
		}
	}

	// 2) Detect sessions waiting for input
	var currentWaitingList []string
	for _, sid := range currentIDList {
		state := claude.DetectSessionState(sid, claudeDataDir)
		if state == claude.StateWaitingInput {
			currentWaitingList = append(currentWaitingList, sid)

			// Desktop notify on new transition only
			if !prevWaitingSet[sid] {
				shortID := sid
				if len(shortID) > 6 {
					shortID = shortID[:6]
				}
				alias := config.GetAlias(aliasMap, sid)
				msg := "\u270b " + shortID
				if alias != "" {
					msg += " (" + alias + ")"
				}
				msg += " waiting for input"
				_ = notify.Send("taux", msg)
			}
		}
	}

	// Save current state
	current := watchState{
		ActiveIDList:  currentIDList,
		WaitingIDList: currentWaitingList,
	}
	if data, err := json.Marshal(current); err == nil {
		_ = os.MkdirAll(filepath.Dir(statePath), 0o755)
		_ = os.WriteFile(statePath, data, 0o644)
	}
}

// currentPaneBranch finds the Claude session running in the current tmux pane
// and returns its git branch (with [wt] suffix if in a worktree).
// Returns "" if no session is found in the current pane.
func currentPaneBranch(activeList []claude.ProcessInfo, claudeDataDir string) string {
	panePID, err := tmux.CurrentPanePID()
	if err != nil || panePID == 0 {
		return ""
	}

	// Find the active session whose PID is a child of the current pane
	var matched *claude.ProcessInfo
	for i := range activeList {
		if activeList[i].SessionID != "" && claude.IsChildOf(activeList[i].PID, panePID) {
			matched = &activeList[i]
			break
		}
	}
	if matched == nil {
		return ""
	}

	// Get branch from session metadata (quick scan of first ~40 lines)
	branch, cwd := claude.QuickBranchAndCWD(matched.SessionID, claudeDataDir)
	if branch == "" {
		return ""
	}

	result := "#[fg=colour242]" + branch + "#[fg=default]"
	if cwd != "" && claude.IsWorktree(cwd) {
		result += " #[fg=colour242][wt]#[fg=default]"
	}
	return result
}

// formatCompletionMessage builds the notification string for a completed session.
func formatCompletionMessage(shortID, project, alias string) string {
	msg := "\u2713 " + shortID
	if alias != "" {
		msg += " (" + alias + ")"
	}
	if project != "" {
		msg += " [" + project + "]"
	}
	msg += " completed"
	return msg
}

// autoAliasNewSession assigns an alias to newly appeared agent-spawned sessions.
// A child session is identified by having a teamName in its metadata.
// Alias format: "parentAlias/agentName" or "teamName/agentName" or just "agentName".
func autoAliasNewSession(currentIDList []string, prevActiveSet map[string]bool, aliasMap map[string]string, claudeDataDir, configDir string) {
	// Pre-compute team info for all active sessions to avoid O(n²) file reads when
	// buildChildAlias scans all sessions looking for a parent.
	teamInfoByID := make(map[string]sessionTeamInfo, len(currentIDList))
	for _, sid := range currentIDList {
		t, a, p := claude.QuickTeamInfo(sid, claudeDataDir)
		teamInfoByID[sid] = sessionTeamInfo{t, a, p}
	}

	changed := false
	for _, sid := range currentIDList {
		if prevActiveSet[sid] || aliasMap[sid] != "" {
			continue
		}
		info := teamInfoByID[sid]
		if info.teamName == "" {
			continue
		}
		alias := buildChildAlias(aliasMap, currentIDList, sid, info.teamName, info.agentName, info.project, teamInfoByID)
		if alias == "" {
			continue
		}
		aliasMap[sid] = alias
		changed = true
	}

	if changed {
		_ = config.SaveAlias(configDir, aliasMap)
	}
}

// buildChildAlias creates an alias for a child session by looking for a parent alias.
// Parent heuristic: active session in same project without teamName that has an alias.
func buildChildAlias(aliasMap map[string]string, activeIDList []string, childSID, teamName, agentName, project string, teamInfoByID map[string]sessionTeamInfo) string {
	suffix := agentName
	if suffix == "" {
		suffix = teamName
	}

	for _, sid := range activeIDList {
		if sid == childSID {
			continue
		}
		pInfo := teamInfoByID[sid]
		if pInfo.project != project || pInfo.teamName != "" {
			continue
		}
		if parentAlias := aliasMap[sid]; parentAlias != "" {
			return parentAlias + "/" + suffix
		}
	}

	return suffix
}

// formatClosedMessage builds the notification string for a manually closed session.
func formatClosedMessage(shortID, project, alias string) string {
	msg := "\u23f9 " + shortID
	if alias != "" {
		msg += " (" + alias + ")"
	}
	if project != "" {
		msg += " [" + project + "]"
	}
	msg += " closed"
	return msg
}

// countAllActiveProcesses returns the total number of active agent sessions
// across all registered providers. Uses direct process scanning (not cache)
// so it completes in <50ms without requiring a full session list load.
func countAllActiveProcesses(app *App) int {
	ctx := context.Background()
	status, err := app.Registry.AggregateStatus(ctx)
	if err != nil || status == nil {
		// Fallback to Claude-only count on error
		activeList, _ := claude.FindActiveProcess()
		return len(activeList)
	}
	return status.ActiveCount
}
