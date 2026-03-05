package view

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/glory0216/taux/internal/model"
	statsutil "github.com/glory0216/taux/internal/stats"
)

var (
	statsHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	statsLabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Width(14)
	statsValueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))
	modelNameStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))
	cacheStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
)

// RenderStatsPanel renders the stats tab.
// Shows: daily activity summary, model token breakdown, disk usage.
func RenderStatsPanel(stats *model.StatsCache, agg *model.AggregatedStats, diskUsageBytes int64, width, height int) string {
	if agg == nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Padding(1, 2).
			Render("No stats available. Run some Claude Code sessions to generate stats.")
	}

	var lineList []string

	// Activity Summary
	lineList = append(lineList, statsHeaderStyle.Render("  Activity Summary"))
	lineList = append(lineList, "")

	todayCostStr := ""
	if agg.TodayCost > 0 {
		todayCostStr = fmt.Sprintf("   $%.2f", agg.TodayCost)
	}
	lineList = append(lineList, renderStatsRow("Today",
		fmt.Sprintf("%d sessions   %s messages   %s tokens%s",
			agg.TodaySessions, formatNumber(agg.TodayMessages), formatTokenCount(agg.TodayTokens), todayCostStr)))
	if agg.TodayCacheRead > 0 || agg.TodayCacheWrite > 0 {
		lineList = append(lineList, "  "+statsLabelStyle.Render("")+" "+
			cacheStyle.Render(fmt.Sprintf("+ %s cache_read, %s cache_write",
				formatTokenCount(agg.TodayCacheRead), formatTokenCount(agg.TodayCacheWrite))))
	}

	weekCostStr := ""
	if agg.WeekCost > 0 {
		weekCostStr = fmt.Sprintf("   $%.2f", agg.WeekCost)
	}
	lineList = append(lineList, renderStatsRow("This Week",
		fmt.Sprintf("%d sessions   %s messages   %s tokens%s",
			agg.WeekSessions, formatNumber(agg.WeekMessages), formatTokenCount(agg.WeekTokens), weekCostStr)))

	monthCostStr := ""
	if agg.MonthCost > 0 {
		monthCostStr = fmt.Sprintf("   $%.2f", agg.MonthCost)
	}
	lineList = append(lineList, renderStatsRow("This Month",
		fmt.Sprintf("%d sessions   %s messages   %s tokens%s",
			agg.MonthSessions, formatNumber(agg.MonthMessages), formatTokenCount(agg.MonthTokens), monthCostStr)))

	// All Time with input+output tokens
	allTimeTokens := 0
	var totalCacheRead, totalCacheWrite int
	if agg.ModelBreakdown != nil {
		for _, usage := range agg.ModelBreakdown {
			allTimeTokens += usage.InputTokens + usage.OutputTokens
			totalCacheRead += usage.CacheReadInputTokens
			totalCacheWrite += usage.CacheCreationInputTokens
		}
	}
	totalCostStr := ""
	if agg.TotalCost > 0 {
		totalCostStr = fmt.Sprintf("   $%.2f", agg.TotalCost)
	}
	lineList = append(lineList, renderStatsRow("All Time",
		fmt.Sprintf("%d sessions   %s messages   %s tokens%s",
			agg.TotalSessions, formatNumber(agg.TotalMessages), formatTokenCount(allTimeTokens), totalCostStr)))

	lineList = append(lineList, "")

	// Daily Activity Chart (14 days)
	if stats != nil && len(stats.DailyActivity) > 0 {
		lineList = append(lineList, statsHeaderStyle.Render("  Daily Activity (14 days)"))
		lineList = append(lineList, "")

		chartBarWidth := width - 30
		if chartBarWidth < 10 {
			chartBarWidth = 10
		}
		if chartBarWidth > 50 {
			chartBarWidth = 50
		}

		pointList := statsutil.BuildDailyPoints(stats, 14, agg)

		barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
		dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
		valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))

		// Find max
		maxVal := 0
		for _, p := range pointList {
			if p.Messages > maxVal {
				maxVal = p.Messages
			}
		}

		for _, p := range pointList {
			barLen := 0
			if maxVal > 0 {
				barLen = (p.Messages * chartBarWidth) / maxVal
			}
			bar := strings.Repeat("\u2588", barLen)

			t, _ := time.Parse("2006-01-02", p.Date)
			label := t.Format("Mon 01/02")

			valStr := formatNumber(p.Messages)
			lineList = append(lineList,
				"  "+dateStyle.Render(label)+"  "+barStyle.Render(bar)+" "+valStyle.Render(valStr))
		}

		lineList = append(lineList, "")
	}

	// Model Breakdown
	if agg.ModelBreakdown != nil && len(agg.ModelBreakdown) > 0 {
		lineList = append(lineList, statsHeaderStyle.Render("  Model Usage (input + output)"))
		lineList = append(lineList, "")

		type modelEntry struct {
			name     string
			ioTokens int
			cost     float64
		}
		var entryList []modelEntry
		for name, usage := range agg.ModelBreakdown {
			ioTokens := usage.InputTokens + usage.OutputTokens
			entryList = append(entryList, modelEntry{name, ioTokens, usage.CostUSD})
		}
		sort.Slice(entryList, func(i, j int) bool {
			return entryList[i].ioTokens > entryList[j].ioTokens
		})

		for _, e := range entryList {
			nameStr := modelNameStyle.Render(fmt.Sprintf("    %-30s", e.name))
			valueStr := statsValueStyle.Render(fmt.Sprintf("%s tokens", formatTokenCount(e.ioTokens)))
			if e.cost > 0 {
				valueStr += statsValueStyle.Render(fmt.Sprintf("   $%.2f", e.cost))
			}
			lineList = append(lineList, nameStr+valueStr)
		}

		lineList = append(lineList, "")

		// Cache tokens
		if totalCacheRead > 0 || totalCacheWrite > 0 {
			lineList = append(lineList, statsHeaderStyle.Render("  Cache Tokens"))
			lineList = append(lineList, "")
			cacheReadLabel := cacheStyle.Render("    cache_read ")
			lineList = append(lineList, cacheReadLabel+statsValueStyle.Render(
				fmt.Sprintf("%s tokens  (reused context, 90%% discount)", formatTokenCount(totalCacheRead))))
			cacheWriteLabel := cacheStyle.Render("    cache_write")
			lineList = append(lineList, cacheWriteLabel+statsValueStyle.Render(
				fmt.Sprintf(" %s tokens  (new context cached)", formatTokenCount(totalCacheWrite))))
			lineList = append(lineList, "")
		}
	}

	// Disk Usage
	lineList = append(lineList, statsHeaderStyle.Render("  Disk Usage"))
	lineList = append(lineList, "")
	diskStr := formatSize(diskUsageBytes)
	if diskUsageBytes == 0 && agg.DiskUsageBytes > 0 {
		diskStr = formatSize(agg.DiskUsageBytes)
	}
	lineList = append(lineList, renderStatsRow("Sessions", fmt.Sprintf("%s (%d files)", diskStr, agg.TotalSessions)))

	// Pad remaining height
	for len(lineList) < height {
		lineList = append(lineList, "")
	}

	return strings.Join(lineList, "\n")
}

func renderStatsRow(label, value string) string {
	return "  " + statsLabelStyle.Render(label) + " " + statsValueStyle.Render(value)
}
