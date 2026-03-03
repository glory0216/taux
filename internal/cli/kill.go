package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newKillCmd(app *App) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "kill <session-id>",
		Short: "Kill an active session",
		Long: `Send SIGTERM to the process running a specific session.
The session file (JSONL) is preserved — only the process is stopped.
The session will appear as "dead" in the dashboard.

Use 'delete' to remove the session file, or 'memorize' to archive and remove.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, argList []string) error {
			ctx := cmd.Context()

			fullID, err := resolveSessionID(app, ctx, argList[0])
			if err != nil {
				return err
			}

			shortID := fullID
			if len(shortID) > 6 {
				shortID = shortID[:6]
			}

			if !force {
				fmt.Printf("Kill session %s? [y/N] ", shortID)
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			// Try each provider
			for _, p := range app.Registry.Available() {
				if err := p.KillSession(ctx, fullID); err == nil {
					fmt.Printf("Sent SIGTERM to session %s\n", shortID)
					return nil
				}
			}

			return fmt.Errorf("failed to kill session %s: no active process found", shortID)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}
