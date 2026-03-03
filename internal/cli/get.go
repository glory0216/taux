package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/glory0216/taux/internal/config"
	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/provider"
)

func newGetCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Display one or many resources",
		Long:  "Display sessions, projects, or stats. Usage: taux get [sessions|projects|stats]",
	}

	cmd.AddCommand(
		newGetSessionsCmd(app),
		newGetProjectsCmd(app),
		newGetStatsCmd(app),
	)

	return cmd
}

func newGetSessionsCmd(app *App) *cobra.Command {
	var (
		status      string
		providerArg string
		project     string
		limit       int
	)

	cmd := &cobra.Command{
		Use:     "sessions",
		Aliases: []string{"session", "sess"},
		Short:   "List sessions",
		Long:    "Display a table of all known agent sessions with status, project, model, and more.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			filter := provider.Filter{
				Project: project,
				Limit:   limit,
			}
			if status != "" {
				filter.Status = model.SessionStatus(status)
			}

			var sessionList []model.Session
			var err error

			if providerArg != "" {
				p := app.Registry.Get(providerArg)
				if p == nil {
					return fmt.Errorf("unknown provider: %s", providerArg)
				}
				sessionList, err = p.ListSession(ctx, filter)
			} else {
				sessionList, err = app.Registry.AllSession(ctx, filter)
			}
			if err != nil {
				return fmt.Errorf("list sessions: %w", err)
			}

			if len(sessionList) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			// Load aliases
			configDir := filepath.Dir(config.ConfigPath())
			aliasMap := config.LoadAlias(configDir)

			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tALIAS\tSTATUS\tENV\tPROJECT\tMODEL\tMSGS\tSIZE\tMEM\tCPU\tAGE\tDESCRIPTION")

			now := time.Now()
			for _, s := range sessionList {
				age := now.Sub(s.StartedAt)
				if s.StartedAt.IsZero() {
					age = now.Sub(s.LastActive)
				}

				statusStr := statusIcon(s.Status)
				modelStr := shortenModel(s.Model)
				envStr := envIcon(s.Environment)
				alias := config.GetAlias(aliasMap, s.ID)

				memStr := "-"
				cpuStr := "-"
				if s.Status == model.SessionActive && s.RSS > 0 {
					memStr = formatSizeShort(s.RSS)
				}
				if s.Status == model.SessionActive && s.CPUPercent > 0 {
					cpuStr = fmt.Sprintf("%.1f%%", s.CPUPercent)
				}

				desc := s.Description
				if len([]rune(desc)) > 50 {
					desc = string([]rune(desc)[:47]) + "..."
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
					s.ShortID,
					alias,
					statusStr,
					envStr,
					s.Project,
					modelStr,
					s.MessageCount,
					formatSizeShort(s.FileSize),
					memStr,
					cpuStr,
					formatAge(age),
					desc,
				)
			}

			return w.Flush()
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status (active, dead)")
	cmd.Flags().StringVarP(&providerArg, "provider", "p", "", "Filter by provider (claude, cursor, copilot)")
	cmd.Flags().StringVar(&project, "project", "", "Filter by project name")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "Maximum number of sessions to display")

	return cmd
}

func newGetProjectsCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "projects",
		Aliases: []string{"project", "proj"},
		Short:   "List projects",
		Long:    "Display a table of projects grouped by working directory with session counts and disk usage.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			sessionList, err := app.Registry.AllSession(ctx, provider.Filter{})
			if err != nil {
				return fmt.Errorf("list sessions: %w", err)
			}

			projectList := model.BuildProjectList(sessionList)
			if len(projectList) == 0 {
				fmt.Println("No projects found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tPATH\tSESSIONS\tACTIVE\tSIZE\tMEM\tCPU")

			for _, p := range projectList {
				memStr := "-"
				cpuStr := "-"
				if p.ActiveCount > 0 && p.TotalRSS > 0 {
					memStr = formatSizeShort(p.TotalRSS)
				}
				if p.ActiveCount > 0 && p.TotalCPU > 0 {
					cpuStr = fmt.Sprintf("%.1f%%", p.TotalCPU)
				}
				fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\t%s\t%s\n",
					p.Name,
					p.Path,
					p.SessionCount,
					p.ActiveCount,
					formatSizeShort(p.TotalSize),
					memStr,
					cpuStr,
				)
			}

			return w.Flush()
		},
	}

	return cmd
}

// newGetStatsCmd creates the "get stats" subcommand — delegates to the shared stats logic.
func newGetStatsCmd(app *App) *cobra.Command {
	cmd := newStatsCmd(app)
	cmd.Use = "stats"
	cmd.Aliases = []string{"stat"}
	return cmd
}
