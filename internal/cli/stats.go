package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/glory0216/taux/internal/config"
	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/provider"
	"github.com/glory0216/taux/internal/provider/claude"
	"github.com/glory0216/taux/internal/stats"
)

func newStatsCmd(app *App) *cobra.Command {
	var period string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show aggregated statistics",
		Long:  "Display session, message, and token statistics from the stats cache.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			claudeDataDir := config.ExpandPath(app.Config.Providers.Claude.DataDir)
			statsPath := filepath.Join(claudeDataDir, "stats-cache.json")

			var statsCache model.StatsCache
			data, err := os.ReadFile(statsPath)
			if err != nil {
				if !os.IsNotExist(err) {
					return fmt.Errorf("read stats cache: %w", err)
				}
				// No stats cache yet — use empty stats
				statsCache = model.StatsCache{ModelUsage: make(map[string]model.ModelUsage)}
			} else if err := json.Unmarshal(data, &statsCache); err != nil {
				return fmt.Errorf("parse stats cache: %w", err)
			}

			overrideMap := app.Config.Pricing.ToTokenPriceMap()
			agg := stats.AggregateStats(&statsCache, overrideMap)

			// Compute disk usage from session files
			projectsDir := filepath.Join(claudeDataDir, "projects")
			pattern := filepath.Join(projectsDir, "*", "*.jsonl")
			matchList, _ := filepath.Glob(pattern)
			var diskBytes int64
			for _, m := range matchList {
				if st, err := os.Stat(m); err == nil {
					diskBytes += st.Size()
				}
			}
			agg.DiskUsageBytes = diskBytes

			// Supplement today stats from live session data if cache is stale
			ctx := cmd.Context()
			sessionList, err := app.Registry.AllSession(ctx, provider.Filter{})
			if err == nil {
				stats.SupplementToday(agg, sessionList, claudeDataDir, claudeTodayTokens)
			}

			printStats(agg, period)
			return nil
		},
	}

	cmd.Flags().StringVar(&period, "period", "all", "Display period (today, week, month, all)")

	return cmd
}


// claudeTodayTokens adapts claude.SumTodayTokens to the stats.TodayTokensFunc signature.
func claudeTodayTokens(dataDir string) stats.TodayTokens {
	ts := claude.SumTodayTokens(dataDir)
	return stats.TodayTokens{
		IOTokens:   ts.IOTokens,
		CacheRead:  ts.CacheRead,
		CacheWrite: ts.CacheWrite,
	}
}

// printStats renders the stats to stdout.
func printStats(agg *model.AggregatedStats, period string) {
	fmt.Println("\033[1m═══ taux Stats ═══\033[0m")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)

	showRow := func(label string, sessions, messages, tokens int, cost float64) {
		costStr := ""
		if cost > 0 {
			costStr = fmt.Sprintf("\t$%.2f", cost)
		}
		fmt.Fprintf(w, "%s\t%d sessions\t%s messages\t%s tokens%s\n",
			label,
			sessions,
			formatNumber(messages),
			formatTokenCount(tokens),
			costStr,
		)
	}

	showToday := func() {
		showRow("Today", agg.TodaySessions, agg.TodayMessages, agg.TodayTokens, agg.TodayCost)
		if agg.TodayCacheRead > 0 || agg.TodayCacheWrite > 0 {
			fmt.Fprintf(w, "\t\t\t  + %s cache_read, %s cache_write\n",
				formatTokenCount(agg.TodayCacheRead), formatTokenCount(agg.TodayCacheWrite))
		}
	}

	switch period {
	case "today":
		showToday()
	case "week":
		showRow("This Week", agg.WeekSessions, agg.WeekMessages, agg.WeekTokens, agg.WeekCost)
	case "month":
		showRow("This Month", agg.MonthSessions, agg.MonthMessages, agg.MonthTokens, agg.MonthCost)
	default:
		// Show all periods
		showToday()
		showRow("This Week", agg.WeekSessions, agg.WeekMessages, agg.WeekTokens, agg.WeekCost)
		showRow("This Month", agg.MonthSessions, agg.MonthMessages, agg.MonthTokens, agg.MonthCost)
		showRow("All Time", agg.TotalSessions, agg.TotalMessages, stats.IOTokensFromModels(agg.ModelBreakdown), agg.TotalCost)
	}

	w.Flush()

	// Model breakdown
	if len(agg.ModelBreakdown) > 0 {
		fmt.Println()
		fmt.Println("\033[1mModel Usage (input + output)\033[0m")

		type modelEntry struct {
			name       string
			ioTokens   int
			cacheRead  int
			cacheWrite int
			cost       float64
		}
		var entryList []modelEntry
		for name, usage := range agg.ModelBreakdown {
			entryList = append(entryList, modelEntry{
				name:       name,
				ioTokens:   usage.InputTokens + usage.OutputTokens,
				cacheRead:  usage.CacheReadInputTokens,
				cacheWrite: usage.CacheCreationInputTokens,
				cost:       usage.CostUSD,
			})
		}
		sort.Slice(entryList, func(i, j int) bool {
			return entryList[i].ioTokens > entryList[j].ioTokens
		})

		mw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		for _, e := range entryList {
			costStr := ""
			if e.cost > 0 {
				costStr = fmt.Sprintf("\t$%.2f", e.cost)
			}
			fmt.Fprintf(mw, "  %s\t%s tokens%s\n", e.name, formatTokenCount(e.ioTokens), costStr)
		}
		mw.Flush()

		// Cache breakdown
		var totalCacheRead, totalCacheWrite int
		for _, e := range entryList {
			totalCacheRead += e.cacheRead
			totalCacheWrite += e.cacheWrite
		}
		if totalCacheRead > 0 || totalCacheWrite > 0 {
			fmt.Println()
			fmt.Println("\033[1mCache Tokens\033[0m")
			cw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintf(cw, "  cache_read\t%s tokens\t(reused context, 90%% discount)\n", formatTokenCount(totalCacheRead))
			fmt.Fprintf(cw, "  cache_write\t%s tokens\t(new context cached)\n", formatTokenCount(totalCacheWrite))
			cw.Flush()
		}
	}

	// Disk usage
	fmt.Printf("\nDisk Usage: %s (%d sessions)\n", formatSize(agg.DiskUsageBytes), agg.TotalSessions)
}

