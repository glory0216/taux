package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

// execAttachWithDir changes to the session's project directory and execs the command.
func execAttachWithDir(cmdStr string, argSlice []string, workDir string) error {
	// Change to the session's project directory so claude --resume can find it
	if workDir != "" {
		if err := os.Chdir(workDir); err != nil {
			return fmt.Errorf("chdir to %s: %w", workDir, err)
		}
	}

	binary, err := exec.LookPath(cmdStr)
	if err != nil {
		return fmt.Errorf("binary not found: %s: %w", cmdStr, err)
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
