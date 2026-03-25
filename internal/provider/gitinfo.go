package provider

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const gitBranchCacheTTL = 60 * time.Second

type gitBranchEntry struct {
	branch    string
	expiresAt time.Time
}

var (
	gitBranchCache   = make(map[string]gitBranchEntry)
	gitBranchCacheMu sync.RWMutex
)

// DetectGitBranch runs git rev-parse to detect the current branch for a project path.
// Results are cached per projectPath with a 60s TTL. Returns empty string on failure.
func DetectGitBranch(projectPath string) string {
	if projectPath == "" {
		return ""
	}

	now := time.Now()
	gitBranchCacheMu.RLock()
	if entry, ok := gitBranchCache[projectPath]; ok && now.Before(entry.expiresAt) {
		gitBranchCacheMu.RUnlock()
		return entry.branch
	}
	gitBranchCacheMu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", projectPath, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()

	branch := ""
	if err == nil {
		branch = strings.TrimSpace(string(out))
	}

	gitBranchCacheMu.Lock()
	gitBranchCache[projectPath] = gitBranchEntry{branch: branch, expiresAt: now.Add(gitBranchCacheTTL)}
	gitBranchCacheMu.Unlock()

	return branch
}
