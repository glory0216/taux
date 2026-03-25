package tmux

import (
	"os/exec"
	"strconv"
	"strings"
)

// IsRunning checks if tmux server is running.
func IsRunning() bool {
	err := exec.Command("tmux", "info").Run()
	return err == nil
}

// Reload sources the tmux config file.
func Reload(confPath string) error {
	return exec.Command("tmux", "source-file", confPath).Run()
}

// SetOption sets a tmux global option.
func SetOption(key, value string) error {
	return exec.Command("tmux", "set", "-g", key, value).Run()
}

// GetOption gets a tmux option value.
func GetOption(key string) (string, error) {
	out, err := exec.Command("tmux", "show-option", "-gv", key).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// BindKey binds a key in tmux.
func BindKey(key, command string) error {
	return exec.Command("tmux", "bind", key, command).Run()
}

// DisplayPopup shows a tmux popup.
func DisplayPopup(opts PopupOpts) error {
	argList := []string{"display-popup", "-E"}
	if opts.Width != "" {
		argList = append(argList, "-w", opts.Width)
	}
	if opts.Height != "" {
		argList = append(argList, "-h", opts.Height)
	}
	if opts.Title != "" {
		argList = append(argList, "-T", opts.Title)
	}
	argList = append(argList, opts.Command)
	return exec.Command("tmux", argList...).Run()
}

// PopupOpts configures a tmux display-popup command.
type PopupOpts struct {
	Width   string
	Height  string
	Title   string
	Command string
}

// DisplayMessage shows a brief message on the tmux status line.
func DisplayMessage(msg string) error {
	return exec.Command("tmux", "display-message", msg).Run()
}

// ListSession returns current tmux sessions.
func ListSession() ([]TmuxSession, error) {
	out, err := exec.Command("tmux", "list-sessions", "-F",
		"#{session_name}:#{session_windows}:#{session_attached}").Output()
	if err != nil {
		return nil, err
	}
	var list []TmuxSession
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		partList := strings.SplitN(line, ":", 3)
		if len(partList) >= 3 {
			list = append(list, TmuxSession{
				Name:     partList[0],
				Windows:  partList[1],
				Attached: partList[2] == "1",
			})
		}
	}
	return list, nil
}

// TmuxSession holds basic tmux session info.
type TmuxSession struct {
	Name     string
	Windows  string
	Attached bool
}


// CurrentPanePID returns the PID of the shell running in the current tmux pane.
func CurrentPanePID() (int, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{pane_pid}").Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

