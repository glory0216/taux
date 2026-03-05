package provider

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	gitBranchCache   = make(map[string]string)
	gitBranchCacheMu sync.RWMutex
)

// DetectGitBranch runs git rev-parse to detect the current branch for a project path.
// Results are cached per projectPath. Returns empty string on failure.
func DetectGitBranch(projectPath string) string {
	if projectPath == "" {
		return ""
	}

	gitBranchCacheMu.RLock()
	if branch, ok := gitBranchCache[projectPath]; ok {
		gitBranchCacheMu.RUnlock()
		return branch
	}
	gitBranchCacheMu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", projectPath, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		gitBranchCacheMu.Lock()
		gitBranchCache[projectPath] = ""
		gitBranchCacheMu.Unlock()
		return ""
	}

	branch := strings.TrimSpace(string(out))

	gitBranchCacheMu.Lock()
	gitBranchCache[projectPath] = branch
	gitBranchCacheMu.Unlock()

	return branch
}
