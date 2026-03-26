package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newDeleteCmd(app *App) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <session-id>",
		Short: "Delete a session",
		Long: `Permanently delete a session's JSONL file. This cannot be undone.
If the session is still active, the process is killed first.

To archive the conversation before deleting, use 'memorize' instead.`,
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
				fmt.Printf("Delete session %s? This cannot be undone. [y/N] ", shortID)
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			var lastErr error
			for _, p := range app.Registry.Available() {
				if err := p.DeleteSession(ctx, fullID); err == nil {
					fmt.Printf("Deleted session %s\n", shortID)
					return nil
				} else {
					lastErr = err
				}
			}

			// Distinguish "already gone" from a real failure so the user
			// isn't confused if another process deleted it first.
			if lastErr != nil && strings.Contains(lastErr.Error(), "not found") {
				return fmt.Errorf("session %s not found (already deleted?)", shortID)
			}
			return fmt.Errorf("failed to delete session %s", shortID)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}
