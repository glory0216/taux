package model

import (
	"testing"
	"time"
)

func TestTokenUsage_Total(t *testing.T) {
	usage := TokenUsage{
		InputTokens:              100,
		OutputTokens:             200,
		CacheReadInputTokens:     50,
		CacheCreationInputTokens: 30,
	}
	if got := usage.Total(); got != 380 {
		t.Errorf("Total() = %d, want 380", got)
	}
}

func TestTokenUsage_Total_Zero(t *testing.T) {
	var usage TokenUsage
	if got := usage.Total(); got != 0 {
		t.Errorf("Total() = %d, want 0", got)
	}
}

func TestSessionStatus_Constants(t *testing.T) {
	if SessionActive != "active" {
		t.Errorf("SessionActive = %q, want %q", SessionActive, "active")
	}
	if SessionDead != "dead" {
		t.Errorf("SessionDead = %q, want %q", SessionDead, "dead")
	}
}

func TestBuildProjectList_Empty(t *testing.T) {
	result := BuildProjectList(nil)
	if len(result) != 0 {
		t.Errorf("BuildProjectList(nil) returned %d projects, want 0", len(result))
	}
}

func TestBuildProjectList_Grouping(t *testing.T) {
	now := time.Now()
	sessionList := []Session{
		{Project: "projA", ProjectPath: "/home/user/projA", Status: SessionActive, FileSize: 100, RSS: 500, CPUPercent: 1.5, StartedAt: now},
		{Project: "projA", ProjectPath: "/home/user/projA", Status: SessionDead, FileSize: 200, StartedAt: now},
		{Project: "projB", ProjectPath: "/home/user/projB", Status: SessionActive, FileSize: 50, RSS: 300, CPUPercent: 0.5, StartedAt: now},
	}

	result := BuildProjectList(sessionList)

	if len(result) != 2 {
		t.Fatalf("got %d projects, want 2", len(result))
	}

	// Sorted by TotalSize descending: projA (300) > projB (50)
	if result[0].Name != "projA" {
		t.Errorf("first project = %q, want %q", result[0].Name, "projA")
	}
	if result[0].SessionCount != 2 {
		t.Errorf("projA SessionCount = %d, want 2", result[0].SessionCount)
	}
	if result[0].ActiveCount != 1 {
		t.Errorf("projA ActiveCount = %d, want 1", result[0].ActiveCount)
	}
	if result[0].TotalSize != 300 {
		t.Errorf("projA TotalSize = %d, want 300", result[0].TotalSize)
	}
	// Only active sessions contribute to RSS/CPU
	if result[0].TotalRSS != 500 {
		t.Errorf("projA TotalRSS = %d, want 500", result[0].TotalRSS)
	}
	if result[0].TotalCPU != 1.5 {
		t.Errorf("projA TotalCPU = %f, want 1.5", result[0].TotalCPU)
	}
}

func TestBuildProjectList_FallbackToProjectName(t *testing.T) {
	sessionList := []Session{
		{Project: "no-path", ProjectPath: "", FileSize: 100, Status: SessionDead},
	}
	result := BuildProjectList(sessionList)
	if len(result) != 1 {
		t.Fatalf("got %d projects, want 1", len(result))
	}
	if result[0].Name != "no-path" {
		t.Errorf("project name = %q, want %q", result[0].Name, "no-path")
	}
}
