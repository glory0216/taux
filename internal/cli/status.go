package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/glory0216/taux/internal/config"
	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/pricing"
	"github.com/glory0216/taux/internal/provider/claude"
)

func newStatusCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Status bar output for tmux",
		Long:  "Single-line output for tmux status-right. Designed to complete in <50ms.",
		RunE: func(_ *cobra.Command, _ []string) error {
			claudeDataDir := config.ExpandPath(app.Config.Providers.Claude.DataDir)

			// Read stats-cache.json (fast file read, no JSONL parsing)
			statsPath := filepath.Join(claudeDataDir, "stats-cache.json")
			data, err := os.ReadFile(statsPath)
			if err != nil {
				// Silent fallback: print minimal status
				fmt.Print("\u2b21 0/0")
				return nil
			}

			var statsCache model.StatsCache
			if err := json.Unmarshal(data, &statsCache); err != nil {
				fmt.Print("\u2b21 0/0")
				return nil
			}

			// Count active processes (ps is fast)
			activeList, _ := claude.FindActiveProcess()
			activeCount := len(activeList)
			totalCount := statsCache.TotalSessions

			// Today's messages and tokens
			today := time.Now().Format("2006-01-02")
			var todayMessages int
			var todayTokens int

			for _, da := range statsCache.DailyActivity {
				if da.Date == today {
					todayMessages += da.MessageCount
					break
				}
			}

			// Compute effective cost-per-token per model.
			overrideMap := app.Config.Pricing.ToTokenPriceMap()
			costPerToken := make(map[string]float64, len(statsCache.ModelUsage))
			for name, usage := range statsCache.ModelUsage {
				costPerToken[name] = pricing.EffectiveCostPerToken(
					name, usage.InputTokens, usage.OutputTokens,
					usage.CacheReadInputTokens, usage.CacheCreationInputTokens,
					overrideMap,
				)
			}

			var todayCost float64
			for _, dt := range statsCache.DailyModelTokens {
				if dt.Date == today {
					for modelName, count := range dt.TokensByModel {
						todayTokens += count
						if rate, ok := costPerToken[modelName]; ok {
							todayCost += float64(count) * rate
						}
					}
					break
				}
			}

			// Format: "⬡ 3/8  142msg  12.4k tok  $2.14"
			if todayCost > 0 {
				fmt.Printf("\u2b21 %d/%d  %dmsg  %s tok  $%.2f",
					activeCount, totalCount, todayMessages,
					formatTokenCount(todayTokens), todayCost)
			} else {
				fmt.Printf("\u2b21 %d/%d  %dmsg  %s tok",
					activeCount, totalCount, todayMessages,
					formatTokenCount(todayTokens))
			}

			return nil
		},
	}

	return cmd
}
