package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	tabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED")).Underline(true)
	tabNormalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	tabGapStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))
)

// RenderTabs renders the tab bar: [Sessions] [Stats] [Projects]
// Highlights the active tab.
func RenderTabs(tabNameList []string, activeIdx int, width int) string {
	var partList []string
	for i, name := range tabNameList {
		label := " " + name + " "
		if i == activeIdx {
			partList = append(partList, tabActiveStyle.Render(label))
		} else {
			partList = append(partList, tabNormalStyle.Render(label))
		}
	}

	tabBar := "  " + strings.Join(partList, tabGapStyle.Render("  |  "))

	// Pad to width
	runes := []rune(tabBar)
	if len(runes) < width {
		tabBar += strings.Repeat(" ", width-len(runes))
	}

	return tabBar
}
