package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/provider"
)

// formatAge converts a duration to a compact human-readable string.
// Examples: "3d", "2h", "45m", "12s"
func formatAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d >= 24*time.Hour:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}

// formatSize formats bytes into a human-readable string.
// Examples: "12.3 MB", "4.5 KB", "512 B"
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

// formatSizeShort formats bytes compactly for table display.
// Examples: "12.3M", "4.5K"
func formatSizeShort(bytes int64) string {
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

// formatTokenCount formats a token count with SI-like suffixes.
// Examples: 1000->"1.0k", 1000000->"1.0M", 500->"500"
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

// formatNumber formats an integer with comma separators.
// Examples: 1000->"1,000", 1000000->"1,000,000"
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

// shortenModel strips common prefixes from model names for compact display.
// "claude-opus-4-6" -> "opus-4-6"
// "claude-opus-4-5-20251101" -> "opus-4-5"
// "claude-sonnet-4-5-20250929" -> "sonnet-4-5"
func shortenModel(name string) string {
	name = strings.TrimPrefix(name, "claude-")
	// Strip date suffix like -20251101
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

// resolveSessionID finds a full session ID from a partial prefix.
// It searches all providers and returns an error if the prefix is ambiguous
// or no match is found.
func resolveSessionID(app *App, ctx context.Context, partial string) (string, error) {
	if partial == "" {
		return "", fmt.Errorf("session ID required")
	}

	sessionList, err := app.Registry.AllSession(ctx, provider.Filter{})
	if err != nil {
		return "", fmt.Errorf("list sessions: %w", err)
	}

	var matchList []model.Session
	for _, s := range sessionList {
		if s.ID == partial {
			// Exact match — return immediately
			return s.ID, nil
		}
		if strings.HasPrefix(s.ID, partial) {
			matchList = append(matchList, s)
		}
	}

	switch len(matchList) {
	case 0:
		return "", fmt.Errorf("no session found matching %q", partial)
	case 1:
		return matchList[0].ID, nil
	default:
		var idList []string
		for _, s := range matchList {
			idList = append(idList, s.ShortID)
		}
		return "", fmt.Errorf("ambiguous session ID %q matches: %s", partial, strings.Join(idList, ", "))
	}
}

// emptyFilter returns a zero-value filter for unfiltered queries.
func emptyFilter() provider.Filter {
	return provider.Filter{}
}

// envIcon returns a compact environment label for table display.
func envIcon(env string) string {
	switch env {
	case "ide":
		return "\033[36mIDE\033[0m"
	case "cli":
		return "CLI"
	default:
		return env
	}
}

// statusIcon returns a colored status indicator.
func statusIcon(status model.SessionStatus) string {
	switch status {
	case model.SessionActive:
		return "\033[32m● act\033[0m"
	case model.SessionDead:
		return "\033[2m○ dead\033[0m"
	default:
		return "? " + string(status)
	}
}
