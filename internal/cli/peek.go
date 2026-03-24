package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/glory0216/taux/internal/config"
	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/provider/claude"
	"github.com/glory0216/taux/internal/tmux"
)

func newPeekCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "peek",
		Short: "Peek at the current pane's Claude session",
		Long:  "Show state, tasks, and key info for the Claude session running in the current tmux pane.",
		RunE: func(_ *cobra.Command, _ []string) error {
			claudeDataDir := config.ExpandPath(app.Config.Providers.Claude.DataDir)

			// Find session in current pane
			sessionID, err := findCurrentPaneSession(claudeDataDir)
			if err != nil {
				fmt.Println("\033[2mNo Claude session in this pane.\033[0m")
				return nil
			}

			// Get full session detail
			ctx := context.Background()
			var detail *model.SessionDetail
			for _, p := range app.Registry.Available() {
				d, err := p.GetSession(ctx, sessionID)
				if err == nil && d != nil {
					detail = d
					break
				}
			}
			if detail == nil {
				fmt.Println("\033[2mSession not found.\033[0m")
				return nil
			}

			// Detect state
			state := claude.DetectSessionState(sessionID, claudeDataDir)

			renderPeek(detail, state)
			return nil
		},
	}
}

// findCurrentPaneSession finds the Claude session ID running in the current tmux pane.
func findCurrentPaneSession(claudeDataDir string) (string, error) {
	panePID, err := tmux.CurrentPanePID()
	if err != nil || panePID == 0 {
		return "", fmt.Errorf("not in tmux or no pane PID")
	}

	activeList, err := claude.FindActiveProcess()
	if err != nil {
		return "", err
	}

	for _, proc := range activeList {
		if proc.SessionID != "" && claude.IsChildOf(proc.PID, panePID) {
			return proc.SessionID, nil
		}
	}
	return "", fmt.Errorf("no claude session in current pane")
}

// renderPeek outputs a compact session overview for popup display.
func renderPeek(detail *model.SessionDetail, state claude.SessionState) {
	// Header
	shortID := detail.ShortID
	fmt.Printf("\033[1m Session %s\033[0m\n", shortID)
	fmt.Println(strings.Repeat("\u2500", 40))

	// State
	var stateStr string
	switch state {
	case claude.StateWaitingInput:
		stateStr = "\033[33m\u270b Waiting for input\033[0m"
	case claude.StateWorking:
		stateStr = "\033[32m\u25b6 Working\033[0m"
	default:
		stateStr = "\033[2m? Unknown\033[0m"
	}
	fmt.Printf("  State:   %s\n", stateStr)

	// Model
	if detail.Model != "" {
		fmt.Printf("  Model:   %s\n", shortenModel(detail.Model))
	}

	// Branch
	if detail.GitBranch != "" {
		fmt.Printf("  Branch:  %s\n", detail.GitBranch)
	}

	// Context window
	if detail.ContextMax > 0 {
		pct := float64(detail.ContextUsed) / float64(detail.ContextMax) * 100
		bar := renderCLIContextBar(pct, 20)
		fmt.Printf("  Context: %s %.0f%%\n", bar, pct)
	}

	// Tasks
	if len(detail.TaskList) > 0 {
		completed := 0
		inProgress := 0
		pending := 0
		for _, t := range detail.TaskList {
			switch t.Status {
			case "completed":
				completed++
			case "in_progress":
				inProgress++
			default:
				pending++
			}
		}

		fmt.Println()
		fmt.Printf("  \033[1mTasks\033[0m (%d/%d)\n", completed, len(detail.TaskList))
		for _, t := range detail.TaskList {
			var icon string
			switch t.Status {
			case "completed":
				icon = "\033[32m\u2713\033[0m"
			case "in_progress":
				icon = "\033[33m\u25d0\033[0m"
			default:
				icon = "\033[2m\u25cb\033[0m"
			}
			subject := t.Subject
			if len([]rune(subject)) > 50 {
				subject = string([]rune(subject)[:47]) + "..."
			}
			fmt.Printf("    %s %s\n", icon, subject)
		}
	}

	fmt.Println()
}
