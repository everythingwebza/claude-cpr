package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Cursor struct {
	Project   string `json:"project"`
	SessionID string `json:"sessionId"`
}

type State struct {
	Version    int             `json:"version"`
	LastCursor Cursor          `json:"lastCursor"`
	Expanded   map[string]bool `json:"expanded"`
	Pinned     []string        `json:"pinned"`
	SortMode   string          `json:"sortMode"`
}

func Default() State {
	return State{
		Version:  1,
		Expanded: map[string]bool{},
		Pinned:   []string{},
		SortMode: "recent",
	}
}

// Load reads the state file. Missing or corrupt → defaults, no error.
func Load(path string) (State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Default(), nil // other read errors → defaults silently
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return Default(), nil
	}
	if s.Version == 0 {
		s.Version = 1
	}
	if s.Expanded == nil {
		s.Expanded = map[string]bool{}
	}
	if s.Pinned == nil {
		s.Pinned = []string{}
	}
	if s.SortMode == "" {
		s.SortMode = "recent"
	}
	return s, nil
}

// Save writes the state file atomically (write to .tmp, rename).
func Save(path string, s State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
