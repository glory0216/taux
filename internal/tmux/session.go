package tmux

import (
	"os/exec"
	"strings"
)

// ClaudePane holds information about a tmux pane running Claude Code.
type ClaudePane struct {
	SessionName string
	PanePID     string
	Command     string
}

// FindClaudeSession finds tmux panes running Claude Code.
// It lists all panes across all sessions and filters for those
// whose current command contains "claude".
func FindClaudeSession() ([]ClaudePane, error) {
	out, err := exec.Command("tmux", "list-panes", "-a", "-F",
		"#{session_name}:#{pane_pid}:#{pane_current_command}").Output()
	if err != nil {
		return nil, err
	}

	var list []ClaudePane
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		partList := strings.SplitN(line, ":", 3)
		if len(partList) < 3 {
			continue
		}
		cmd := strings.ToLower(partList[2])
		if strings.Contains(cmd, "claude") {
			list = append(list, ClaudePane{
				SessionName: partList[0],
				PanePID:     partList[1],
				Command:     partList[2],
			})
		}
	}
	return list, nil
}
