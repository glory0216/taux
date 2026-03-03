package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSelfUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update taux to the latest version",
		Long:  "Self-update is a placeholder. Rebuild from source to update.",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("Self-update not yet implemented. Rebuild from source.")
		},
	}

	return cmd
}
