package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/glory0216/taux/internal/cache"
	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/provider"
)

// Provider implements provider.Provider for Claude Code.
type Provider struct {
	dataDir string
	cache   *cache.Cache
}

// New creates a new Claude Code provider.
// dataDir is typically ~/.claude (after path expansion).
func New(dataDir string, cache *cache.Cache) *Provider {
	return &Provider{
		dataDir: dataDir,
		cache:   cache,
	}
}

func (p *Provider) ClearCache() { p.cache.Clear() }

// Name returns the short identifier for this provider.
func (p *Provider) Name() string {
	return "claude"
}

// DisplayName returns the human-readable name.
func (p *Provider) DisplayName() string {
	return "Claude Code"
}

// Available returns true if the Claude Code data directory exists with a
// projects subdirectory.
func (p *Provider) Available() bool {
	info, err := os.Stat(filepath.Join(p.dataDir, "projects"))
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ListSession returns all discovered sessions, optionally filtered.
// It enriches each session with live process info to determine active status.
func (p *Provider) ListSession(ctx context.Context, filter provider.Filter) ([]model.Session, error) {
	const cacheKey = "claude:session_list"

	// Check cache first
	if cached, ok := p.cache.Get(cacheKey); ok {
		if sessionList, ok := cached.([]model.Session); ok {
			return applyFilter(sessionList, filter), nil
		}
	}

	// Scan filesystem
	sessionList, err := ScanSession(p.dataDir)
	if err != nil {
		return nil, fmt.Errorf("scan sessions: %w", err)
	}

	// Enrich with process info
	activeProcessList, _ := FindActiveProcess()
	activeMap := make(map[string]ProcessInfo) // sessionID -> ProcessInfo
	for _, proc := range activeProcessList {
		if proc.SessionID != "" {
			activeMap[proc.SessionID] = proc
		}
	}

	for i := range sessionList {
		if proc, ok := activeMap[sessionList[i].ID]; ok {
			sessionList[i].Status = model.SessionActive
			sessionList[i].PID = proc.PID
			sessionList[i].RSS = proc.RSS
			sessionList[i].CPUPercent = proc.CPUPercent
			// Detect session state (working vs waiting for input)
			state := DetectSessionState(sessionList[i].ID, p.dataDir)
			switch state {
			case StateWaitingInput:
				sessionList[i].State = model.StateWaitingInput
			case StateWorking:
				sessionList[i].State = model.StateWorking
			}
		}
	}

	p.cache.Set(cacheKey, sessionList)
	return applyFilter(sessionList, filter), nil
}

// GetSession parses the full JSONL for a session and returns detailed info.
func (p *Provider) GetSession(ctx context.Context, id string) (*model.SessionDetail, error) {
	cacheKey := "claude:session:" + id

	if cached, ok := p.cache.Get(cacheKey); ok {
		if detail, ok := cached.(*model.SessionDetail); ok {
			return detail, nil
		}
	}

	// Find the JSONL file for this session
	path, err := p.findSessionFile(id)
	if err != nil {
		return nil, fmt.Errorf("find session %s: %w", id, err)
	}

	detail, err := ParseSession(path)
	if err != nil {
		return nil, fmt.Errorf("parse session %s: %w", id, err)
	}

	// File info
	stat, _ := os.Stat(path)
	if stat != nil {
		detail.FileSize = stat.Size()
	}
	detail.FilePath = path

	// Derive project info from path
	projectsDir := filepath.Join(p.dataDir, "projects")
	rel, err := filepath.Rel(projectsDir, path)
	if err == nil {
		dirName := filepath.Dir(rel)
		detail.ProjectPath = decodeProjectPath(dirName)
		detail.Project = filepath.Base(detail.ProjectPath)
	}

	// Check if active — use full process info for RSS/CPU
	detail.Status = model.SessionDead
	activeProcessList, _ := FindActiveProcess()
	for _, proc := range activeProcessList {
		if proc.SessionID == id {
			detail.Status = model.SessionActive
			detail.PID = proc.PID
			detail.RSS = proc.RSS
			detail.CPUPercent = proc.CPUPercent
			break
		}
	}

	p.cache.Set(cacheKey, detail)
	return detail, nil
}

// GetStatus reads stats-cache.json for a quick provider status summary.
func (p *Provider) GetStatus(ctx context.Context) (*provider.ProviderStatus, error) {
	const cacheKey = "claude:status"

	if cached, ok := p.cache.Get(cacheKey); ok {
		if status, ok := cached.(*provider.ProviderStatus); ok {
			return status, nil
		}
	}

	status := &provider.ProviderStatus{}

	// Read stats-cache.json
	statsPath := filepath.Join(p.dataDir, "stats-cache.json")
	data, err := os.ReadFile(statsPath)
	if err == nil {
		var statsCache model.StatsCache
		if jsonErr := json.Unmarshal(data, &statsCache); jsonErr == nil {
			status.TotalCount = statsCache.TotalSessions
			status.MessageCount = statsCache.TotalMessages

			// Sum tokens from model usage
			for _, usage := range statsCache.ModelUsage {
				status.TokenCount += usage.InputTokens + usage.OutputTokens +
					usage.CacheReadInputTokens + usage.CacheCreationInputTokens
			}
		}
	}

	// Count active sessions from cached session list (avoids duplicate ps+lsof)
	if cached, ok := p.cache.Get("claude:session_list"); ok {
		if sessionList, ok := cached.([]model.Session); ok {
			for _, s := range sessionList {
				if s.Status == model.SessionActive {
					status.ActiveCount++
				}
			}
		}
	} else {
		// Fallback if session list not cached yet
		activeList, _ := FindActiveProcess()
		status.ActiveCount = len(activeList)
	}

	p.cache.Set(cacheKey, status)
	return status, nil
}

// ActiveSession returns only sessions that have a running process.
func (p *Provider) ActiveSession(ctx context.Context) ([]model.Session, error) {
	return p.ListSession(ctx, provider.Filter{Status: model.SessionActive})
}

// AttachSession returns the command, arguments, and working directory to resume a session.
func (p *Provider) AttachSession(id string) (string, []string, string, error) {
	// Find the session file to determine its project path
	path, err := p.findSessionFile(id)
	if err != nil {
		return "claude", []string{"--resume", id}, "", nil
	}

	projectsDir := filepath.Join(p.dataDir, "projects")
	rel, err := filepath.Rel(projectsDir, path)
	if err != nil {
		return "claude", []string{"--resume", id}, "", nil
	}

	dirName := filepath.Dir(rel)
	projectPath := decodeProjectPath(dirName)

	return "claude", []string{"--resume", id}, projectPath, nil
}

// KillSession terminates the process for a given session by sending SIGTERM.
func (p *Provider) KillSession(ctx context.Context, id string) error {
	pid, err := FindProcessBySession(id)
	if err != nil {
		return fmt.Errorf("find process for session %s: %w", id, err)
	}
	if pid == 0 {
		return fmt.Errorf("no active process found for session %s", id)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to pid %d: %w", pid, err)
	}

	// Invalidate cached session list
	p.cache.Invalidate("claude:session_list")
	p.cache.Invalidate("claude:session:" + id)

	return nil
}

// DeleteSession removes the JSONL file for a given session.
func (p *Provider) DeleteSession(ctx context.Context, id string) error {
	path, err := p.findSessionFile(id)
	if err != nil {
		return fmt.Errorf("find session %s: %w", id, err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove session file %s: %w", path, err)
	}
	p.cache.Invalidate("claude:session_list")
	p.cache.Invalidate("claude:session:" + id)
	p.cache.Invalidate("claude:status")
	return nil
}

// CleanSession removes session JSONL files older than the given duration string.
// Returns the number of bytes freed.
func (p *Provider) CleanSession(ctx context.Context, olderThan string) (int64, error) {
	duration, err := time.ParseDuration(olderThan)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", olderThan, err)
	}

	cutoff := time.Now().Add(-duration)
	var freedBytes int64

	projectsDir := filepath.Join(p.dataDir, "projects")
	pattern := filepath.Join(projectsDir, "*", "*.jsonl")
	matchList, err := filepath.Glob(pattern)
	if err != nil {
		return 0, fmt.Errorf("glob session files: %w", err)
	}

	for _, match := range matchList {
		stat, err := os.Stat(match)
		if err != nil {
			continue
		}
		if stat.ModTime().Before(cutoff) {
			size := stat.Size()
			if err := os.Remove(match); err == nil {
				freedBytes += size
			}
		}
	}

	// Invalidate cache after cleanup
	p.cache.Clear()

	return freedBytes, nil
}

// findSessionFile locates the JSONL file for a given session ID by scanning
// all project directories.
func (p *Provider) findSessionFile(id string) (string, error) {
	projectsDir := filepath.Join(p.dataDir, "projects")
	pattern := filepath.Join(projectsDir, "*", id+".jsonl")
	matchList, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matchList) == 0 {
		return "", fmt.Errorf("session file not found: %s", id)
	}
	return matchList[0], nil
}

// applyFilter filters and limits a session list based on the given criteria.
func applyFilter(sessionList []model.Session, filter provider.Filter) []model.Session {
	var result []model.Session
	for _, s := range sessionList {
		if filter.Status != "" && s.Status != filter.Status {
			continue
		}
		if filter.Project != "" && s.Project != filter.Project {
			continue
		}
		result = append(result, s)
	}
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	return result
}
