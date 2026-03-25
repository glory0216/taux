package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/glory0216/taux/internal/provider"
	"github.com/glory0216/taux/internal/provider/claude"
)

func newSearchCmd(app *App) *cobra.Command {
	var projectFilter string
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search session content",
		Long:  "Full-text search across all session conversations.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, argList []string) error {
			query := argList[0]
			ctx := context.Background()

			sessionList, err := app.Registry.AllSession(ctx, provider.Filter{})
			if err != nil {
				return fmt.Errorf("list sessions: %w", err)
			}

			found := 0
			now := time.Now()
			for _, s := range sessionList {
				if limit > 0 && found >= limit {
					break
				}

				// Project filter
				if projectFilter != "" && s.Project != projectFilter {
					continue
				}

				if s.FilePath == "" || s.Provider != "claude" {
					continue
				}

				resultList := claude.SearchSession(s.FilePath, query, 1)
				if len(resultList) == 0 {
					continue
				}

				// Format age
				age := now.Sub(s.StartedAt)
				if s.StartedAt.IsZero() {
					age = now.Sub(s.LastActive)
				}

				fmt.Printf("%s [%s] %s (%s)\n",
					s.ShortID,
					s.Project,
					s.GitBranch,
					formatAge(age),
				)
				for _, r := range resultList {
					fmt.Printf("  %s\n", r.Snippet)
				}
				fmt.Println()
				found++
			}

			if found == 0 {
				fmt.Println("No matches found.")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Filter by project name")
	cmd.Flags().IntVarP(&limit, "limit", "l", 10, "Maximum number of results")

	return cmd
}
