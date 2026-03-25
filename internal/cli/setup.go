package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	tauxBlockStart = "# ===== taux ====="
	tauxBlockEnd   = "# ===== End taux ====="
)

// tauxBlockLines are the tmux.conf lines that don't depend on existing config.
// The status-right line is generated dynamically by buildTauxBlock() to avoid
// overwriting any existing status-right the user already has.
var tauxBlockLines = []string{
	tauxBlockStart,
	"set -g status-interval 10",
	// status-right line inserted here by buildTauxBlock()
	"# Active window highlight",
	"setw -g window-status-style 'fg=colour245'",
	"setw -g window-status-current-style 'fg=colour16,bg=colour39,bold'",
	"# Keybindings",
	"bind H display-popup -E -w 80% -h 80% -T ' taux ' 'taux dashboard --split-target #{pane_id}'",
	"bind A display-popup -E -w 60% -h 50% -T ' Active Sessions ' 'bash -c \"taux get sessions -s active; read -rsn1\"'",
	"bind S display-popup -E -w 60% -h 50% -T ' Stats ' 'bash -c \"taux get stats; read -rsn1\"'",
	"bind P display-popup -E -w 60% -h 50% -T ' Peek ' 'bash -c \"taux peek; read -rsn1\"'",
	tauxBlockEnd,
}

// buildTauxBlock returns the full taux tmux.conf block.
// If tmux is running and has a non-taux status-right, the existing value is
// preserved by prepending the taux fragment rather than replacing it.
func buildTauxBlock() string {
	fragment := "#(taux status 2>/dev/null)"
	statusRight := fragment + "  %H:%M %Y-%m-%d" // default

	if isTmuxRunning() {
		if out, err := exec.Command("tmux", "show-option", "-gqv", "status-right").Output(); err == nil {
			existing := strings.TrimSpace(string(out))
			if existing != "" && !strings.Contains(existing, "taux status") {
				statusRight = fragment + "  " + existing
			}
		}
	}

	lines := make([]string, 0, len(tauxBlockLines)+1)
	for i, line := range tauxBlockLines {
		lines = append(lines, line)
		// Insert status-right after the status-interval line
		if i == 1 {
			lines = append(lines, "set -g status-right '"+statusRight+"'")
		}
	}
	return strings.Join(lines, "\n")
}

func newSetupCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Add taux configuration to tmux.conf",
		Long:  "Adds or replaces the taux block in ~/.tmux.conf for status bar integration and keybindings.",
		RunE: func(_ *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home directory: %w", err)
			}
			tmuxConfPath := filepath.Join(home, ".tmux.conf")
			tauxBlock := buildTauxBlock()

			if dryRun {
				fmt.Printf("Would add to %s:\n\n%s\n", tmuxConfPath, tauxBlock)
				return nil
			}

			// Read existing tmux.conf (may not exist)
			existing, _ := os.ReadFile(tmuxConfPath)
			content := string(existing)

			// Replace or append the taux block
			newContent := replaceOrAppendBlock(content, tauxBlock)

			if err := os.WriteFile(tmuxConfPath, []byte(newContent), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", tmuxConfPath, err)
			}

			fmt.Printf("Updated %s\n", tmuxConfPath)
			fmt.Println()
			fmt.Println("Block added:")
			fmt.Println(tauxBlock)
			fmt.Println()

			// Offer to reload tmux
			if isTmuxRunning() {
				fmt.Print("Reload tmux config now? [Y/n] ")
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer == "" || answer == "y" || answer == "yes" {
					if err := exec.Command("tmux", "source-file", tmuxConfPath).Run(); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: tmux reload failed: %v\n", err)
					} else {
						fmt.Println("tmux config reloaded.")
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be added without modifying files")

	return cmd
}

// replaceOrAppendBlock replaces the taux block if it already exists in the
// content, or appends it at the end.
func replaceOrAppendBlock(content string, block string) string {
	startIdx := strings.Index(content, tauxBlockStart)
	endIdx := strings.Index(content, tauxBlockEnd)

	if startIdx >= 0 && endIdx >= 0 {
		// Replace existing block
		endIdx += len(tauxBlockEnd)
		// Include trailing newline if present
		if endIdx < len(content) && content[endIdx] == '\n' {
			endIdx++
		}
		return content[:startIdx] + block + "\n" + content[endIdx:]
	}

	// Append
	result := strings.TrimRight(content, "\n")
	if result != "" {
		result += "\n\n"
	}
	result += block + "\n"
	return result
}

// isTmuxRunning checks if a tmux server is running.
func isTmuxRunning() bool {
	err := exec.Command("tmux", "has-session").Run()
	return err == nil
}
