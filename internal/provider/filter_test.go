package provider

import (
	"testing"

	"github.com/glory0216/taux/internal/model"
)

func TestFilterSessionList_EmptyFilter(t *testing.T) {
	sessionList := []model.Session{
		{ShortID: "abc123", Project: "myproj"},
		{ShortID: "def456", Project: "other"},
	}
	result := FilterSessionList(sessionList, "", nil)
	if len(result) != 2 {
		t.Errorf("empty filter: got %d sessions, want 2", len(result))
	}
}

func TestFilterSessionList_MatchShortID(t *testing.T) {
	sessionList := []model.Session{
		{ShortID: "abc123", Project: "proj1"},
		{ShortID: "def456", Project: "proj2"},
	}
	result := FilterSessionList(sessionList, "abc", nil)
	if len(result) != 1 || result[0].ShortID != "abc123" {
		t.Errorf("filter by ShortID: got %v", result)
	}
}

func TestFilterSessionList_MatchProject(t *testing.T) {
	sessionList := []model.Session{
		{ShortID: "aaa", Project: "frontend"},
		{ShortID: "bbb", Project: "backend"},
	}
	result := FilterSessionList(sessionList, "front", nil)
	if len(result) != 1 || result[0].Project != "frontend" {
		t.Errorf("filter by Project: got %v", result)
	}
}

func TestFilterSessionList_MatchModel(t *testing.T) {
	sessionList := []model.Session{
		{ShortID: "aaa", Model: "claude-opus-4"},
		{ShortID: "bbb", Model: "gpt-4o"},
	}
	result := FilterSessionList(sessionList, "opus", nil)
	if len(result) != 1 || result[0].Model != "claude-opus-4" {
		t.Errorf("filter by Model: got %v", result)
	}
}

func TestFilterSessionList_MatchStatus(t *testing.T) {
	sessionList := []model.Session{
		{ShortID: "aaa", Status: model.SessionActive},
		{ShortID: "bbb", Status: model.SessionDead},
	}
	result := FilterSessionList(sessionList, "active", nil)
	if len(result) != 1 || result[0].Status != model.SessionActive {
		t.Errorf("filter by Status: got %v", result)
	}
}

func TestFilterSessionList_CaseInsensitive(t *testing.T) {
	sessionList := []model.Session{
		{ShortID: "aaa", Project: "MyProject"},
	}
	result := FilterSessionList(sessionList, "MYPROJECT", nil)
	if len(result) != 1 {
		t.Errorf("case-insensitive match failed: got %d, want 1", len(result))
	}
}

func TestFilterSessionList_MatchAlias(t *testing.T) {
	sessionList := []model.Session{
		{ID: "full-id-1", ShortID: "aaa", Project: "proj"},
		{ID: "full-id-2", ShortID: "bbb", Project: "proj"},
	}
	aliasFunc := func(id string) string {
		if id == "full-id-1" {
			return "my-alias"
		}
		return ""
	}
	result := FilterSessionList(sessionList, "alias", aliasFunc)
	if len(result) != 1 || result[0].ID != "full-id-1" {
		t.Errorf("filter by alias: got %v", result)
	}
}

func TestFilterSessionList_NilAliasFunc(t *testing.T) {
	sessionList := []model.Session{
		{ShortID: "aaa", Project: "proj"},
	}
	// Should not panic with nil aliasFunc
	result := FilterSessionList(sessionList, "proj", nil)
	if len(result) != 1 {
		t.Errorf("nil aliasFunc: got %d, want 1", len(result))
	}
}

func TestFilterSessionList_NoMatch(t *testing.T) {
	sessionList := []model.Session{
		{ShortID: "aaa", Project: "proj", Model: "opus"},
	}
	result := FilterSessionList(sessionList, "zzzzz", nil)
	if len(result) != 0 {
		t.Errorf("no match: got %d sessions, want 0", len(result))
	}
}
