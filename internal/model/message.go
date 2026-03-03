package model

import (
	"encoding/json"
	"time"
)

// JSONLRecord represents a single line in a Claude Code JSONL session file.
type JSONLRecord struct {
	UUID        string          `json:"uuid"`
	ParentUUID  string          `json:"parentUuid"`
	SessionID   string          `json:"sessionId"`
	Type        string          `json:"type"` // "user", "assistant", "progress", "result", "summary"
	Timestamp   time.Time       `json:"timestamp"`
	CWD         string          `json:"cwd"`
	Version     string          `json:"version"`
	GitBranch   string          `json:"gitBranch"`
	TeamName    string          `json:"teamName,omitempty"`
	AgentName   string          `json:"agentName,omitempty"`
	RequestID   string          `json:"requestId,omitempty"`
	Message     json.RawMessage `json:"message,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
	IsSidechain bool            `json:"isSidechain"`
}

// AssistantMessage is the parsed message content from an assistant record.
type AssistantMessage struct {
	Model      string          `json:"model"`
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	StopReason *string         `json:"stop_reason"`
	Usage      *MessageUsage   `json:"usage"`
}

// MessageUsage is the token usage for a single assistant message.
type MessageUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// ContentBlock represents a single block in an assistant message content array.
type ContentBlock struct {
	Type  string `json:"type"` // "text", "tool_use", "tool_result"
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Text  string `json:"text,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}
