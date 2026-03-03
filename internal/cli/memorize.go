package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/glory0216/taux/internal/config"
)

func newMemorizeCmd(app *App) *cobra.Command {
	var (
		outDir     string
		keepFile   bool
	)

	cmd := &cobra.Command{
		Use:   "memorize <session-id>",
		Short: "Memorize a session and delete it",
		Long: `Export the session conversation as a compact markdown summary, then delete the original JSONL file.
The markdown is saved to the memorize directory (default: ~/.taux/memories/).

Use --keep to preserve the original session file after export.
Use -o to specify a custom output directory.`,
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

			if outDir == "" {
				outDir = config.ExpandPath(app.Config.Memorize.Dir)
			}

			// Find the provider that can memorize this session
			for _, p := range app.Registry.Available() {
				if cp, ok := p.(interface {
					MemorizeSession(id string, outDir string) (string, error)
				}); ok {
					outPath, err := cp.MemorizeSession(fullID, outDir)
					if err != nil {
						return fmt.Errorf("memorize session %s: %w", shortID, err)
					}
					fmt.Printf("Memorized session %s → %s\n", shortID, outPath)

					if !keepFile {
						if err := p.DeleteSession(ctx, fullID); err != nil {
							fmt.Printf("Warning: could not delete session file: %v\n", err)
						} else {
							fmt.Printf("Deleted session %s\n", shortID)
						}
					}
					return nil
				}
			}

			return fmt.Errorf("no provider supports memorize for session %s", shortID)
		},
	}

	cmd.Flags().StringVarP(&outDir, "output", "o", "", "Output directory for memorized file (default: config memorize.dir)")
	cmd.Flags().BoolVar(&keepFile, "keep", false, "Keep the original session file after memorizing")

	return cmd
}
