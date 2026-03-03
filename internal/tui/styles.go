package tui

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	ColorPrimary = lipgloss.Color("#7C3AED") // Purple
	ColorActive  = lipgloss.Color("#22C55E") // Green
	ColorDead    = lipgloss.Color("#6B7280") // Gray
	ColorAccent  = lipgloss.Color("#3B82F6") // Blue
	ColorWarning = lipgloss.Color("#F59E0B") // Yellow
	ColorBorder  = lipgloss.Color("#374151") // Dark gray
	ColorDim     = lipgloss.Color("#9CA3AF") // Light gray
)

// Styles
var (
	TitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	ActiveStyle    = lipgloss.NewStyle().Foreground(ColorActive)
	DeadStyle      = lipgloss.NewStyle().Foreground(ColorDead)
	AccentStyle    = lipgloss.NewStyle().Foreground(ColorAccent)
	DimStyle       = lipgloss.NewStyle().Foreground(ColorDim)
	SelectedStyle  = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#1F2937"))
	TabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Underline(true)
	TabStyle       = lipgloss.NewStyle().Foreground(ColorDim)
	StatusBarStyle = lipgloss.NewStyle().Background(lipgloss.Color("#111827")).Foreground(lipgloss.Color("#D1D5DB"))
	HelpStyle      = lipgloss.NewStyle().Foreground(ColorDim)
	BorderStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder)
)
