package claude

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ProcessInfo holds info about a running Claude Code process.
type ProcessInfo struct {
	PID        int
	SessionID  string
	CWD        string
	RSS        int64   // resident memory in KB (from ps)
	CPUPercent float64 // %CPU (from ps)
}

// uuidInTaskPath matches a UUID in a .claude/tasks/<uuid> path.
var uuidInTaskPath = regexp.MustCompile(`/\.claude/tasks/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)

// FindActiveProcess returns all running Claude Code processes.
// It parses `ps -eo pid,rss,pcpu,args` to find claude processes with resource info.
// For CLI processes without --resume (new sessions), it falls back to lsof to
// find the session ID from open .claude/tasks/<uuid> dirs.
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

		// Skip lines that don't involve claude
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "claude") {
			continue
		}
		// Skip our own ps process
		if strings.Contains(line, "ps -eo") {
			continue
		}

		// Parse: PID RSS %CPU ARGS...
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

		// Args start at field 3
		argLine := strings.Join(fieldList[3:], " ")
		sessionID := extractResumeSessionID(argLine)

		// Determine if this is an IDE process
		isIDE := strings.Contains(argLine, "--output-format") ||
			strings.Contains(argLine, "stream-json") ||
			strings.Contains(argLine, "vscode") ||
			strings.Contains(argLine, "cursor")

		info := ProcessInfo{
			PID:        pid,
			SessionID:  sessionID,
			RSS:        rssKB * 1024, // convert KB to bytes
			CPUPercent: cpuPct,
		}

		// Only try lsof for CLI processes without --resume
		if sessionID == "" && !isIDE {
			if sid := findSessionIDByLsof(pid); sid != "" {
				info.SessionID = sid
			}
		}

		result = append(result, info)
	}

	return result, nil
}

// findSessionIDByLsof uses lsof to find a session ID from open
// .claude/tasks/<uuid> directories for a given PID.
func findSessionIDByLsof(pid int) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lsof", "-p", fmt.Sprintf("%d", pid))
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Find the most frequently referenced session ID (the process's own session).
	countMap := make(map[string]int)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		matchList := uuidInTaskPath.FindStringSubmatch(scanner.Text())
		if len(matchList) >= 2 {
			countMap[matchList[1]]++
		}
	}

	if len(countMap) == 0 {
		return ""
	}

	var bestID string
	var bestCount int
	for id, count := range countMap {
		if count > bestCount {
			bestID = id
			bestCount = count
		}
	}
	return bestID
}

// extractResumeSessionID extracts the session ID from a "--resume <id>" flag
// in a command line string. Returns empty string if not found.
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

// BuildPPIDMap returns a pid→ppid map for all running processes by parsing
// a single "ps -eo pid,ppid" call. Pass the result to IsChildOf to avoid
// spawning one subprocess per PPID-chain hop.
func BuildPPIDMap() map[int]int {
	out, err := exec.Command("ps", "-eo", "pid,ppid").Output()
	if err != nil {
		return nil
	}
	m := make(map[int]int)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 == nil && err2 == nil && pid > 0 {
			m[pid] = ppid
		}
	}
	return m
}

// IsChildOf checks whether pid is a descendant of ancestorPID by walking
// the pre-built ppidMap in memory. Returns false if ppidMap is nil or the
// chain cannot be resolved.
func IsChildOf(pid, ancestorPID int, ppidMap map[int]int) bool {
	if pid == ancestorPID {
		return true
	}
	if ppidMap == nil {
		return false
	}
	visited := make(map[int]bool)
	current := pid
	for current > 1 && !visited[current] {
		visited[current] = true
		ppid, ok := ppidMap[current]
		if !ok {
			return false
		}
		if ppid == ancestorPID {
			return true
		}
		current = ppid
	}
	return false
}

// IsWorktree returns true if the given directory is inside a git worktree
// (i.e., .git is a file pointing to the main repo, not a directory).
func IsWorktree(cwd string) bool {
	info, err := os.Lstat(cwd + "/.git")
	if err != nil {
		return false
	}
	return !info.IsDir() // worktree has .git as a file, not a directory
}

