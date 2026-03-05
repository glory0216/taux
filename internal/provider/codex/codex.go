package codex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/glory0216/taux/internal/cache"
	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/provider"
)

// Provider implements provider.Provider for OpenAI Codex CLI.
type Provider struct {
	dataDir string
	cache   *cache.Cache
}

// New creates a new Codex CLI provider.
// dataDir is typically ~/.codex (after path expansion).
func New(dataDir string, cache *cache.Cache) *Provider {
	return &Provider{
		dataDir: dataDir,
		cache:   cache,
	}
}

func (p *Provider) ClearCache() { p.cache.Clear() }

func (p *Provider) Name() string        { return "codex" }
func (p *Provider) DisplayName() string  { return "Codex CLI" }

// Available returns true if the Codex sessions directory exists.
func (p *Provider) Available() bool {
	info, err := os.Stat(filepath.Join(p.dataDir, "sessions"))
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ListSession returns all discovered sessions, optionally filtered.
func (p *Provider) ListSession(ctx context.Context, filter provider.Filter) ([]model.Session, error) {
	const cacheKey = "codex:session_list"

	if cached, ok := p.cache.Get(cacheKey); ok {
		if sessionList, ok := cached.([]model.Session); ok {
			return applyFilter(sessionList, filter), nil
		}
	}

	sessionList, err := ScanSession(p.dataDir)
	if err != nil {
		return nil, fmt.Errorf("scan sessions: %w", err)
	}

	// Detect git branch for sessions with a project path
	for i := range sessionList {
		if sessionList[i].GitBranch == "" && sessionList[i].ProjectPath != "" {
			sessionList[i].GitBranch = provider.DetectGitBranch(sessionList[i].ProjectPath)
		}
	}

	// Enrich with process info
	activeProcessList, _ := FindActiveProcess()
	activeMap := make(map[string]ProcessInfo)
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
		}
	}

	p.cache.Set(cacheKey, sessionList)
	return applyFilter(sessionList, filter), nil
}

// GetSession parses the full JSONL for a session and returns detailed info.
func (p *Provider) GetSession(ctx context.Context, id string) (*model.SessionDetail, error) {
	cacheKey := "codex:session:" + id

	if cached, ok := p.cache.Get(cacheKey); ok {
		if detail, ok := cached.(*model.SessionDetail); ok {
			return detail, nil
		}
	}

	path, err := p.findSessionFile(id)
	if err != nil {
		return nil, fmt.Errorf("find session %s: %w", id, err)
	}

	detail, err := ParseSession(path)
	if err != nil {
		return nil, fmt.Errorf("parse session %s: %w", id, err)
	}

	stat, _ := os.Stat(path)
	if stat != nil {
		detail.FileSize = stat.Size()
	}
	detail.FilePath = path
	detail.ID = id

	if len(detail.ID) >= 6 {
		detail.ShortID = detail.ID[:6]
	}

	// Check if active
	if pid, findErr := FindProcessBySession(id); findErr == nil && pid > 0 {
		detail.Status = model.SessionActive
		detail.PID = pid
	} else {
		detail.Status = model.SessionDead
	}

	p.cache.Set(cacheKey, detail)
	return detail, nil
}

// GetStatus returns a quick provider status summary.
func (p *Provider) GetStatus(ctx context.Context) (*provider.ProviderStatus, error) {
	const cacheKey = "codex:status"

	if cached, ok := p.cache.Get(cacheKey); ok {
		if status, ok := cached.(*provider.ProviderStatus); ok {
			return status, nil
		}
	}

	status := &provider.ProviderStatus{}

	// Count from cached session list if available
	if cached, ok := p.cache.Get("codex:session_list"); ok {
		if sessionList, ok := cached.([]model.Session); ok {
			status.TotalCount = len(sessionList)
			for _, s := range sessionList {
				if s.Status == model.SessionActive {
					status.ActiveCount++
				}
				status.MessageCount += s.MessageCount
			}
		}
	} else {
		// Fallback: just count active processes
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

// AttachSession returns the command to resume a Codex session.
// Extracts the session's date directory for context but Codex doesn't use workDir.
func (p *Provider) AttachSession(id string) (string, []string, string, error) {
	// Try to find CWD from the session file metadata
	path, err := p.findSessionFile(id)
	if err != nil {
		return "codex", []string{"--resume", id}, "", nil
	}

	// Parse first line for CWD
	detail, parseErr := ParseSession(path)
	if parseErr == nil && detail.CWD != "" {
		return "codex", []string{"--resume", id}, detail.CWD, nil
	}

	return "codex", []string{"--resume", id}, "", nil
}

// KillSession terminates the process for a given session.
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

	p.cache.Invalidate("codex:session_list")
	p.cache.Invalidate("codex:session:" + id)
	p.cache.Invalidate("codex:status")
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
	p.cache.Clear()
	return nil
}

// CleanSession removes session files older than the given duration.
func (p *Provider) CleanSession(ctx context.Context, olderThan string) (int64, error) {
	duration, err := time.ParseDuration(olderThan)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", olderThan, err)
	}

	cutoff := time.Now().Add(-duration)
	var freedBytes int64

	sessionsDir := filepath.Join(p.dataDir, "sessions")
	pattern := filepath.Join(sessionsDir, "*", "*", "*", "*.jsonl")
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

	p.cache.Clear()
	return freedBytes, nil
}

// findSessionFile locates the JSONL file for a given session ID.
func (p *Provider) findSessionFile(id string) (string, error) {
	sessionsDir := filepath.Join(p.dataDir, "sessions")

	// Try with rollout- prefix first
	pattern := filepath.Join(sessionsDir, "*", "*", "*", "rollout-"+id+".jsonl")
	matchList, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matchList) > 0 {
		return matchList[0], nil
	}

	// Try exact filename
	pattern = filepath.Join(sessionsDir, "*", "*", "*", id+".jsonl")
	matchList, err = filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matchList) > 0 {
		return matchList[0], nil
	}

	return "", fmt.Errorf("session file not found: %s", id)
}

// applyFilter filters and limits a session list.
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
