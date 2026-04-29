package data

import (
	"testing"
)

func TestParseHistory_FixtureProducesExpectedAggregates(t *testing.T) {
	aggs, err := parseHistory("testdata/history.jsonl")
	if err != nil {
		t.Fatalf("parseHistory error: %v", err)
	}

	p1 := aggs["/home/u/proj1"]
	if p1 == nil {
		t.Fatalf("missing aggregates for /home/u/proj1")
	}
	a := p1["sess-a"]
	if a == nil {
		t.Fatalf("missing sess-a")
	}
	if a.MsgCount != 4 {
		t.Errorf("MsgCount: got %d, want 4", a.MsgCount)
	}
	if a.FirstTS != 1700000000000 {
		t.Errorf("FirstTS: got %d, want 1700000000000", a.FirstTS)
	}
	if a.LastTS != 1700000600000 {
		t.Errorf("LastTS: got %d, want 1700000600000", a.LastTS)
	}
	// last useful prompt skips short noise like "yes" and "/exit"
	if a.LastPrompt != "second prompt" {
		t.Errorf("LastPrompt: got %q, want %q", a.LastPrompt, "second prompt")
	}

	if _, ok := aggs["/home/u/proj2"]["sess-b"]; !ok {
		t.Errorf("missing sess-b")
	}
	// entries with missing sessionId or project are dropped
	for _, sess := range aggs[""] {
		t.Errorf("entries with empty project should be dropped: %+v", sess)
	}
}
