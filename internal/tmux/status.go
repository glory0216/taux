package tmux

import "fmt"

// FormatStatusRight formats the status-right string for tmux.
// Input: active count, total count, today messages, today tokens
// Output: "⬡ 3/8  142msg  12.4k tok"
func FormatStatusRight(active, total, msgCount, tokenCount int) string {
	return fmt.Sprintf("\u2b21 %d/%d  %dmsg  %s tok",
		active,
		total,
		msgCount,
		FormatTokenCount(tokenCount),
	)
}

// FormatTokenCount formats token count with SI suffix.
// Examples: 1000->"1.0k", 1000000->"1.0M", 500->"500"
func FormatTokenCount(n int) string {
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
