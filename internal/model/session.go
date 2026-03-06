package model

import "time"

// SessionStatus represents the lifecycle state of an agent session.
type SessionStatus string

const (
	SessionActive SessionStatus = "active"
	SessionDead   SessionStatus = "dead"
)

// Session is a lightweight summary for list views.
type Session struct {
	ID           string        `json:"id"`
	ShortID      string        `json:"short_id"`
	Provider     string        `json:"provider"`
	Status       SessionStatus `json:"status"`
	Project      string        `json:"project"`
	ProjectPath  string        `json:"project_path"`
	Model        string        `json:"model"`
	Description  string        `json:"description"`
	Environment  string        `json:"environment"` // "cli", "ide" (Cursor/VSCode)
	CWD          string        `json:"cwd"`
	GitBranch    string        `json:"git_branch"`
	MessageCount int           `json:"message_count"`
	StartedAt    time.Time     `json:"started_at"`
	LastActive   time.Time     `json:"last_active"`
	FilePath     string        `json:"file_path"`
	FileSize     int64         `json:"file_size"`
	PID          int           `json:"pid,omitempty"`
	RSS          int64         `json:"rss,omitempty"`        // resident memory in bytes
	CPUPercent   float64       `json:"cpu_percent,omitempty"`
}

// SessionDetail holds the full detail view of a single session.
type SessionDetail struct {
	Session
	Version       string       `json:"version"`
	GitBranch     string       `json:"git_branch"`
	TokenUsage    TokenUsage   `json:"token_usage"`
	ToolCallCount int          `json:"tool_call_count"`
	ContextUsed    int            `json:"context_used"`
	ContextMax     int            `json:"context_max"`
	ToolUsage     map[string]int `json:"tool_usage"`
	TaskList      []Task       `json:"task_list,omitempty"`
	TeamName      string       `json:"team_name,omitempty"`
	AgentName     string       `json:"agent_name,omitempty"`
}

// TokenUsage tracks token consumption across models.
type TokenUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

func (t TokenUsage) Total() int {
	return t.InputTokens + t.OutputTokens + t.CacheReadInputTokens + t.CacheCreationInputTokens
}
