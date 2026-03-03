package stats

import (
	"time"

	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/pricing"
)

// TodayTokens holds today's token breakdown from live JSONL scanning.
type TodayTokens struct {
	IOTokens   int
	CacheRead  int
	CacheWrite int
}

// TodayTokensFunc scans a data directory and returns today's token counts.
// Injected by the caller to avoid coupling this package to a specific provider.
type TodayTokensFunc func(dataDir string) TodayTokens

// AggregateStats computes period-level aggregations from the stats cache.
// This is the shared implementation used by both CLI and TUI.
func AggregateStats(sc *model.StatsCache, overrideMap map[string]pricing.TokenPrice) *model.AggregatedStats {
	now := time.Now()
	today := now.Format("2006-01-02")

	// Compute week start (Monday)
	weekday := now.Weekday()
	if weekday == 0 {
		weekday = 7
	}
	weekStart := now.AddDate(0, 0, -int(weekday-1)).Format("2006-01-02")

	// Compute month start
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")

	// Copy ModelUsage map and compute CostUSD + effective cost-per-token in one pass.
	modelBreakdown := make(map[string]model.ModelUsage, len(sc.ModelUsage))
	costPerToken := make(map[string]float64, len(sc.ModelUsage))
	for name, usage := range sc.ModelUsage {
		usage.CostUSD = pricing.CalculateModelCost(
			name, usage.InputTokens, usage.OutputTokens,
			usage.CacheReadInputTokens, usage.CacheCreationInputTokens,
			overrideMap,
		)
		modelBreakdown[name] = usage
		costPerToken[name] = pricing.EffectiveCostPerToken(
			name, usage.InputTokens, usage.OutputTokens,
			usage.CacheReadInputTokens, usage.CacheCreationInputTokens,
			overrideMap,
		)
	}

	agg := &model.AggregatedStats{
		TotalMessages:  sc.TotalMessages,
		TotalSessions:  sc.TotalSessions,
		ModelBreakdown: modelBreakdown,
	}

	for _, da := range sc.DailyActivity {
		if da.Date == today {
			agg.TodayMessages += da.MessageCount
			agg.TodaySessions += da.SessionCount
		}
		if da.Date >= weekStart {
			agg.WeekMessages += da.MessageCount
			agg.WeekSessions += da.SessionCount
		}
		if da.Date >= monthStart {
			agg.MonthMessages += da.MessageCount
			agg.MonthSessions += da.SessionCount
		}
	}

	for _, dt := range sc.DailyModelTokens {
		dayTotal := 0
		var dayCost float64
		for modelName, count := range dt.TokensByModel {
			dayTotal += count
			if rate, ok := costPerToken[modelName]; ok {
				dayCost += float64(count) * rate
			}
		}
		if dt.Date == today {
			agg.TodayTokens += dayTotal
			agg.TodayCost += dayCost
		}
		if dt.Date >= weekStart {
			agg.WeekTokens += dayTotal
			agg.WeekCost += dayCost
		}
		if dt.Date >= monthStart {
			agg.MonthTokens += dayTotal
			agg.MonthCost += dayCost
		}
	}

	// Total cost (exact, from all-time model usage)
	for _, usage := range modelBreakdown {
		agg.TotalCost += usage.CostUSD
	}

	return agg
}

// SupplementToday fills in today's session/message/token counts from live
// session data when stats-cache.json is stale (hasn't been updated today).
// todayTokensFn is optional — pass nil to skip JSONL token scanning.
func SupplementToday(agg *model.AggregatedStats, sessionList []model.Session, dataDir string, todayTokensFn TodayTokensFunc) {
	if agg.TodaySessions > 0 || agg.TodayMessages > 0 {
		return // stats-cache already has today's data
	}

	today := time.Now().Format("2006-01-02")
	for _, s := range sessionList {
		active := s.LastActive
		if active.IsZero() {
			active = s.StartedAt
		}
		if active.Format("2006-01-02") == today {
			agg.TodaySessions++
			agg.TodayMessages += s.MessageCount
		}
	}

	// Compute today's tokens from JSONL files modified today
	if agg.TodayTokens == 0 && dataDir != "" && todayTokensFn != nil {
		ts := todayTokensFn(dataDir)
		agg.TodayTokens = ts.IOTokens
		agg.TodayCacheRead = ts.CacheRead
		agg.TodayCacheWrite = ts.CacheWrite
	}
}

// IOTokensFromModels sums input + output tokens across all models (excludes cache).
func IOTokensFromModels(modelMap map[string]model.ModelUsage) int {
	var total int
	for _, usage := range modelMap {
		total += usage.InputTokens + usage.OutputTokens
	}
	return total
}
