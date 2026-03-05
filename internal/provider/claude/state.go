package claude

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/glory0216/taux/internal/model"
)

// SessionState describes what a session is currently doing.
type SessionState string

const (
	StateWorking      SessionState = "working"       // Agent is processing
	StateWaitingInput SessionState = "waiting_input"  // Waiting for user input
	StateUnknown      SessionState = "unknown"
)

// DetectSessionState reads the tail of a JSONL file to determine if the
// session is waiting for user input. This is fast (~5KB read from end).
func DetectSessionState(sessionID, dataDir string) SessionState {
	// Find the session file
	projectsDir := filepath.Join(dataDir, "projects")
	pattern := filepath.Join(projectsDir, "*", sessionID+".jsonl")
	matchList, err := filepath.Glob(pattern)
	if err != nil || len(matchList) == 0 {
		return StateUnknown
	}

	return detectStateFromFile(matchList[0])
}

func detectStateFromFile(path string) SessionState {
	f, err := os.Open(path)
	if err != nil {
		return StateUnknown
	}
	defer f.Close()

	// Read last 8KB — enough to capture the last record
	const tailSize = 8192
	stat, err := f.Stat()
	if err != nil {
		return StateUnknown
	}

	offset := stat.Size() - tailSize
	if offset < 0 {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return StateUnknown
	}

	buf, err := io.ReadAll(f)
	if err != nil {
		return StateUnknown
	}

	// Find the last complete JSON line
	lines := strings.Split(strings.TrimSpace(string(buf)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		var rec model.JSONLRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}

		// Skip sidechain messages
		if rec.IsSidechain {
			continue
		}

		if rec.Type == "assistant" && len(rec.Message) > 0 {
			var am model.AssistantMessage
			if err := json.Unmarshal(rec.Message, &am); err != nil {
				return StateUnknown
			}
			if am.StopReason != nil && *am.StopReason == "end_turn" {
				return StateWaitingInput
			}
			return StateWorking
		}

		// If last meaningful record is user/result → agent is working
		if rec.Type == "user" || rec.Type == "result" {
			return StateWorking
		}
	}

	return StateUnknown
}
