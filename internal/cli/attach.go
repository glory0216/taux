package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
)

// execAttachWithDir changes to the session's project directory and execs the command.
func execAttachWithDir(cmdStr string, argSlice []string, workDir string) error {
	// Resolve binary before Chdir so a lookup failure doesn't leave the process
	// in a different directory with a confusing error message.
	binary, err := exec.LookPath(cmdStr)
	if err != nil {
		return fmt.Errorf("binary not found: %s: %w", cmdStr, err)
	}
	// Make the path absolute before Chdir — LookPath can return a relative
	// path (e.g. "./claude") that would break after the directory changes.
	if !filepath.IsAbs(binary) {
		if abs, err := filepath.Abs(binary); err == nil {
			binary = abs
		}
	}

	if workDir != "" {
		if err := os.Chdir(workDir); err != nil {
			return fmt.Errorf("chdir to %s: %w", workDir, err)
		}
	}

	argv := append([]string{binary}, argSlice...)
	return syscall.Exec(binary, argv, syscall.Environ())
}

func newAttachCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach <session-id>",
		Short: "Attach to a session",
		Long:  "Resume a Claude Code session by replacing the current process with `claude --resume <id>`.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, argList []string) error {
			ctx := cmd.Context()

			fullID, err := resolveSessionID(app, ctx, argList[0])
			if err != nil {
				return err
			}

			// Find the provider that owns this session and get the attach command
			for _, p := range app.Registry.Available() {
				cmdStr, argSlice, workDir, err := p.AttachSession(fullID)
				if err != nil {
					continue
				}
				if cmdStr == "" {
					continue
				}

				return execAttachWithDir(cmdStr, argSlice, workDir)
			}

			return fmt.Errorf("no provider can attach to session %s", argList[0])
		},
	}

	return cmd
}
