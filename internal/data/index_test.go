package data

import (
	"testing"
)

func TestLoadIndex_FixtureProducesEntries(t *testing.T) {
	indexed, err := parseAllIndices("testdata/projects")
	if err != nil {
		t.Fatalf("parseAllIndices error: %v", err)
	}
	e, ok := indexed[indexKey{Project: "/home/u/proj1", SessionID: "sess-a"}]
	if !ok {
		t.Fatalf("missing sess-a entry")
	}
	if e.Summary != "Refactor the parser" {
		t.Errorf("Summary: got %q, want %q", e.Summary, "Refactor the parser")
	}
	if e.MessageCount != 12 {
		t.Errorf("MessageCount: got %d, want 12", e.MessageCount)
	}
	if e.GitBranch != "main" {
		t.Errorf("GitBranch: got %q, want %q", e.GitBranch, "main")
	}
}
