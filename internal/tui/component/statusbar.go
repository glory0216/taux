package component

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/glory0216/taux/internal/provider"
)

var (
	statusBarBgStyle    = lipgloss.NewStyle().Background(lipgloss.Color("#111827")).Foreground(lipgloss.Color("#D1D5DB"))
	statusActiveStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	statusDeadStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	statusMsgStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
)

// RenderStatusBar renders the bottom bar with session counts and status message.
func RenderStatusBar(status *provider.ProviderStatus, statusText string, width int) string {
	var left string
	if status != nil {
		deadCount := status.TotalCount - status.ActiveCount
		if deadCount < 0 {
			deadCount = 0
		}
		left = fmt.Sprintf("  %s %d active  %s %d dead  |  %d msg  %s tok",
			statusActiveStyle.Render("\u25cf"),
			status.ActiveCount,
			statusDeadStyle.Render("\u25cb"),
			deadCount,
			status.MessageCount,
			formatTokenCountShort(status.TokenCount),
		)
	} else {
		left = "  Loading..."
	}

	if statusText != "" {
		left += "  |  " + statusMsgStyle.Render(statusText)
	}

	// Pad to fill width
	padded := left
	runes := []rune(padded)
	if len(runes) < width {
		for len(runes) < width {
			runes = append(runes, ' ')
		}
		padded = string(runes)
	}

	return statusBarBgStyle.Render(padded)
}

// formatTokenCountShort formats a token count with SI-like suffixes.
func formatTokenCountShort(n int) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fG", float64(n)/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
