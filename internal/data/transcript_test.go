package data

import (
	"testing"
)

func TestExtractMessages_DedupAndOrder(t *testing.T) {
	msgs, err := extractMessages("testdata/projects/-home-u-proj1/sess-a.jsonl", 100)
	if err != nil {
		t.Fatalf("extractMessages error: %v", err)
	}

	want := []struct{ role, text string }{
		{"user", "hello there"},
		{"assistant", "hi! how can I help"}, // streaming dedup keeps last
		{"user", "please refactor X"},        // list-of-content joined
		{"assistant", "sure"},
	}
	if len(msgs) != len(want) {
		t.Fatalf("len: got %d, want %d. msgs: %+v", len(msgs), len(want), msgs)
	}
	for i, w := range want {
		if msgs[i].Role != w.role || msgs[i].Text != w.text {
			t.Errorf("[%d]: got %s/%q, want %s/%q", i, msgs[i].Role, msgs[i].Text, w.role, w.text)
		}
	}
}

func TestExtractCustomTitle(t *testing.T) {
	title, err := extractCustomTitle("testdata/projects/-home-u-proj1/sess-a.jsonl")
	if err != nil {
		t.Fatalf("extractCustomTitle error: %v", err)
	}
	if title != "Refactor the parser" {
		t.Errorf("got %q, want %q", title, "Refactor the parser")
	}
}
