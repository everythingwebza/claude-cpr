package data

import "regexp"

// validSessionID matches the UUID-ish identifiers Claude Code emits and
// rejects anything outside that shape. Used to guard against path-traversal
// (e.g. "../etc/whatever") and option-injection (e.g. "-rf") in any code
// path that joins a session ID into a filesystem path or argv slot.
var validSessionID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

// IsValidSessionID reports whether s is safe to use as a path or argv
// component. Empty strings, traversal sequences, leading dashes, and
// over-long inputs are rejected.
func IsValidSessionID(s string) bool {
	return validSessionID.MatchString(s)
}

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
	FirstTS      int64
	LastTS       int64
	MsgCount     int
	LastPrompt   string
	lastUsefulTS int64 // timestamp of the useful prompt currently stored in LastPrompt
}
