package provider

import (
	"strings"

	"github.com/glory0216/taux/internal/model"
)

// FilterSessionList applies a case-insensitive substring filter across
// session fields (ShortID, Project, Model, Status, and alias via aliasFunc).
// Shared by TUI's filter bar. Returns all sessions if filterText is empty.
func FilterSessionList(sessionList []model.Session, filterText string, aliasFunc func(string) string) []model.Session {
	if filterText == "" {
		return sessionList
	}
	lower := strings.ToLower(filterText)
	var result []model.Session
	for _, s := range sessionList {
		alias := ""
		if aliasFunc != nil {
			alias = strings.ToLower(aliasFunc(s.ID))
		}
		if strings.Contains(strings.ToLower(s.ShortID), lower) ||
			strings.Contains(strings.ToLower(s.Project), lower) ||
			strings.Contains(strings.ToLower(s.Model), lower) ||
			strings.Contains(strings.ToLower(string(s.Status)), lower) ||
			strings.Contains(alias, lower) {
			result = append(result, s)
		}
	}
	return result
}
