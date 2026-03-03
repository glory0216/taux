package cursor

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/glory0216/taux/internal/cache"
	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/provider"
)

// Provider implements provider.Provider for Cursor AI.
type Provider struct {
	dataDir string
	cache   *cache.Cache
}

// New creates a new Cursor provider.
// dataDir is typically ~/Library/Application Support/Cursor/User (macOS)
// or ~/.config/Cursor/User (Linux).
func New(dataDir string, cache *cache.Cache) *Provider {
	return &Provider{
		dataDir: dataDir,
		cache:   cache,
	}
}

func (p *Provider) ClearCache() { p.cache.Clear() }

func (p *Provider) Name() string       { return "cursor" }
func (p *Provider) DisplayName() string { return "Cursor" }

// Available returns true if the Cursor data directory exists with globalStorage.
func (p *Provider) Available() bool {
	dbPath := globalDBPath(p.dataDir)
	_, err := os.Stat(dbPath)
	return err == nil
}

// ListSession returns all discovered Cursor sessions.
func (p *Provider) ListSession(ctx context.Context, filter provider.Filter) ([]model.Session, error) {
	const cacheKey = "cursor:session_list"

	if cached, ok := p.cache.Get(cacheKey); ok {
		if sessionList, ok := cached.([]model.Session); ok {
			return applyFilter(sessionList, filter), nil
		}
	}

	sessionList, err := ScanSession(p.dataDir)
	if err != nil {
		return nil, fmt.Errorf("scan cursor sessions: %w", err)
	}

	// Enrich with process info — mark only the most recent session as active
	// (Cursor has a single main process; we can't reliably map processes to sessions)
	procList, _ := FindCursorProcess()
	if len(procList) > 0 && len(sessionList) > 0 {
		proc := procList[0]
		// Only mark the first (most recent by LastActive) session as active
		sessionList[0].Status = model.SessionActive
		sessionList[0].PID = proc.PID
		sessionList[0].RSS = proc.RSS
		sessionList[0].CPUPercent = proc.CPUPercent
	}

	p.cache.Set(cacheKey, sessionList)
	return applyFilter(sessionList, filter), nil
}

// GetSession parses full detail for a Cursor session.
func (p *Provider) GetSession(ctx context.Context, id string) (*model.SessionDetail, error) {
	cacheKey := "cursor:session:" + id

	if cached, ok := p.cache.Get(cacheKey); ok {
		if detail, ok := cached.(*model.SessionDetail); ok {
			return detail, nil
		}
	}

	detail, err := ParseSession(p.dataDir, id)
	if err != nil {
		return nil, fmt.Errorf("parse cursor session %s: %w", id, err)
	}
	if detail == nil {
		return nil, fmt.Errorf("cursor session not found: %s", id)
	}

	p.cache.Set(cacheKey, detail)
	return detail, nil
}

// GetStatus returns a quick status summary.
func (p *Provider) GetStatus(ctx context.Context) (*provider.ProviderStatus, error) {
	const cacheKey = "cursor:status"

	if cached, ok := p.cache.Get(cacheKey); ok {
		if status, ok := cached.(*provider.ProviderStatus); ok {
			return status, nil
		}
	}

	status := &provider.ProviderStatus{}

	// Count from cached session list
	if cached, ok := p.cache.Get("cursor:session_list"); ok {
		if sessionList, ok := cached.([]model.Session); ok {
			status.TotalCount = len(sessionList)
			for _, s := range sessionList {
				if s.Status == model.SessionActive {
					status.ActiveCount++
				}
				status.MessageCount += s.MessageCount
			}
		}
	}

	p.cache.Set(cacheKey, status)
	return status, nil
}

// ActiveSession returns only active sessions.
func (p *Provider) ActiveSession(ctx context.Context) ([]model.Session, error) {
	return p.ListSession(ctx, provider.Filter{Status: model.SessionActive})
}

// AttachSession opens Cursor to the session's project directory.
func (p *Provider) AttachSession(id string) (string, []string, string, error) {
	// Find project path from cached sessions
	if cached, ok := p.cache.Get("cursor:session_list"); ok {
		if sessionList, ok := cached.([]model.Session); ok {
			for _, s := range sessionList {
				if s.ID == id && s.ProjectPath != "" {
					if runtime.GOOS == "darwin" {
						return "open", []string{"-a", "Cursor", s.ProjectPath}, "", nil
					}
					return "cursor", []string{s.ProjectPath}, "", nil
				}
			}
		}
	}

	if runtime.GOOS == "darwin" {
		return "open", []string{"-a", "Cursor"}, "", nil
	}
	return "cursor", nil, "", nil
}

// KillSession is not supported for Cursor (IDE sessions).
func (p *Provider) KillSession(_ context.Context, _ string) error {
	return fmt.Errorf("cursor provider does not support killing individual sessions")
}

// DeleteSession removes a Cursor composer session from the SQLite DB.
func (p *Provider) DeleteSession(_ context.Context, id string) error {
	dbPath := globalDBPath(p.dataDir)
	db, err := openDBReadWrite(dbPath)
	if err != nil {
		return fmt.Errorf("open cursor db for write: %w", err)
	}
	if db == nil {
		return fmt.Errorf("cursor db not found")
	}
	defer db.Close()

	deleted, err := deleteComposerData(db, id)
	if err != nil {
		return fmt.Errorf("delete cursor session %s: %w", id, err)
	}
	if deleted == 0 {
		return fmt.Errorf("cursor session not found: %s", id)
	}

	p.cache.Invalidate("cursor:session_list")
	p.cache.Invalidate("cursor:session:" + id)
	p.cache.Invalidate("cursor:status")
	return nil
}

// CleanSession is a no-op for Cursor.
func (p *Provider) CleanSession(_ context.Context, _ string) (int64, error) {
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
