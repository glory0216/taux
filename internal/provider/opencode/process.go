package opencode

import (
	"bufio"
	"bytes"
	"os/exec"
	"strconv"
	"strings"
)

// FindActiveProcess returns all running opencode processes with basic info.
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

		// Match opencode processes, excluding our own ps
		if !strings.Contains(line, "opencode") || strings.Contains(line, "ps -eo") {
			continue
		}

		fieldList := strings.Fields(line)
		if len(fieldList) < 4 {
			continue
		}
		pid, err := strconv.Atoi(fieldList[0])
		if err != nil {
			continue
		}
		rssKB, _ := strconv.ParseInt(fieldList[1], 10, 64)
		cpuPct, _ := strconv.ParseFloat(fieldList[2], 64)

		result = append(result, ProcessInfo{
			PID:        pid,
			RSS:        rssKB * 1024,
			CPUPercent: cpuPct,
		})
	}

	return result, nil
}

// FindProcessBySession returns the PID for a running opencode session.
// OpenCode does not expose session ID via process args, so this always returns 0.
func FindProcessBySession(sessionID string) (int, error) {
	_ = sessionID
	return 0, nil
}

