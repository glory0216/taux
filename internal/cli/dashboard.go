package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/glory0216/taux/internal/tui"
)

func newDashboardCmd(app *App) *cobra.Command {
	var splitTarget string

	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Interactive TUI dashboard",
		Long:  "Launch an interactive terminal dashboard for managing AI agent sessions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDashboard(app, splitTarget)
		},
	}

	cmd.Flags().StringVar(&splitTarget, "split-target", "", "tmux pane ID to split when attaching (e.g. %3)")

	return cmd
}

// runDashboard launches the TUI and handles attach requests after exit.
// When splitTarget is set (a tmux pane ID like %3), attach splits that pane
// instead of opening a new window.
func runDashboard(app *App, splitTarget string) error {
	var pendingStatus string
	for {
		m := tui.NewModel(app.Registry, app.Config, Version, pendingStatus)
		pendingStatus = ""
		p := tea.NewProgram(m, tea.WithAltScreen())
		result, err := p.Run()
		if err != nil {
			return err
		}

		finalModel, ok := result.(*tui.Model)
		if !ok {
			return nil
		}

		// Check for replay request first
		replayReq := finalModel.GetReplayRequest()
		if replayReq != nil {
			if err := runReplayInline(replayReq); err != nil {
				// On error, loop back to dashboard
				continue
			}
			// After replay finishes, loop back to dashboard
			continue
		}

		req := finalModel.GetAttachRequest()
		if req == nil {
			// Normal quit — no attach
			return nil
		}

		// Handle attach request
		providerFound := false
		for _, prov := range app.Registry.Available() {
			cmdStr, argSlice, workDir, err := prov.AttachSession(req.SessionID)
			if err != nil || cmdStr == "" {
				continue
			}
			providerFound = true

			if isInsideTmux() {
				var attachErr error
				if splitTarget != "" {
					// Popup mode: split the target pane, then exit so the popup
					// closes and the user lands on the split layout.
					newPaneID, splitErr := tmuxSplitAttach(cmdStr, argSlice, workDir, splitTarget)
					if splitErr == nil {
						// Select the new pane before exiting so that after
						// display-popup restores the session state, focus is on
						// the new pane rather than the original.
						_ = exec.Command("tmux", "select-pane", "-t", newPaneID).Run()
					}
					attachErr = splitErr
				} else {
					// Non-popup mode: open a new window, then restart the
					// dashboard loop so this window stays alive.
					attachErr = tmuxNewWindowAttach(cmdStr, argSlice, workDir, req.SessionID, req.Alias)
				}
				if attachErr != nil {
					pendingStatus = "Attach failed: " + attachErr.Error()
					break
				}
				if splitTarget != "" {
					// Popup mode: exit to close the popup.
					return nil
				}
				// Non-popup mode: fall through to restart the dashboard.
				break
			}
			// Outside tmux → replace process (never returns)
			return execAttachWithDir(cmdStr, argSlice, workDir)
		}

		if !providerFound {
			pendingStatus = "No provider can attach to session " + req.SessionID[:min(6, len(req.SessionID))]
		}

		// Restart dashboard (attach failed or no provider found)
		continue
	}
}

// runReplayInline launches the replay TUI inline (same terminal).
func runReplayInline(req *tui.ReplayRequest) error {
	turnList, err := parseConversation(req.FilePath, false)
	if err != nil {
		return err
	}
	if len(turnList) == 0 {
		return nil
	}

	shortID := req.SessionID
	if len(shortID) > 6 {
		shortID = shortID[:6]
	}

	m := newReplayModel(turnList, shortID, req.Project, req.Model)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// buildShellCmd constructs a quoted shell command string, optionally cd-ing first.
func buildShellCmd(cmdStr string, argSlice []string, workDir string) string {
	var s string
	if workDir != "" {
		s = fmt.Sprintf("cd %q && %q", workDir, cmdStr)
	} else {
		s = fmt.Sprintf("%q", cmdStr)
	}
	for _, arg := range argSlice {
		s += " " + fmt.Sprintf("%q", arg)
	}
	return s
}

// tmuxSplitAttach splits the target pane horizontally, runs the attach command,
// and returns the new pane ID so the caller can select it before closing.
func tmuxSplitAttach(cmdStr string, argSlice []string, workDir string, targetPane string) (string, error) {
	// -d: don't switch focus during split (popup is still on top)
	// -P -F "#{pane_id}": print the new pane ID so we can select it afterward
	out, err := exec.Command("tmux", "split-window",
		"-h", "-d",
		"-t", targetPane,
		"-P", "-F", "#{pane_id}",
		buildShellCmd(cmdStr, argSlice, workDir),
	).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// tmuxNewWindowAttach opens a new tmux window with the attach command.
func tmuxNewWindowAttach(cmdStr string, argSlice []string, workDir string, sessionID string, alias string) error {
	shellCmd := buildShellCmd(cmdStr, argSlice, workDir)

	// Window name: alias if set, otherwise short session ID
	winName := alias
	if winName == "" {
		winName = sessionID
		if len(winName) > 6 {
			winName = winName[:6]
		}
	}

	cmd := exec.Command("tmux", "new-window", "-n", winName, shellCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
