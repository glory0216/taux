package opencode

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

// Provider implements provider.Provider for OpenCode.
type Provider struct {
	dataDir string
	cache   *cache.Cache
}

// New creates a new OpenCode provider.
// dataDir defaults to ~/.local/share/opencode (OPENCODE_DATA_DIR env takes priority).
func New(dataDir string, c *cache.Cache) *Provider {
	return &Provider{dataDir: dataDir, cache: c}
}

func (p *Provider) ClearCache()       { p.cache.Clear() }
func (p *Provider) Name() string        { return "opencode" }
func (p *Provider) DisplayName() string { return "OpenCode" }

// Available returns true if the OpenCode session storage directory exists.
func (p *Provider) Available() bool {
	info, err := os.Stat(filepath.Join(p.dataDir, "storage", "session"))
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ListSession returns all discovered sessions, optionally filtered.
func (p *Provider) ListSession(ctx context.Context, filter provider.Filter) ([]model.Session, error) {
	const cacheKey = "opencode:session_list"

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

	// Enrich with active process info (count only — no session ID mapping available)
	activeProcessList, _ := FindActiveProcess()
	if len(activeProcessList) > 0 {
		// Mark the most recently active session as active if a process is running
		// (OpenCode does not expose session ID in process args)
		for i := range sessionList {
			if i < len(activeProcessList) {
				sessionList[i].Status = model.SessionActive
				sessionList[i].PID = activeProcessList[i].PID
				sessionList[i].RSS = activeProcessList[i].RSS
				sessionList[i].CPUPercent = activeProcessList[i].CPUPercent
			}
		}
	}

	p.cache.Set(cacheKey, sessionList)
	return applyFilter(sessionList, filter), nil
}

// GetSession parses the full session detail from JSON files.
func (p *Provider) GetSession(ctx context.Context, id string) (*model.SessionDetail, error) {
	cacheKey := "opencode:session:" + id

	if cached, ok := p.cache.Get(cacheKey); ok {
		if detail, ok := cached.(*model.SessionDetail); ok {
			return detail, nil
		}
	}

	detail, err := ParseSession(p.dataDir, id)
	if err != nil {
		return nil, fmt.Errorf("parse session %s: %w", id, err)
	}

	p.cache.Set(cacheKey, detail)
	return detail, nil
}

// GetStatus returns a quick provider status summary.
func (p *Provider) GetStatus(ctx context.Context) (*provider.ProviderStatus, error) {
	const cacheKey = "opencode:status"

	if cached, ok := p.cache.Get(cacheKey); ok {
		if status, ok := cached.(*provider.ProviderStatus); ok {
			return status, nil
		}
	}

	status := &provider.ProviderStatus{}

	if cached, ok := p.cache.Get("opencode:session_list"); ok {
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

// AttachSession returns the command to resume an OpenCode session.
func (p *Provider) AttachSession(id string) (string, []string, string, error) {
	detail, err := ParseSession(p.dataDir, id)
	if err != nil {
		return "opencode", nil, "", nil
	}
	return "opencode", nil, detail.CWD, nil
}

// KillSession sends SIGTERM to the opencode process.
func (p *Provider) KillSession(ctx context.Context, id string) error {
	activeList, err := FindActiveProcess()
	if err != nil {
		return fmt.Errorf("find active processes: %w", err)
	}
	if len(activeList) == 0 {
		return fmt.Errorf("no active opencode process found for session %s", id)
	}

	// Best effort: kill the first matching active process
	proc, err := os.FindProcess(activeList[0].PID)
	if err != nil {
		return fmt.Errorf("find process %d: %w", activeList[0].PID, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to pid %d: %w", activeList[0].PID, err)
	}

	p.cache.Invalidate("opencode:session_list")
	p.cache.Invalidate("opencode:session:" + id)
	p.cache.Invalidate("opencode:status")
	return nil
}

// DeleteSession removes the session JSON file and its message directory.
func (p *Provider) DeleteSession(ctx context.Context, id string) error {
	sessionFile, err := findSessionFile(p.dataDir, id)
	if err != nil {
		return fmt.Errorf("find session %s: %w", id, err)
	}
	if err := os.Remove(sessionFile); err != nil {
		return fmt.Errorf("remove session file: %w", err)
	}

	// Also remove message directory (best effort)
	msgDir := filepath.Join(p.dataDir, "storage", "message", id)
	_ = os.RemoveAll(msgDir)

	p.cache.Clear()
	return nil
}

// CleanSession removes session files (and message dirs) older than olderThan.
func (p *Provider) CleanSession(ctx context.Context, olderThan string) (int64, error) {
	duration, err := time.ParseDuration(olderThan)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", olderThan, err)
	}

	cutoff := time.Now().Add(-duration)
	var freedBytes int64

	pattern := filepath.Join(p.dataDir, "storage", "session", "*", "*.json")
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
			// Parse to get session ID for message cleanup
			var sj sessionJSON
			if data, readErr := os.ReadFile(match); readErr == nil {
				_ = json.Unmarshal(data, &sj)
			}
			freedBytes += stat.Size()
			if err := os.Remove(match); err == nil && sj.ID != "" {
				msgDir := filepath.Join(p.dataDir, "storage", "message", sj.ID)
				_ = os.RemoveAll(msgDir)
			}
		}
	}

	p.cache.Clear()
	return freedBytes, nil
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
