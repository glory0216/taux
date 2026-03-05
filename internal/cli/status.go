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
	"github.com/glory0216/taux/internal/tmux"
)

// watchState persists active session IDs between taux status invocations.
type watchState struct {
	ActiveIDList []string `json:"active_ids"`
}

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
				fmt.Print("#[fg=colour242]\u25cb#[fg=default]")
				return nil
			}

			var statsCache model.StatsCache
			if err := json.Unmarshal(data, &statsCache); err != nil {
				fmt.Print("#[fg=colour242]\u25cb#[fg=default]")
				return nil
			}

			// Count active processes (ps is fast)
			activeList, _ := claude.FindActiveProcess()
			activeCount := len(activeList)

			// Session completion notification
			if app.Config.General.NotifyCompletion {
				notifyCompletedSession(activeList, claudeDataDir, app)
			}

			// Idle state — minimal footprint
			if activeCount == 0 {
				fmt.Print("#[fg=colour242]\u25cb#[fg=default]")
				return nil
			}

			// Active state — collect today's stats
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

			// Format: "● 3  520msg  45.2k tok  $3.14"
			// Green dot + count, default stats, yellow cost
			fmt.Printf("#[fg=green]\u25cf %d#[fg=default]  %dmsg  %s tok",
				activeCount, todayMessages, formatTokenCount(todayTokens))
			if todayCost > 0 {
				fmt.Printf("  #[fg=yellow]$%.2f#[fg=default]", todayCost)
			}

			return nil
		},
	}

	return cmd
}

// notifyCompletedSession detects sessions that were active on the previous
// status invocation but are no longer running, and sends a tmux display-message
// for each completed session.
func notifyCompletedSession(activeList []claude.ProcessInfo, claudeDataDir string, app *App) {
	configDir := filepath.Dir(config.ConfigPath())
	statePath := filepath.Join(configDir, ".watch-state.json")

	// Build current active ID set
	currentIDSet := make(map[string]bool, len(activeList))
	var currentIDList []string
	for _, proc := range activeList {
		if proc.SessionID != "" {
			currentIDSet[proc.SessionID] = true
			currentIDList = append(currentIDList, proc.SessionID)
		}
	}

	// Read previous state
	var prev watchState
	if data, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(data, &prev)
	}

	// Detect completed sessions (were active, now gone)
	aliasMap := config.LoadAlias(configDir)
	for _, prevID := range prev.ActiveIDList {
		if currentIDSet[prevID] {
			continue
		}
		// This session completed — build notification message
		shortID := prevID
		if len(shortID) > 6 {
			shortID = shortID[:6]
		}

		// Try to get project name from session file
		project := ""
		sessionList, _ := claude.ScanSession(claudeDataDir)
		for _, s := range sessionList {
			if s.ID == prevID {
				project = s.Project
				break
			}
		}

		alias := config.GetAlias(aliasMap, prevID)
		msg := formatCompletionMessage(shortID, project, alias)
		_ = tmux.DisplayMessage(msg)
	}

	// Save current state
	current := watchState{ActiveIDList: currentIDList}
	if data, err := json.Marshal(current); err == nil {
		_ = os.MkdirAll(filepath.Dir(statePath), 0o755)
		_ = os.WriteFile(statePath, data, 0o644)
	}
}

// formatCompletionMessage builds the tmux display-message string for a completed session.
func formatCompletionMessage(shortID, project, alias string) string {
	msg := "\u2713 Session " + shortID
	if alias != "" {
		msg += " (" + alias + ")"
	}
	if project != "" {
		msg += " [" + project + "]"
	}
	msg += " completed"
	return msg
}
