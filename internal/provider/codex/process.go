package codex

import (
	"bufio"
	"bytes"
	"os/exec"
	"strconv"
	"strings"
)

// ProcessInfo holds info about a running Codex CLI process.
type ProcessInfo struct {
	PID        int
	SessionID  string
	RSS        int64   // resident memory in bytes
	CPUPercent float64
}

// FindActiveProcess returns all running Codex CLI processes.
// It parses `ps -eo pid,rss,pcpu,args` to find codex processes.
func FindActiveProcess() ([]ProcessInfo, error) {
	cmd := exec.Command("ps", "-eo", "pid,rss,pcpu,args")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result []ProcessInfo
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse: PID RSS %CPU ARGS...
		fieldList := strings.Fields(line)
		if len(fieldList) < 4 {
			continue
		}

		argLine := strings.Join(fieldList[3:], " ")
		if !isCodexProcess(argLine) {
			continue
		}

		pid, err := strconv.Atoi(fieldList[0])
		if err != nil {
			continue
		}
		rssKB, _ := strconv.ParseInt(fieldList[1], 10, 64)
		cpuPct, _ := strconv.ParseFloat(fieldList[2], 64)

		sessionID := extractResumeSessionID(argLine)

		result = append(result, ProcessInfo{
			PID:        pid,
			SessionID:  sessionID,
			RSS:        rssKB * 1024,
			CPUPercent: cpuPct,
		})
	}

	return result, nil
}

// isCodexProcess checks if the command line belongs to a Codex CLI process.
// We check that the binary name is exactly "codex" to avoid false positives.
func isCodexProcess(argLine string) bool {
	fieldList := strings.Fields(argLine)
	if len(fieldList) == 0 {
		return false
	}
	// The binary could be a full path like /usr/local/bin/codex or just "codex"
	binary := fieldList[0]
	baseName := binary
	if idx := strings.LastIndex(binary, "/"); idx >= 0 {
		baseName = binary[idx+1:]
	}
	return baseName == "codex"
}

// extractResumeSessionID extracts the session ID from a "--resume <id>" flag.
func extractResumeSessionID(argLine string) string {
	idx := strings.Index(argLine, "--resume")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(argLine[idx+len("--resume"):])
	fieldList := strings.Fields(rest)
	if len(fieldList) == 0 {
		return ""
	}
	return fieldList[0]
}

// FindProcessBySession returns the PID for a specific session ID.
func FindProcessBySession(sessionID string) (int, error) {
	processList, err := FindActiveProcess()
	if err != nil {
		return 0, err
	}
	for _, p := range processList {
		if p.SessionID == sessionID {
			return p.PID, nil
		}
	}
	return 0, nil
}
