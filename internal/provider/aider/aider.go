package aider

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/glory0216/taux/internal/cache"
	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/provider"
)

// Provider implements provider.Provider for aider.
type Provider struct {
	scanDirList []string
	cache       *cache.Cache
}

// New creates a new aider provider.
func New(scanDirList []string, cache *cache.Cache) *Provider {
	return &Provider{
		scanDirList: scanDirList,
		cache:       cache,
	}
}

func (p *Provider) Name() string        { return "aider" }
func (p *Provider) DisplayName() string { return "Aider" }
func (p *Provider) ClearCache()         { p.cache.Clear() }

// Available returns true if scan directories are configured and at least
// one .aider.chat.history.md file exists.
func (p *Provider) Available() bool {
	if len(p.scanDirList) == 0 {
		return false
	}
	fileList := discoverHistoryFileList(p.scanDirList)
	return len(fileList) > 0
}

// ListSession returns all discovered aider sessions.
func (p *Provider) ListSession(ctx context.Context, filter provider.Filter) ([]model.Session, error) {
	const cacheKey = "aider:session_list"

	if cached, ok := p.cache.Get(cacheKey); ok {
		if sessionList, ok := cached.([]model.Session); ok {
			return applyFilter(sessionList, filter), nil
		}
	}

	sessionList, err := ScanSession(p.scanDirList)
	if err != nil {
		return nil, fmt.Errorf("scan aider sessions: %w", err)
	}

	// Detect git branch for sessions with a project path
	for i := range sessionList {
		if sessionList[i].GitBranch == "" && sessionList[i].ProjectPath != "" {
			sessionList[i].GitBranch = provider.DetectGitBranch(sessionList[i].ProjectPath)
		}
	}

	// Enrich with process info
	procList, _ := FindAiderProcess()
	if len(procList) > 0 && len(sessionList) > 0 {
		// If aider is running, mark the most recently active session as active.
		// (Aider runs in a single terminal per project; precise CWD matching
		// is unreliable from ps output.)
		sessionList[0].Status = model.SessionActive
		sessionList[0].PID = procList[0].PID
		sessionList[0].RSS = procList[0].RSS
		sessionList[0].CPUPercent = procList[0].CPUPercent
	}

	p.cache.Set(cacheKey, sessionList)
	return applyFilter(sessionList, filter), nil
}

// GetSession returns detail for a specific aider session.
func (p *Provider) GetSession(ctx context.Context, id string) (*model.SessionDetail, error) {
	cacheKey := "aider:session:" + id
	if cached, ok := p.cache.Get(cacheKey); ok {
		if detail, ok := cached.(*model.SessionDetail); ok {
			return detail, nil
		}
	}

	sessionList, err := ScanSession(p.scanDirList)
	if err != nil {
		return nil, err
	}

	for _, s := range sessionList {
		if s.ID == id {
			detail := &model.SessionDetail{
				Session:   s,
				ToolUsage: make(map[string]int),
			}
			p.cache.Set(cacheKey, detail)
			return detail, nil
		}
	}
	return nil, fmt.Errorf("aider session not found: %s", id)
}

// GetStatus returns a quick status summary.
func (p *Provider) GetStatus(ctx context.Context) (*provider.ProviderStatus, error) {
	const cacheKey = "aider:status"
	if cached, ok := p.cache.Get(cacheKey); ok {
		if status, ok := cached.(*provider.ProviderStatus); ok {
			return status, nil
		}
	}

	status := &provider.ProviderStatus{}
	sessionList, err := p.ListSession(ctx, provider.Filter{})
	if err == nil {
		status.TotalCount = len(sessionList)
		for _, s := range sessionList {
			if s.Status == model.SessionActive {
				status.ActiveCount++
			}
			status.MessageCount += s.MessageCount
		}
	}

	p.cache.Set(cacheKey, status)
	return status, nil
}

// ActiveSession returns only active sessions.
func (p *Provider) ActiveSession(ctx context.Context) ([]model.Session, error) {
	return p.ListSession(ctx, provider.Filter{Status: model.SessionActive})
}

// AttachSession returns the command to launch aider in the project directory.
func (p *Provider) AttachSession(id string) (string, []string, string, error) {
	sessionList, _ := ScanSession(p.scanDirList)
	for _, s := range sessionList {
		if s.ID == id {
			return "aider", nil, s.ProjectPath, nil
		}
	}
	return "aider", nil, "", nil
}

// KillSession terminates an aider process by PID.
func (p *Provider) KillSession(ctx context.Context, id string) error {
	sessionList, _ := p.ListSession(ctx, provider.Filter{})
	for _, s := range sessionList {
		if s.ID == id && s.PID > 0 {
			proc, err := os.FindProcess(s.PID)
			if err != nil {
				return fmt.Errorf("find process %d: %w", s.PID, err)
			}
			if err := proc.Signal(syscall.SIGTERM); err != nil {
				return fmt.Errorf("send SIGTERM to pid %d: %w", s.PID, err)
			}
			p.cache.Clear()
			return nil
		}
	}
	return fmt.Errorf("no active process found for aider session %s", id)
}

// DeleteSession removes the aider history file.
func (p *Provider) DeleteSession(ctx context.Context, id string) error {
	sessionList, _ := ScanSession(p.scanDirList)
	for _, s := range sessionList {
		if s.ID == id {
			if err := os.Remove(s.FilePath); err != nil {
				return fmt.Errorf("remove aider history: %w", err)
			}
			p.cache.Clear()
			return nil
		}
	}
	return fmt.Errorf("aider session not found: %s", id)
}

// CleanSession is a no-op for aider (history files are per-project).
func (p *Provider) CleanSession(ctx context.Context, olderThan string) (int64, error) {
	return 0, nil
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
