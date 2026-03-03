package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/glory0216/taux/internal/model"
)

func newDescribeCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "describe <session-id>",
		Aliases: []string{"inspect", "desc"},
		Short:   "Show detailed session information",
		Long:    "Display comprehensive details about a single session including tokens, tools, and metadata.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, argList []string) error {
			ctx := cmd.Context()

			fullID, err := resolveSessionID(app, ctx, argList[0])
			if err != nil {
				return err
			}

			// Find the right provider and get details
			var detail *model.SessionDetail
			for _, p := range app.Registry.Available() {
				d, err := p.GetSession(ctx, fullID)
				if err == nil && d != nil {
					detail = d
					break
				}
			}
			if detail == nil {
				return fmt.Errorf("session not found: %s", argList[0])
			}

			// Status line
			var statusStr string
			if detail.Status == model.SessionActive {
				statusStr = fmt.Sprintf("\033[32m● active\033[0m (PID %d)", detail.PID)
			} else {
				statusStr = "\033[2m○ dead\033[0m"
			}

			fmt.Printf("Session:  %s\n", detail.ID)
			fmt.Printf("Provider: %s\n", detail.Provider)
			fmt.Printf("Status:   %s\n", statusStr)
			fmt.Printf("Project:  %s (%s)\n", detail.Project, detail.ProjectPath)
			fmt.Printf("Model:    %s\n", detail.Model)
			if detail.Version != "" {
				fmt.Printf("Version:  %s\n", detail.Version)
			}
			if detail.Environment != "" {
				envLabel := "CLI (Terminal)"
				if detail.Environment == "ide" {
					envLabel = "IDE (Cursor/VSCode)"
				}
				fmt.Printf("Env:      %s\n", envLabel)
			}
			if detail.GitBranch != "" {
				fmt.Printf("Branch:   %s\n", detail.GitBranch)
			}
			if detail.CWD != "" {
				fmt.Printf("CWD:      %s\n", detail.CWD)
			}
			fmt.Printf("Messages: %s\n", formatNumber(detail.MessageCount))

			// Token usage
			tu := detail.TokenUsage
			fmt.Printf("Tokens:   input=%s output=%s cache_read=%s cache_create=%s\n",
				formatTokenCount(tu.InputTokens),
				formatTokenCount(tu.OutputTokens),
				formatTokenCount(tu.CacheReadInputTokens),
				formatTokenCount(tu.CacheCreationInputTokens),
			)

			// Tool usage, sorted by count descending
			if len(detail.ToolUsage) > 0 {
				type toolEntry struct {
					name  string
					count int
				}
				var entryList []toolEntry
				for name, count := range detail.ToolUsage {
					entryList = append(entryList, toolEntry{name, count})
				}
				sort.Slice(entryList, func(i, j int) bool {
					return entryList[i].count > entryList[j].count
				})

				var partList []string
				for _, e := range entryList {
					partList = append(partList, fmt.Sprintf("%s(%d)", e.name, e.count))
				}
				fmt.Printf("Tools:    %s\n", strings.Join(partList, " "))
			}

			// Timestamps
			if !detail.StartedAt.IsZero() {
				fmt.Printf("Started:  %s\n", detail.StartedAt.Format("2006-01-02 15:04:05"))
			}
			if !detail.LastActive.IsZero() {
				fmt.Printf("Last:     %s\n", detail.LastActive.Format("2006-01-02 15:04:05"))
			}

			fmt.Printf("Size:     %s\n", formatSize(detail.FileSize))

			// Team info
			if detail.TeamName != "" {
				fmt.Printf("Team:     %s\n", detail.TeamName)
			}
			if detail.AgentName != "" {
				fmt.Printf("Agent:    %s\n", detail.AgentName)
			}

			// Source file
			if detail.FilePath != "" {
				fmt.Printf("Source:   %s\n", detail.FilePath)
			}

			return nil
		},
	}

	return cmd
}
