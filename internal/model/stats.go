package model

// StatsCache maps directly to ~/.claude/stats-cache.json
type StatsCache struct {
	Version          int              `json:"version"`
	LastComputedDate string           `json:"lastComputedDate"`
	DailyActivity    []DailyActivity  `json:"dailyActivity"`
	DailyModelTokens []DailyModelTokens `json:"dailyModelTokens"`
	ModelUsage       map[string]ModelUsage `json:"modelUsage"`
	TotalSessions    int              `json:"totalSessions"`
	TotalMessages    int              `json:"totalMessages"`
	LongestSession   *LongestSession  `json:"longestSession"`
	FirstSessionDate string           `json:"firstSessionDate"`
	HourCounts       map[string]int   `json:"hourCounts"`
}

type DailyActivity struct {
	Date          string `json:"date"`
	MessageCount  int    `json:"messageCount"`
	SessionCount  int    `json:"sessionCount"`
	ToolCallCount int    `json:"toolCallCount"`
}

type DailyModelTokens struct {
	Date          string         `json:"date"`
	TokensByModel map[string]int `json:"tokensByModel"`
}

type ModelUsage struct {
	InputTokens              int `json:"inputTokens"`
	OutputTokens             int `json:"outputTokens"`
	CacheReadInputTokens     int `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int `json:"cacheCreationInputTokens"`
	WebSearchRequests        int `json:"webSearchRequests"`
	CostUSD                  float64 `json:"costUSD"`
}

type LongestSession struct {
	SessionID    string `json:"sessionId"`
	Duration     int64  `json:"duration"`
	MessageCount int    `json:"messageCount"`
	Timestamp    string `json:"timestamp"`
}

// AggregatedStats is the computed stats for display.
type AggregatedStats struct {
	TodayMessages        int
	TodaySessions        int
	TodayTokens          int
	TodayCacheRead       int
	TodayCacheWrite      int
	WeekMessages         int
	WeekSessions         int
	WeekTokens           int
	MonthMessages        int
	MonthSessions        int
	MonthTokens          int
	TotalMessages        int
	TotalSessions        int
	DiskUsageBytes       int64
	ModelBreakdown       map[string]ModelUsage
	TodayCost            float64
	WeekCost             float64
	MonthCost            float64
	TotalCost            float64
}
