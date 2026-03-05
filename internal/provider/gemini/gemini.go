package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/glory0216/taux/internal/cache"
	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/provider"
)

// Provider implements provider.Provider for Google Gemini CLI.
type Provider struct {
	dataDir string
	cache   *cache.Cache
}

// New creates a new Gemini CLI provider.
// dataDir is typically ~/.gemini (after path expansion).
func New(dataDir string, cache *cache.Cache) *Provider {
	return &Provider{
		dataDir: dataDir,
		cache:   cache,
	}
}

func (p *Provider) ClearCache() { p.cache.Clear() }

func (p *Provider) Name() string        { return "gemini" }
func (p *Provider) DisplayName() string  { return "Gemini CLI" }

// Available returns true if the Gemini tmp directory exists.
func (p *Provider) Available() bool {
	info, err := os.Stat(filepath.Join(p.dataDir, "tmp"))
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ListSession returns all discovered sessions, optionally filtered.
func (p *Provider) ListSession(ctx context.Context, filter provider.Filter) ([]model.Session, error) {
	const cacheKey = "gemini:session_list"

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

	// Enrich with process info (best-effort: Gemini CLI doesn't expose session IDs)
	// Mark one session per running Gemini process, starting from most recent
	activeProcessList, _ := FindActiveProcess()
	for i, proc := range activeProcessList {
		if i >= len(sessionList) {
			break
		}
		sessionList[i].Status = model.SessionActive
		sessionList[i].PID = proc.PID
		sessionList[i].RSS = proc.RSS
		sessionList[i].CPUPercent = proc.CPUPercent
	}

	p.cache.Set(cacheKey, sessionList)
	return applyFilter(sessionList, filter), nil
}

// GetSession parses the full JSON for a session and returns detailed info.
func (p *Provider) GetSession(ctx context.Context, id string) (*model.SessionDetail, error) {
	cacheKey := "gemini:session:" + id

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
		detail.LastActive = stat.ModTime()
	}
	detail.FilePath = path
	detail.ID = id

	if len(detail.ID) >= 6 {
		detail.ShortID = detail.ID[:6]
	}

	// Extract project info from path
	tmpDir := filepath.Join(p.dataDir, "tmp")
	if rel, relErr := filepath.Rel(tmpDir, path); relErr == nil {
		dirParts := strings.Split(filepath.Dir(rel), string(filepath.Separator))
		if len(dirParts) >= 1 {
			projectHash := dirParts[0]
			detail.ProjectPath = projectHash
			detail.Project = resolveProjectName(tmpDir, projectHash)
		}
	}

	// Check if active (best-effort)
	if pid, findErr := FindProcessByPID(); findErr == nil && pid > 0 {
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
	const cacheKey = "gemini:status"

	if cached, ok := p.cache.Get(cacheKey); ok {
		if status, ok := cached.(*provider.ProviderStatus); ok {
			return status, nil
		}
	}

	status := &provider.ProviderStatus{}

	if cached, ok := p.cache.Get("gemini:session_list"); ok {
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

	// Sum token usage from cached session details
	if cached, ok := p.cache.Get("gemini:session_list"); ok {
		if sessionList, ok := cached.([]model.Session); ok {
			for _, s := range sessionList {
				detailKey := "gemini:session:" + s.ID
				if dc, ok := p.cache.Get(detailKey); ok {
					if detail, ok := dc.(*model.SessionDetail); ok {
						status.TokenCount += detail.TokenUsage.Total()
					}
				}
			}
		}
	}

	p.cache.Set(cacheKey, status)
	return status, nil
}

// ActiveSession returns only sessions that have a running process.
func (p *Provider) ActiveSession(ctx context.Context) ([]model.Session, error) {
	return p.ListSession(ctx, provider.Filter{Status: model.SessionActive})
}

// AttachSession returns the command to resume a Gemini session.
// Limitation: Gemini CLI --resume does not accept a session ID,
// so it always resumes the last session in the working directory.
func (p *Provider) AttachSession(id string) (string, []string, string, error) {
	// Try to determine the project directory from the session file
	path, err := p.findSessionFile(id)
	if err != nil {
		return "gemini", []string{"--resume"}, "", nil
	}

	// Extract project hash and try to find the project path
	tmpDir := filepath.Join(p.dataDir, "tmp")
	if rel, relErr := filepath.Rel(tmpDir, path); relErr == nil {
		dirParts := strings.Split(filepath.Dir(rel), string(filepath.Separator))
		if len(dirParts) >= 1 {
			projectHash := dirParts[0]
			hashDir := filepath.Join(tmpDir, projectHash)
			// Check for metadata with project path
			for _, name := range []string{"metadata.json", "config.json"} {
				data, readErr := os.ReadFile(filepath.Join(hashDir, name))
				if readErr != nil {
					continue
				}
				var meta struct {
					ProjectPath string `json:"project_path"`
					Path        string `json:"path"`
				}
				if jsonErr := json.Unmarshal(data, &meta); jsonErr == nil {
					if meta.ProjectPath != "" {
						return "gemini", []string{"--resume"}, meta.ProjectPath, nil
					}
					if meta.Path != "" {
						return "gemini", []string{"--resume"}, meta.Path, nil
					}
				}
			}
		}
	}

	return "gemini", []string{"--resume"}, "", nil
}

// KillSession terminates any running Gemini process.
// Note: Since Gemini process → session mapping is best-effort,
// this kills the first found Gemini process.
func (p *Provider) KillSession(ctx context.Context, id string) error {
	pid, err := FindProcessByPID()
	if err != nil {
		return fmt.Errorf("find gemini process: %w", err)
	}
	if pid == 0 {
		return fmt.Errorf("no active gemini process found")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to pid %d: %w", pid, err)
	}

	p.cache.Invalidate("gemini:session_list")
	p.cache.Invalidate("gemini:session:" + id)
	p.cache.Invalidate("gemini:status")
	return nil
}

// DeleteSession removes the JSON file for a given session.
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

	tmpDir := filepath.Join(p.dataDir, "tmp")
	pattern := filepath.Join(tmpDir, "*", "chats", "*.json")
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

// findSessionFile locates the JSON file for a given session ID.
func (p *Provider) findSessionFile(id string) (string, error) {
	tmpDir := filepath.Join(p.dataDir, "tmp")
	pattern := filepath.Join(tmpDir, "*", "chats", id+".json")
	matchList, err := filepath.Glob(pattern)
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
