package data

// SessionInfo aggregates a session's metadata from all sources.
type SessionInfo struct {
	Project   string
	SessionID string
	Title     string
	Modified  string // RFC3339 / ISO 8601 string for stable lex sort
	MsgCount  int
	Branch    string
}

// historyAgg is the per-session aggregate built from history.jsonl.
type historyAgg struct {
	FirstTS    int64
	LastTS     int64
	MsgCount   int
	LastPrompt string
}
