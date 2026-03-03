package model

import "sort"

// Project groups sessions by their working directory.
type Project struct {
	Name         string  `json:"name"`
	Path         string  `json:"path"`
	SessionCount int     `json:"session_count"`
	ActiveCount  int     `json:"active_count"`
	TotalSize    int64   `json:"total_size"`
	TotalRSS     int64   `json:"total_rss"`
	TotalCPU     float64 `json:"total_cpu"`
}

// BuildProjectList aggregates sessions by project directory and returns
// a sorted list (by TotalSize descending). Shared by CLI and TUI.
func BuildProjectList(sessionList []Session) []Project {
	projectMap := make(map[string]*Project)
	for _, s := range sessionList {
		key := s.ProjectPath
		if key == "" {
			key = s.Project
		}
		p, ok := projectMap[key]
		if !ok {
			p = &Project{
				Name: s.Project,
				Path: s.ProjectPath,
			}
			projectMap[key] = p
		}
		p.SessionCount++
		if s.Status == SessionActive {
			p.ActiveCount++
			p.TotalRSS += s.RSS
			p.TotalCPU += s.CPUPercent
		}
		p.TotalSize += s.FileSize
	}

	var result []Project
	for _, p := range projectMap {
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalSize > result[j].TotalSize
	})
	return result
}
