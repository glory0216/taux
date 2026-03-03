package component

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	tableHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9CA3AF"))
	tableRowStyle    = lipgloss.NewStyle()
	tableSelStyle    = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#1F2937"))
)

// Table renders a simple aligned table with headers and rows.
// Uses lipgloss for styling.
type Table struct {
	HeaderList []string
	RowList    [][]string
	WidthList  []int
	Selected   int
}

// Render produces the table string fitting within the given width.
func (t *Table) Render(width int) string {
	if len(t.HeaderList) == 0 {
		return ""
	}

	// Ensure we have widths for all columns
	widthList := t.computeWidthList(width)

	// Render header
	var headerPartList []string
	for i, h := range t.HeaderList {
		w := widthList[i]
		headerPartList = append(headerPartList, fmt.Sprintf("%-*s", w, truncateStr(h, w)))
	}
	header := tableHeaderStyle.Render(strings.Join(headerPartList, "  "))

	var lineList []string
	lineList = append(lineList, header)

	// Render rows
	for rowIdx, row := range t.RowList {
		var partList []string
		for colIdx, cell := range row {
			if colIdx >= len(widthList) {
				break
			}
			w := widthList[colIdx]
			partList = append(partList, fmt.Sprintf("%-*s", w, truncateStr(cell, w)))
		}
		line := strings.Join(partList, "  ")
		if rowIdx == t.Selected {
			lineList = append(lineList, tableSelStyle.Width(width).Render(line))
		} else {
			lineList = append(lineList, tableRowStyle.Render(line))
		}
	}

	return strings.Join(lineList, "\n")
}

// computeWidthList determines column widths.
func (t *Table) computeWidthList(totalWidth int) []int {
	colCount := len(t.HeaderList)
	widthList := make([]int, colCount)

	// Use provided widths if available
	if len(t.WidthList) >= colCount {
		copy(widthList, t.WidthList[:colCount])
		return widthList
	}

	// Otherwise, distribute evenly
	gaps := (colCount - 1) * 2 // 2 spaces between columns
	available := totalWidth - gaps
	if available < colCount {
		available = colCount
	}
	each := available / colCount
	for i := range widthList {
		widthList[i] = each
	}
	// Give remainder to the last column
	widthList[colCount-1] += available - each*colCount

	return widthList
}

// truncateStr trims a string to fit within maxWidth.
func truncateStr(s string, maxWidth int) string {
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
