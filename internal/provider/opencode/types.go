package opencode

// sessionJSON is the on-disk format of a session file:
// storage/session/{projectID}/{sessionID}.json
type sessionJSON struct {
	ID        string      `json:"id"`
	Version   string      `json:"version"`
	ProjectID string      `json:"projectID"`
	Directory string      `json:"directory"`
	Title     string      `json:"title"`
	Time      timeJSON    `json:"time"`
	Summary   summaryJSON `json:"summary"`
}

// timeJSON holds millisecond-epoch timestamps.
// The value may be a JSON number or string depending on OpenCode version.
type timeJSON struct {
	Created interface{} `json:"created"`
	Updated interface{} `json:"updated"`
}

type summaryJSON struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Files     int `json:"files"`
}

// messageJSON is the on-disk format of a message file:
// storage/message/{sessionID}/msg_{messageID}.json
type messageJSON struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionID"`
	Role      string    `json:"role"` // "user" | "assistant"
	Model     string    `json:"model"`
	Parts     []partJSON `json:"parts"`
	Usage     usageJSON  `json:"usage"`
	Time      timeJSON   `json:"time"`
}

type partJSON struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type usageJSON struct {
	InputTokens         int `json:"inputTokens"`
	OutputTokens        int `json:"outputTokens"`
	CacheCreationTokens int `json:"cacheCreationTokens"`
	CacheReadTokens     int `json:"cacheReadTokens"`
}

// ProcessInfo holds info about a running opencode process.
type ProcessInfo struct {
	PID        int
	SessionID  string
	RSS        int64
	CPUPercent float64
}
