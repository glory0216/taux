package format

import (
	"fmt"
	"strings"
	"time"
)

// FormatAge converts a duration to a compact human-readable string.
// Examples: "3d", "2h", "45m", "12s"
func FormatAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}

// FormatSizeShort formats bytes compactly for table display.
// Examples: "12.3M", "4.5K", "512B"
func FormatSizeShort(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.1fG", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1fM", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1fK", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// ShortenModel strips common prefixes and date suffixes from model names for compact display.
// "claude-opus-4-6"           → "opus-4-6"
// "claude-sonnet-4-5-20250929" → "sonnet-4-5"
// "o4-mini"                   → "o4-mini"   (no change)
func ShortenModel(name string) string {
	name = strings.TrimPrefix(name, "claude-")
	// Strip 8-digit date suffix like -20251101
	if idx := strings.LastIndex(name, "-"); idx > 0 {
		suffix := name[idx+1:]
		if len(suffix) == 8 {
			allDigit := true
			for _, c := range suffix {
				if c < '0' || c > '9' {
					allDigit = false
					break
				}
			}
			if allDigit {
				name = name[:idx]
			}
		}
	}
	return name
}
