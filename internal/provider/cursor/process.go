package cursor

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ProcessInfo holds info about a running Cursor process.
type ProcessInfo struct {
	PID        int
	RSS        int64
	CPUPercent float64
}

// FindCursorProcess returns info about running Cursor processes.
func FindCursorProcess() ([]ProcessInfo, error) {
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

		// Match Cursor main process
		if !isCursorProcess(line) {
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

// isCursorProcess checks if a ps line represents the main Cursor process.
func isCursorProcess(line string) bool {
	lower := strings.ToLower(line)

	// Skip helper processes
	if strings.Contains(lower, "cursor helper") ||
		strings.Contains(lower, "cursor-helper") ||
		strings.Contains(lower, "crashpad") ||
		strings.Contains(lower, "gpu-process") {
		return false
	}

	// Match main Cursor process
	return strings.Contains(line, "Cursor.app/Contents/MacOS/Cursor") ||
		strings.Contains(line, "/cursor ") ||
		strings.HasSuffix(line, "/cursor")
}

// IsCursorRunning returns true if any Cursor process is active.
func IsCursorRunning() bool {
	procList, err := FindCursorProcess()
	if err != nil {
		return false
	}
	return len(procList) > 0
}
