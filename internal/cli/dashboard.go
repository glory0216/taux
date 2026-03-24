package cli

import (
	"fmt"
	"os"
	"os/exec"

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
	for {
		m := tui.NewModel(app.Registry, app.Config, Version)
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
		for _, prov := range app.Registry.Available() {
			cmdStr, argSlice, workDir, err := prov.AttachSession(req.SessionID)
			if err != nil || cmdStr == "" {
				continue
			}

			if isInsideTmux() {
				if splitTarget != "" {
					_ = tmuxSplitAttach(cmdStr, argSlice, workDir, splitTarget)
				} else {
					_ = tmuxNewWindowAttach(cmdStr, argSlice, workDir, req.SessionID, req.Alias)
				}
				break
			}
			// Outside tmux → replace process (never returns)
			return execAttachWithDir(cmdStr, argSlice, workDir)
		}

		// No provider could attach — just restart dashboard
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

// tmuxSplitAttach splits the target pane horizontally and runs the attach command.
func tmuxSplitAttach(cmdStr string, argSlice []string, workDir string, targetPane string) error {
	cmd := exec.Command("tmux", "split-window", "-h", "-t", targetPane, buildShellCmd(cmdStr, argSlice, workDir))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
