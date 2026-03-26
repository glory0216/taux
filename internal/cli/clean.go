package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/glory0216/taux/internal/config"
	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/provider"
)

func newCleanCmd(app *App) *cobra.Command {
	var (
		olderThan string
		dryRun    bool
		broken    bool
	)

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean old session artifacts",
		Long:  "Remove old session JSONL files. Use --broken to remove sessions with missing/corrupt timestamps.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if broken {
				return cleanBroken(app, dryRun)
			}

			duration, err := parseDurationWithDays(olderThan)
			if err != nil {
				return fmt.Errorf("invalid duration %q: %w", olderThan, err)
			}

			if dryRun {
				fmt.Printf("Dry run: would clean sessions older than %s\n\n", olderThan)
				return dryRunClean(app, duration)
			}

			var totalFreed int64
			var anyFailed bool
			for _, p := range app.Registry.Available() {
				freed, err := p.CleanSession(ctx, duration.String())
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: %s clean failed: %v\n", p.Name(), err)
					anyFailed = true
					continue
				}
				totalFreed += freed
			}

			if anyFailed {
				fmt.Fprintf(os.Stderr, "Note: some providers failed; %s freed from successful providers.\n", formatSize(totalFreed))
			} else {
				fmt.Printf("Cleaned %s of session data.\n", formatSize(totalFreed))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&olderThan, "older-than", "30d", "Remove sessions older than this duration (e.g., 7d, 2w, 30d)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deleted without removing anything")
	cmd.Flags().BoolVar(&broken, "broken", false, "Remove sessions with missing or corrupt timestamps")

	return cmd
}

// cleanBroken finds and removes sessions with broken timestamps (zero/epoch time
// resulting in absurd ages like 106751 days).
func cleanBroken(app *App, dryRun bool) error {
	ctx := context.Background()
	sessionList, err := app.Registry.AllSession(ctx, provider.Filter{})
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	cutoff := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var brokenList []model.Session
	for _, s := range sessionList {
		isBroken := false
		if s.StartedAt.IsZero() && s.LastActive.IsZero() {
			isBroken = true
		} else if !s.StartedAt.IsZero() && s.StartedAt.Before(cutoff) {
			isBroken = true
		} else if s.StartedAt.IsZero() && !s.LastActive.IsZero() && s.LastActive.Before(cutoff) {
			isBroken = true
		}
		if isBroken {
			brokenList = append(brokenList, s)
		}
	}

	if len(brokenList) == 0 {
		fmt.Println("No broken sessions found.")
		return nil
	}

	var totalSize int64
	for _, s := range brokenList {
		totalSize += s.FileSize
		if dryRun {
			fmt.Printf("  %s  %s  %s  %s\n",
				s.ShortID,
				s.Project,
				formatSizeShort(s.FileSize),
				s.FilePath,
			)
		}
	}

	if dryRun {
		fmt.Printf("\nWould remove %d broken sessions (%s)\n", len(brokenList), formatSize(totalSize))
		return nil
	}

	var deleted int
	for _, s := range brokenList {
		for _, p := range app.Registry.Available() {
			if err := p.DeleteSession(ctx, s.ID); err == nil {
				deleted++
				break
			}
		}
	}

	fmt.Printf("Cleaned %d broken sessions (%s)\n", deleted, formatSize(totalSize))
	return nil
}

// parseDurationWithDays extends time.ParseDuration with day ("d") and week ("w") support.
func parseDurationWithDays(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(numStr, "%d", &days); err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	if strings.HasSuffix(s, "w") {
		numStr := strings.TrimSuffix(s, "w")
		var weeks int
		if _, err := fmt.Sscanf(numStr, "%d", &weeks); err != nil {
			return 0, err
		}
		return time.Duration(weeks) * 7 * 24 * time.Hour, nil
	}

	return time.ParseDuration(s)
}

// dryRunClean lists files that would be removed without actually deleting them.
func dryRunClean(app *App, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	claudeDataDir := config.ExpandPath(app.Config.Providers.Claude.DataDir)
	projectsDir := filepath.Join(claudeDataDir, "projects")

	pattern := filepath.Join(projectsDir, "*", "*.jsonl")
	matchList, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob session files: %w", err)
	}

	var totalSize int64
	var count int

	for _, match := range matchList {
		stat, err := os.Stat(match)
		if err != nil {
			continue
		}
		if stat.ModTime().Before(cutoff) {
			fmt.Printf("  %s  %s  %s\n",
				formatSize(stat.Size()),
				stat.ModTime().Format("2006-01-02"),
				filepath.Base(match),
			)
			totalSize += stat.Size()
			count++
		}
	}

	if count == 0 {
		fmt.Println("No sessions to clean.")
	} else {
		fmt.Printf("\nWould remove %d files (%s)\n", count, formatSize(totalSize))
	}

	return nil
}
