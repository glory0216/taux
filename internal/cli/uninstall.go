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

func newUninstallCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall taux",
		Long:  "Remove taux configuration, data, and binary. Confirms each step unless --yes is set.",
		RunE: func(_ *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home directory: %w", err)
			}

			// Step 1: Remove taux block from tmux.conf
			tmuxConfPath := filepath.Join(home, ".tmux.conf")
			if fileExists(tmuxConfPath) {
				if yes || confirm("Remove taux block from ~/.tmux.conf?") {
					if err := removeTauxBlock(tmuxConfPath); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to update tmux.conf: %v\n", err)
					} else {
						fmt.Println("Removed taux block from ~/.tmux.conf")
					}
				}
			}

			// Step 2: Remove ~/.config/taux/
			configDir := filepath.Join(home, ".config", "taux")
			if dirExists(configDir) {
				if yes || confirm("Remove ~/.config/taux/?") {
					if err := os.RemoveAll(configDir); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to remove config dir: %v\n", err)
					} else {
						fmt.Println("Removed ~/.config/taux/")
					}
				}
			}

			// Step 3: Remove the binary itself
			binaryPath, err := os.Executable()
			if err == nil {
				if yes || confirm(fmt.Sprintf("Remove binary %s?", binaryPath)) {
					if err := os.Remove(binaryPath); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to remove binary: %v\n", err)
					} else {
						fmt.Printf("Removed %s\n", binaryPath)
					}
				}
			}

			// Step 4: Reload tmux if running
			if isTmuxRunning() {
				tmuxConf := filepath.Join(home, ".tmux.conf")
				if fileExists(tmuxConf) {
					_ = exec.Command("tmux", "source-file", tmuxConf).Run()
					fmt.Println("Reloaded tmux config.")
				}
			}

			fmt.Println("\ntaux uninstalled.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip all confirmation prompts")

	return cmd
}

// removeTauxBlock reads tmux.conf, strips the taux block, and writes it back.
func removeTauxBlock(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	content := string(data)
	startIdx := strings.Index(content, tauxBlockStart)
	endIdx := strings.Index(content, tauxBlockEnd)

	if startIdx < 0 || endIdx < 0 {
		return nil // No block to remove
	}

	endIdx += len(tauxBlockEnd)
	// Include trailing newline if present
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	newContent := content[:startIdx] + content[endIdx:]
	// Clean up excess blank lines
	newContent = strings.TrimRight(newContent, "\n") + "\n"

	return os.WriteFile(path, []byte(newContent), 0o644)
}

// confirm prompts the user with a yes/no question and returns the answer.
func confirm(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// fileExists checks if a file exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// dirExists checks if a directory exists.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
