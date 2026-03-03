package view

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	filterLabelStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B"))
	filterInputStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))
	filterCursorStyle = lipgloss.NewStyle().Background(lipgloss.Color("#D1D5DB")).Foreground(lipgloss.Color("#111827"))
)

// RenderFilterBar renders the filter input at the top.
// Shows "Filter: " with cursor when active.
func RenderFilterBar(text string, active bool, width int) string {
	label := filterLabelStyle.Render("  Filter: ")
	if active {
		return label + filterInputStyle.Render(text) + filterCursorStyle.Render(" ")
	}
	if text != "" {
		return label + filterInputStyle.Render(text)
	}
	return ""
}
