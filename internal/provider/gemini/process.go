package gemini

import (
	"bufio"
	"bytes"
	"os/exec"
	"strconv"
	"strings"
)

// ProcessInfo holds info about a running Gemini CLI process.
type ProcessInfo struct {
	PID        int
	SessionID  string // best-effort; Gemini --resume doesn't take session ID
	RSS        int64  // resident memory in bytes
	CPUPercent float64
}

// FindActiveProcess returns all running Gemini CLI processes.
// Note: "gemini" is a common word, so we use strict binary name matching.
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

		fieldList := strings.Fields(line)
		if len(fieldList) < 4 {
			continue
		}

		argLine := strings.Join(fieldList[3:], " ")
		if !isGeminiProcess(argLine) {
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

// isGeminiProcess checks if the command line belongs to a Gemini CLI process.
// Strict matching to avoid false positives since "gemini" is a common word.
func isGeminiProcess(argLine string) bool {
	fieldList := strings.Fields(argLine)
	if len(fieldList) == 0 {
		return false
	}

	// Skip grep/ps processes that match "gemini"
	binary := fieldList[0]
	if strings.HasSuffix(binary, "/grep") || strings.HasSuffix(binary, "/ps") {
		return false
	}

	// Extract base name from full path
	baseName := binary
	if idx := strings.LastIndex(binary, "/"); idx >= 0 {
		baseName = binary[idx+1:]
	}

	// Must be exactly "gemini" as the binary name
	return baseName == "gemini"
}

// FindProcessByPID checks if any Gemini process is running and returns the first PID.
func FindProcessByPID() (int, error) {
	processList, err := FindActiveProcess()
	if err != nil {
		return 0, err
	}
	if len(processList) > 0 {
		return processList[0].PID, nil
	}
	return 0, nil
}
