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
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Interactive TUI dashboard",
		Long:  "Launch an interactive terminal dashboard for managing AI agent sessions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDashboard(app)
		},
	}
}

// runDashboard launches the TUI and handles attach requests after exit.
// When inside tmux and an attach is requested, it opens a new tmux window
// and loops back to restart the dashboard so the window stays alive.
func runDashboard(app *App) error {
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

		req := finalModel.GetAttachRequest()
		if req == nil {
			// Normal quit — no attach
			return nil
		}

		// Handle attach request
		attached := false
		for _, prov := range app.Registry.Available() {
			cmdStr, argSlice, workDir, err := prov.AttachSession(req.SessionID)
			if err != nil || cmdStr == "" {
				continue
			}

			if isInsideTmux() {
				// Open in new tmux window, then loop back to dashboard
				_ = tmuxNewWindowAttach(cmdStr, argSlice, workDir, req.SessionID, req.Alias)
				attached = true
				break
			}
			// Outside tmux → replace process (never returns)
			return execAttachWithDir(cmdStr, argSlice, workDir)
		}

		if !attached {
			// No provider could attach — just restart dashboard
			continue
		}
		// Loop back → dashboard restarts in this window
	}
}

// tmuxNewWindowAttach opens a new tmux window with the attach command.
// The dashboard stays running in the original window.
func tmuxNewWindowAttach(cmdStr string, argSlice []string, workDir string, sessionID string, alias string) error {
	// Build the shell command to run in the new tmux window
	// e.g.: cd /path/to/project && claude --resume <id>
	var shellCmd string
	if workDir != "" {
		shellCmd = fmt.Sprintf("cd %q && %q", workDir, cmdStr)
	} else {
		shellCmd = fmt.Sprintf("%q", cmdStr)
	}
	for _, arg := range argSlice {
		shellCmd += " " + fmt.Sprintf("%q", arg)
	}

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
