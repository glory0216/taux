package cursor

import "encoding/json"

// ComposerData represents a composer/conversation from cursorDiskKV.
// Key format: composerData:<composerId>
type ComposerData struct {
	Version                     int            `json:"_v"`
	ComposerID                  string         `json:"composerId"`
	Name                        string         `json:"name"`
	CreatedAt                   int64          `json:"createdAt"`
	LastUpdatedAt               int64          `json:"lastUpdatedAt"`
	Status                      string         `json:"status"`
	IsAgentic                   bool           `json:"isAgentic"`
	FullConversationHeadersOnly []BubbleHeader `json:"fullConversationHeadersOnly"`
	// Legacy v1/v2 inline conversations
	Conversation []LegacyBubble  `json:"conversation,omitempty"`
	RichText     string          `json:"richText,omitempty"`
	Text         string          `json:"text,omitempty"`
	UsageData    json.RawMessage `json:"usageData,omitempty"`
}

// BubbleHeader is a lightweight reference in fullConversationHeadersOnly.
type BubbleHeader struct {
	BubbleID string `json:"bubbleId"`
	Type     int    `json:"type"` // 1=user, 2=AI
}

// BubbleData represents an individual message from cursorDiskKV.
// Key format: bubbleId:<composerId>:<bubbleId>
type BubbleData struct {
	BubbleID   string            `json:"bubbleId"`
	Type       int               `json:"type"` // 1=user, 2=AI
	Text       string            `json:"text"`
	RawText    string            `json:"rawText"`
	Thinking   string            `json:"thinking,omitempty"`
	TokenCount *BubbleTokenCount `json:"tokenCount,omitempty"`
	TimingInfo *BubbleTimingInfo `json:"timingInfo,omitempty"`
}

// BubbleTokenCount holds token usage for a single bubble.
type BubbleTokenCount struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

// BubbleTimingInfo holds timing data for a bubble.
type BubbleTimingInfo struct {
	ClientEndTime int64 `json:"clientEndTime,omitempty"`
	Timestamp     int64 `json:"timestamp,omitempty"`
}

// LegacyBubble is the inline conversation format from v1/v2 composerData.
type LegacyBubble struct {
	Type     int    `json:"type"`
	BubbleID string `json:"bubbleId"`
	Text     string `json:"text"`
}

// WorkspaceJSON represents the workspace.json mapping file.
type WorkspaceJSON struct {
	Folder string `json:"folder"`
}
