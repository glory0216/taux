package aider

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ProcessInfo holds info about a running aider process.
type ProcessInfo struct {
	PID        int
	RSS        int64
	CPUPercent float64
	CWD        string
}

// FindAiderProcess returns info about running aider processes.
func FindAiderProcess() ([]ProcessInfo, error) {
	out, err := exec.Command("ps", "-eo", "pid,rss,pcpu,args").Output()
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}

	var result []ProcessInfo
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !isAiderProcess(line) {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		rssKB, _ := strconv.ParseInt(fields[1], 10, 64)
		cpu, _ := strconv.ParseFloat(fields[2], 64)

		result = append(result, ProcessInfo{
			PID:        pid,
			RSS:        rssKB * 1024, // KB to bytes
			CPUPercent: cpu,
		})
	}
	return result, nil
}

// isAiderProcess checks if a ps line represents an aider process.
func isAiderProcess(line string) bool {
	// Must contain "aider" in the args portion
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return false
	}
	args := strings.Join(fields[3:], " ")
	lower := strings.ToLower(args)

	// Skip non-aider matches
	if !strings.Contains(lower, "aider") {
		return false
	}

	// Skip grep/ps artifacts
	if strings.Contains(lower, "grep") {
		return false
	}

	// Match: python(3) ... aider, or direct aider binary
	return strings.Contains(args, "/aider") ||
		strings.Contains(args, " aider ") ||
		strings.HasSuffix(args, " aider") ||
		strings.Contains(args, " -m aider")
}

// IsAiderRunning returns true if any aider process is active.
func IsAiderRunning() bool {
	procList, err := FindAiderProcess()
	if err != nil {
		return false
	}
	return len(procList) > 0
}
