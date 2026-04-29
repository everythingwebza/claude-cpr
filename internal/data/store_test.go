package data

import (
	"testing"
)

func TestSessionStore_BuildMergesAllSources(t *testing.T) {
	historyPath := "testdata/history.jsonl"
	projectsRoot := "testdata/projects"

	s, err := NewSessionStore(historyPath, projectsRoot)
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	sessions, err := s.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	by := map[string]SessionInfo{}
	for _, ss := range sessions {
		by[ss.SessionID] = ss
	}

	// sess-a: in history + index + has custom-title in transcript
	a, ok := by["sess-a"]
	if !ok {
		t.Fatalf("missing sess-a")
	}
	if a.Title != "Refactor the parser" { // custom-title wins
		t.Errorf("sess-a Title: got %q, want custom-title", a.Title)
	}
	if a.Branch != "main" {
		t.Errorf("sess-a Branch: got %q, want main", a.Branch)
	}
	if a.MsgCount < 4 {
		t.Errorf("sess-a MsgCount: got %d, want >= 4", a.MsgCount)
	}

	// sess-d: index-only, no history. Should still appear.
	d, ok := by["sess-d"]
	if !ok {
		t.Fatalf("missing sess-d (index-only)")
	}
	if d.Title != "Index-only session, never in history" {
		t.Errorf("sess-d Title: got %q", d.Title)
	}

	// sessions sorted by Modified desc
	for i := 1; i < len(sessions); i++ {
		if sessions[i-1].Modified < sessions[i].Modified {
			t.Errorf("not sorted desc by Modified at %d: %s before %s",
				i, sessions[i-1].Modified, sessions[i].Modified)
		}
	}
}
