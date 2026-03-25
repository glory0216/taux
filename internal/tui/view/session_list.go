package view

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/glory0216/taux/internal/format"
	"github.com/glory0216/taux/internal/model"
)

var (
	headerStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9CA3AF"))
	rowStyle         = lipgloss.NewStyle()
	selectedRowStyle = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#1F2937"))
	activeIconStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	workingIconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	waitingIconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	deadIconStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	ideRowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	ideSelStyle      = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#1F2937")).Foreground(lipgloss.Color("#9CA3AF"))
	aliasStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
)

// RenderSessionList renders the session table.
func RenderSessionList(sessionList []model.Session, aliasMap map[string]string, cursor, offset, width, height int) string {
	if len(sessionList) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Padding(1, 2).
			Render("No sessions found. Start a Claude Code session to see it here.")
	}

	// Column widths
	colStatus := 4
	colID := 8
	colAlias := 16
	colEnv := 3
	colProv := 7
	colProject := 12
	colModel := 12
	colBranch := 8
	colMsgs := 6
	colSize := 8
	colMem := 7
	colCPU := 5
	colAge := 5

	// Header
	header := fmt.Sprintf("  %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %*s %*s %*s %*s %*s",
		colStatus, "",
		colID, "ID",
		colAlias, "ALIAS",
		colEnv, "ENV",
		colProv, "PROV",
		colProject, "PROJECT",
		colModel, "MODEL",
		colBranch, "BRANCH",
		colMsgs, "MSGS",
		colSize, "SIZE",
		colMem, "MEM",
		colCPU, "CPU",
		colAge, "AGE",
	)
	headerLine := headerStyle.Render(truncate(header, width))

	// Each session takes 2 lines: main row + description
	rowsPerSession := 2

	// Rows
	now := time.Now()
	var rowList []string
	rowList = append(rowList, headerLine)

	// Calculate visible range (each session uses 2 lines)
	visibleCount := (height - 1) / rowsPerSession // -1 for header
	visibleEnd := offset + visibleCount
	if visibleEnd > len(sessionList) {
		visibleEnd = len(sessionList)
	}
	if offset > len(sessionList) {
		offset = 0
	}

	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	for i := offset; i < visibleEnd; i++ {
		s := sessionList[i]

		// Status icon (with state for active sessions)
		var icon string
		if s.Status == model.SessionActive {
			switch s.State {
			case model.StateWaitingInput:
				icon = waitingIconStyle.Render(" \u270b")
			case model.StateWorking:
				icon = workingIconStyle.Render(" \u25b6")
			default:
				icon = activeIconStyle.Render(" \u25cf")
			}
		} else {
			icon = deadIconStyle.Render(" \u25cb")
		}

		// Age
		age := now.Sub(s.StartedAt)
		if s.StartedAt.IsZero() {
			age = now.Sub(s.LastActive)
		}

		// Environment indicator
		envStr := "CLI"
		envStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
		if s.Environment == "ide" {
			envStr = "IDE"
			envStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8"))
		}

		// Alias
		alias := ""
		if aliasMap != nil {
			alias = aliasMap[s.ID]
		}
		aliasField := fmt.Sprintf("%-*s", colAlias, truncate(alias, colAlias))
		if alias != "" {
			aliasField = aliasStyle.Render(aliasField)
		}

		// MEM/CPU (only for active sessions)
		memStr := "-"
		cpuStr := "-"
		if s.Status == model.SessionActive && s.RSS > 0 {
			memStr = format.FormatSizeShort(s.RSS)
		}
		if s.Status == model.SessionActive && s.CPUPercent > 0 {
			cpuStr = fmt.Sprintf("%.1f%%", s.CPUPercent)
		}

		// Provider
		provField := fmt.Sprintf("%-*s", colProv, truncate(s.Provider, colProv))

		row := fmt.Sprintf("%s %-*s %s %s %s %-*s %-*s %-*s %*d %*s %*s %*s %*s",
			icon,
			colID, s.ShortID,
			aliasField,
			envStyle.Render(fmt.Sprintf("%-*s", colEnv, envStr)),
			provField,
			colProject, truncate(s.Project, colProject),
			colModel, truncate(format.ShortenModel(s.Model), colModel),
			colBranch, truncate(s.GitBranch, colBranch),
			colMsgs, s.MessageCount,
			colSize, format.FormatSizeShort(s.FileSize),
			colMem, memStr,

			colCPU, cpuStr,
			colAge, format.FormatAge(age),
		)

		// Description line (indented under the row)
		desc := s.Description
		maxDescWidth := width - 6
		if maxDescWidth > 0 && desc != "" {
			desc = "    " + truncate(desc, maxDescWidth)
		} else {
			desc = ""
		}

		isIDE := s.Environment == "ide"
		if i == cursor {
			sel := selectedRowStyle
			if isIDE {
				sel = ideSelStyle
			}
			rowList = append(rowList, sel.Width(width).Render(truncate(row, width)))
			if desc != "" {
				rowList = append(rowList, sel.Width(width).Render(desc))
			} else {
				rowList = append(rowList, "")
			}
		} else if isIDE {
			rowList = append(rowList, ideRowStyle.Render(truncate(row, width)))
			rowList = append(rowList, ideRowStyle.Render(desc))
		} else {
			rowList = append(rowList, rowStyle.Render(truncate(row, width)))
			rowList = append(rowList, descStyle.Render(desc))
		}
	}

	// Pad remaining height
	for len(rowList) < height {
		rowList = append(rowList, "")
	}

	return strings.Join(rowList, "\n")
}

// truncate trims a string to a maximum width, adding ellipsis if needed.
func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-1]) + "\u2026"
}

