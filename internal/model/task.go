package model

// Task represents a Claude Code task (from ~/.claude/tasks/).
type Task struct {
	ID          string `json:"id"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Status      string `json:"status"` // "pending", "in_progress", "completed"
	Owner       string `json:"owner,omitempty"`
}
