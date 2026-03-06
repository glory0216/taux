package notify

import (
	"os/exec"
	"runtime"
	"strings"
)

// Send sends a desktop notification. Returns nil silently if the
// notification system is unavailable or unsupported.
func Send(title, body string) error {
	switch runtime.GOOS {
	case "darwin":
		script := `display notification "` + escapeAppleScript(body) + `" with title "` + escapeAppleScript(title) + `" sound name "Glass"`
		return exec.Command("osascript", "-e", script).Run()
	case "linux":
		return exec.Command("notify-send", title, body).Run()
	default:
		return nil
	}
}

// escapeAppleScript escapes special characters for AppleScript strings.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
