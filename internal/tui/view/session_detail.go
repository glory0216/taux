package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/glory0216/taux/internal/model"
)

var (
	detailLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9CA3AF")).Width(12)
	detailValueStyle = lipgloss.NewStyle()
	activeStatusStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#22C55E"))
	deadStatusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	backHintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Italic(true)
)

// RenderSessionDetail renders a single session's details.
func RenderSessionDetail(detail *model.SessionDetail, width, height int) string {
	if detail == nil {
		return "No session selected."
	}

	var lineList []string

	// Session ID
	lineList = append(lineList, renderDetailRow("Session", detail.ID))

	// Provider
	if detail.Provider != "" {
		lineList = append(lineList, renderDetailRow("Provider", detail.Provider))
	}

	// Status
	var statusStr string
	if detail.Status == model.SessionActive {
		statusStr = activeStatusStyle.Render(fmt.Sprintf("\u25cf active (PID %d)", detail.PID))
	} else {
		statusStr = deadStatusStyle.Render("\u25cb dead")
	}
	lineList = append(lineList, renderDetailRow("Status", statusStr))

	// Project
	if detail.Project != "" {
		projectStr := detail.Project
		if detail.ProjectPath != "" {
			projectStr += " (" + detail.ProjectPath + ")"
		}
		lineList = append(lineList, renderDetailRow("Project", projectStr))
	}

	// Model
	if detail.Model != "" {
		lineList = append(lineList, renderDetailRow("Model", detail.Model))
	}

	// Version
	if detail.Version != "" {
		lineList = append(lineList, renderDetailRow("Version", detail.Version))
	}

	// Git Branch
	if detail.GitBranch != "" {
		lineList = append(lineList, renderDetailRow("Branch", detail.GitBranch))
	}

	// CWD
	if detail.CWD != "" {
		lineList = append(lineList, renderDetailRow("CWD", detail.CWD))
	}

	// Messages
	lineList = append(lineList, renderDetailRow("Messages", formatNumber(detail.MessageCount)))

	// Tokens
	tu := detail.TokenUsage
	tokenStr := fmt.Sprintf("in=%s  out=%s  cache_read=%s  cache_create=%s",
		formatTokenCount(tu.InputTokens),
		formatTokenCount(tu.OutputTokens),
		formatTokenCount(tu.CacheReadInputTokens),
		formatTokenCount(tu.CacheCreationInputTokens),
	)
	lineList = append(lineList, renderDetailRow("Tokens", tokenStr))

	// Context window usage
	if detail.ContextMax > 0 {
		pct := float64(detail.ContextUsed) / float64(detail.ContextMax) * 100
		bar := renderContextBar(pct, 20)
		ctxStr := fmt.Sprintf("%s %.0f%% (%s / %s)",
			bar, pct,
			formatTokenCount(detail.ContextUsed),
			formatTokenCount(detail.ContextMax),
		)
		lineList = append(lineList, renderDetailRow("Context", ctxStr))
	}

	// Tool usage
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
		lineList = append(lineList, renderDetailRow("Tools", strings.Join(partList, "  ")))
	}

	// Timestamps
	if !detail.StartedAt.IsZero() {
		lineList = append(lineList, renderDetailRow("Started", detail.StartedAt.Format("2006-01-02 15:04:05")))
	}
	if !detail.LastActive.IsZero() {
		lineList = append(lineList, renderDetailRow("Last", detail.LastActive.Format("2006-01-02 15:04:05")))
	}

	// Size
	lineList = append(lineList, renderDetailRow("Size", formatSize(detail.FileSize)))

	// Team info
	if detail.TeamName != "" {
		lineList = append(lineList, renderDetailRow("Team", detail.TeamName))
	}
	if detail.AgentName != "" {
		lineList = append(lineList, renderDetailRow("Agent", detail.AgentName))
	}

	// Task progress (from TodoWrite)
	if len(detail.TaskList) > 0 {
		completed := 0
		for _, t := range detail.TaskList {
			if t.Status == "completed" {
				completed++
			}
		}

		taskSummary := fmt.Sprintf("%d/%d completed", completed, len(detail.TaskList))
		lineList = append(lineList, renderDetailRow("Tasks", taskSummary))

		doneStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
		progStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
		pendStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

		for _, t := range detail.TaskList {
			var icon string
			switch t.Status {
			case "completed":
				icon = doneStyle.Render("\u2713")
			case "in_progress":
				icon = progStyle.Render("\u25d0")
			default:
				icon = pendStyle.Render("\u25cb")
			}
			subject := t.Subject
			if len([]rune(subject)) > 60 {
				subject = string([]rune(subject)[:57]) + "..."
			}
			lineList = append(lineList, "              "+icon+" "+subject)
		}
	}

	// Source file
	if detail.FilePath != "" {
		lineList = append(lineList, renderDetailRow("Source", detail.FilePath))
	}

	// Hint
	lineList = append(lineList, "")
	lineList = append(lineList, backHintStyle.Render("  Press esc to go back"))

	// Pad remaining height
	for len(lineList) < height {
		lineList = append(lineList, "")
	}

	return strings.Join(lineList, "\n")
}

// renderContextBar renders a colored progress bar for context window usage.
func renderContextBar(pct float64, width int) string {
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	empty := width - filled

	// Color based on usage level
	var color string
	switch {
	case pct > 80:
		color = "#EF4444" // red
	case pct > 50:
		color = "#FBBF24" // yellow
	default:
		color = "#22C55E" // green
	}

	filledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))

	return "[" + filledStyle.Render(strings.Repeat("\u2588", filled)) + emptyStyle.Render(strings.Repeat("\u2591", empty)) + "]"
}

func renderDetailRow(label, value string) string {
	return detailLabelStyle.Render(label+":") + " " + detailValueStyle.Render(value)
}

// formatNumber formats an integer with comma separators.
func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// formatTokenCount formats a token count with SI-like suffixes.
func formatTokenCount(n int) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.2fG", float64(n)/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// formatSize formats bytes into a human-readable string.
func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
