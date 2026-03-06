package cli

import (
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

func newStatusCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Status bar output for tmux",
		Long:  "Single-line output for tmux status-right. Designed to complete in <50ms.",
		RunE: func(_ *cobra.Command, _ []string) error {
			claudeDataDir := config.ExpandPath(app.Config.Providers.Claude.DataDir)

			// Count active processes
			activeList, _ := claude.FindActiveProcess()
			activeCount := len(activeList)

			// Notification: completion + input waiting
			if app.Config.General.NotifyDesktop {
				notifySessionEvent(activeList, claudeDataDir, app)
			}

			// Output: just active count
			if activeCount == 0 {
				fmt.Print("#[fg=colour242]\u25cb#[fg=default]")
			} else {
				fmt.Printf("#[fg=green]\u25cf %d#[fg=default]", activeCount)
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
	pidBySession := make(map[string]int)
	for _, proc := range activeList {
		if proc.SessionID != "" {
			currentIDSet[proc.SessionID] = true
			currentIDList = append(currentIDList, proc.SessionID)
			pidBySession[proc.SessionID] = proc.PID
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

	// Get tmux pane list (for window alerting)
	paneList, _ := tmux.ListPane()
	aliasMap := config.LoadAlias(configDir)

	// 1) Detect completed sessions (were active, now gone)
	for _, prevID := range prev.ActiveIDList {
		if currentIDSet[prevID] {
			continue
		}
		shortID := prevID
		if len(shortID) > 6 {
			shortID = shortID[:6]
		}
		project := ""
		sessionList, _ := claude.ScanSession(claudeDataDir)
		for _, s := range sessionList {
			if s.ID == prevID {
				project = s.Project
				break
			}
		}
		alias := config.GetAlias(aliasMap, prevID)
		msg := formatCompletionMessage(shortID, project, alias)
		_ = notify.Send("taux", msg)
	}

	// 2) Detect sessions waiting for input (new transition only)
	var currentWaitingList []string
	for _, sid := range currentIDList {
		state := claude.DetectSessionState(sid, claudeDataDir)
		if state == claude.StateWaitingInput {
			currentWaitingList = append(currentWaitingList, sid)

			// Only alert on new transition (wasn't waiting before)
			if !prevWaitingSet[sid] {
				// Find tmux window for this process and send bell
				pid := pidBySession[sid]
				if pid > 0 {
					alertWindowByPID(pid, paneList)
				}

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
				_ = tmux.DisplayMessage(msg)
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

// alertWindowByPID finds the tmux window containing a process and sends a bell.
func alertWindowByPID(pid int, paneList []tmux.PaneInfo) {
	for _, pane := range paneList {
		if pane.PanePID == pid {
			_ = tmux.AlertWindow(pane.WindowID)
			return
		}
	}
}

// formatCompletionMessage builds the tmux display-message string for a completed session.
func formatCompletionMessage(shortID, project, alias string) string {
	msg := "\u2713 Session " + shortID
	if alias != "" {
		msg += " (" + alias + ")"
	}
	if project != "" {
		msg += " [" + project + "]"
	}
	msg += " completed"
	return msg
}
