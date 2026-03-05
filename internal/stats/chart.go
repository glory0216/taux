package stats

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/glory0216/taux/internal/model"
)

// DailyPoint holds a single day's aggregated activity for charting.
type DailyPoint struct {
	Date     string
	Messages int
	Tokens   int
	Sessions int
}

// BuildDailyPoints extracts the last `days` of daily activity from a StatsCache,
// filling in zeros for missing dates. If todayAgg is non-nil, today's values
// are supplemented from the aggregated stats (which may include live session data).
func BuildDailyPoints(sc *model.StatsCache, days int, todayAgg ...*model.AggregatedStats) []DailyPoint {
	// Build lookup maps
	msgMap := make(map[string]DailyActivity)
	for _, da := range sc.DailyActivity {
		msgMap[da.Date] = DailyActivity{
			Messages: da.MessageCount,
			Sessions: da.SessionCount,
		}
	}
	tokMap := make(map[string]int)
	for _, dt := range sc.DailyModelTokens {
		total := 0
		for _, count := range dt.TokensByModel {
			total += count
		}
		tokMap[dt.Date] = total
	}

	// Generate date range (most recent first)
	now := time.Now()
	pointList := make([]DailyPoint, days)
	for i := 0; i < days; i++ {
		d := now.AddDate(0, 0, -i)
		dateStr := d.Format("2006-01-02")
		dp := DailyPoint{Date: dateStr}
		if act, ok := msgMap[dateStr]; ok {
			dp.Messages = act.Messages
			dp.Sessions = act.Sessions
		}
		if tok, ok := tokMap[dateStr]; ok {
			dp.Tokens = tok
		}
		pointList[i] = dp
	}

	// Reverse so oldest is first
	for i, j := 0, len(pointList)-1; i < j; i, j = i+1, j-1 {
		pointList[i], pointList[j] = pointList[j], pointList[i]
	}

	// Supplement today from aggregated stats (includes live session data)
	if len(todayAgg) > 0 && todayAgg[0] != nil {
		agg := todayAgg[0]
		today := now.Format("2006-01-02")
		for i := range pointList {
			if pointList[i].Date == today {
				if agg.TodayMessages > pointList[i].Messages {
					pointList[i].Messages = agg.TodayMessages
				}
				if agg.TodayTokens > pointList[i].Tokens {
					pointList[i].Tokens = agg.TodayTokens
				}
				if agg.TodaySessions > pointList[i].Sessions {
					pointList[i].Sessions = agg.TodaySessions
				}
				break
			}
		}
	}

	return pointList
}

// DailyActivity is a lightweight helper for the lookup map.
type DailyActivity struct {
	Messages int
	Sessions int
}

// RenderBarChart renders an ASCII horizontal bar chart from daily points.
// mode: "messages" or "tokens"
// barWidth: maximum number of bar characters
func RenderBarChart(pointList []DailyPoint, mode string, barWidth int) []string {
	if len(pointList) == 0 {
		return nil
	}

	// Find max value
	maxVal := 0
	for _, p := range pointList {
		v := p.Messages
		if mode == "tokens" {
			v = p.Tokens
		}
		if v > maxVal {
			maxVal = v
		}
	}

	if barWidth < 10 {
		barWidth = 10
	}

	var lineList []string
	for _, p := range pointList {
		v := p.Messages
		if mode == "tokens" {
			v = p.Tokens
		}

		// Date label (Mon 03/05 format)
		t, _ := time.Parse("2006-01-02", p.Date)
		label := t.Format("Mon 01/02")

		// Bar
		barLen := 0
		if maxVal > 0 {
			barLen = (v * barWidth) / maxVal
		}
		bar := strings.Repeat("\u2588", barLen)

		// Value label
		var valStr string
		if mode == "tokens" {
			valStr = formatTokenCountCompact(v)
		} else {
			valStr = formatNumberCompact(v)
		}

		if v == 0 {
			lineList = append(lineList, fmt.Sprintf("  %s  %s", label, valStr))
		} else {
			lineList = append(lineList, fmt.Sprintf("  %s  %s %s", label, bar, valStr))
		}
	}

	return lineList
}

// RenderBarChartColored renders a bar chart with lipgloss-compatible color codes.
// barColor: lipgloss-compatible color string (e.g., "#22C55E")
func RenderBarChartColored(pointList []DailyPoint, mode string, barWidth int, barColor, labelColor, valueColor string) []string {
	if len(pointList) == 0 {
		return nil
	}

	maxVal := 0
	for _, p := range pointList {
		v := p.Messages
		if mode == "tokens" {
			v = p.Tokens
		}
		if v > maxVal {
			maxVal = v
		}
	}

	if barWidth < 10 {
		barWidth = 10
	}

	var lineList []string
	for _, p := range pointList {
		v := p.Messages
		if mode == "tokens" {
			v = p.Tokens
		}

		t, _ := time.Parse("2006-01-02", p.Date)
		label := t.Format("Mon 01/02")

		barLen := 0
		if maxVal > 0 {
			barLen = (v * barWidth) / maxVal
		}
		bar := strings.Repeat("\u2588", barLen)

		var valStr string
		if mode == "tokens" {
			valStr = formatTokenCountCompact(v)
		} else {
			valStr = formatNumberCompact(v)
		}

		lineList = append(lineList, fmt.Sprintf("  %s  %s %s", label, bar, valStr))
	}

	return lineList
}

// WeeklyPoint holds one week's aggregated activity.
type WeeklyPoint struct {
	WeekLabel string // e.g., "Feb 24"
	Messages  int
	Tokens    int
	Sessions  int
}

// BuildWeeklyPoints aggregates daily data into weekly buckets for the last N weeks.
func BuildWeeklyPoints(sc *model.StatsCache, weeks int) []WeeklyPoint {
	// Build daily data first (enough days for the requested weeks)
	dailyPointList := BuildDailyPoints(sc, weeks*7)

	// Group by ISO week
	type weekKey struct {
		year int
		week int
	}
	weekMap := make(map[weekKey]*WeeklyPoint)
	var keyOrder []weekKey

	for _, dp := range dailyPointList {
		t, err := time.Parse("2006-01-02", dp.Date)
		if err != nil {
			continue
		}
		y, w := t.ISOWeek()
		k := weekKey{y, w}
		if _, ok := weekMap[k]; !ok {
			// Week label: Monday of that week
			weekday := int(t.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			monday := t.AddDate(0, 0, -(weekday - 1))
			weekMap[k] = &WeeklyPoint{
				WeekLabel: monday.Format("Jan 02"),
			}
			keyOrder = append(keyOrder, k)
		}
		wp := weekMap[k]
		wp.Messages += dp.Messages
		wp.Tokens += dp.Tokens
		wp.Sessions += dp.Sessions
	}

	// Sort by week
	sort.Slice(keyOrder, func(i, j int) bool {
		if keyOrder[i].year != keyOrder[j].year {
			return keyOrder[i].year < keyOrder[j].year
		}
		return keyOrder[i].week < keyOrder[j].week
	})

	result := make([]WeeklyPoint, 0, len(keyOrder))
	for _, k := range keyOrder {
		result = append(result, *weekMap[k])
	}

	return result
}

func formatTokenCountCompact(n int) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatNumberCompact(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
