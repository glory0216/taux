package view

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/glory0216/taux/internal/model"
)

var (
	projectHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9CA3AF"))
	projectRowStyle    = lipgloss.NewStyle()
	projectSelStyle    = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#1F2937"))
	projectActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
)

// replaceLastOccurrence replaces the last occurrence of old with new in s.
func replaceLastOccurrence(s, old, new string) string {
	idx := strings.LastIndex(s, old)
	if idx < 0 {
		return s
	}
	return s[:idx] + new + s[idx+len(old):]
}

// RenderProjectList renders the project list tab.
func RenderProjectList(projectList []model.Project, cursor, offset, width, height int) string {
	if len(projectList) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Padding(1, 2).
			Render("No projects found.")
	}

	colName := 20
	colPath := 30
	colSessions := 10
	colActive := 8
	colSize := 8
	colMem := 8
	colCPU := 6

	fixedWidth := colSessions + colActive + colSize + colMem + colCPU + 14
	remaining := width - fixedWidth
	if remaining > 20 {
		colName = remaining * 2 / 5
		colPath = remaining * 3 / 5
	}

	header := fmt.Sprintf("  %-*s %-*s %*s %*s %*s %*s %*s",
		colName, "NAME",
		colPath, "PATH",
		colSessions, "SESSIONS",
		colActive, "ACTIVE",
		colSize, "SIZE",
		colMem, "MEM",
		colCPU, "CPU",
	)
	headerLine := projectHeaderStyle.Render(truncate(header, width))

	var rowList []string
	rowList = append(rowList, headerLine)

	visibleEnd := offset + height - 1
	if visibleEnd > len(projectList) {
		visibleEnd = len(projectList)
	}

	for i := offset; i < visibleEnd; i++ {
		p := projectList[i]

		memStr := "-"
		cpuStr := "-"
		if p.ActiveCount > 0 && p.TotalRSS > 0 {
			memStr = formatSizeShort(p.TotalRSS)
		}
		if p.ActiveCount > 0 && p.TotalCPU > 0 {
			cpuStr = fmt.Sprintf("%.1f%%", p.TotalCPU)
		}

		row := fmt.Sprintf("  %-*s %-*s %*d %*d %*s %*s %*s",
			colName, truncate(p.Name, colName),
			colPath, truncate(p.Path, colPath),
			colSessions, p.SessionCount,
			colActive, p.ActiveCount,
			colSize, formatSizeShort(p.TotalSize),
			colMem, memStr,
			colCPU, cpuStr,
		)
		row = truncate(row, width)

		// Apply active count coloring
		if p.ActiveCount > 0 {
			plain := fmt.Sprintf("%d", p.ActiveCount)
			styled := projectActiveStyle.Render(plain)
			row = replaceLastOccurrence(row, " "+plain+" ", " "+styled+" ")
		}

		if i == cursor {
			rowList = append(rowList, projectSelStyle.Width(width).Render(row))
		} else {
			rowList = append(rowList, projectRowStyle.Render(row))
		}
	}

	for len(rowList) < height {
		rowList = append(rowList, "")
	}

	return strings.Join(rowList, "\n")
}
