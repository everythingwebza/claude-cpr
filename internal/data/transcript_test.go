package data

import (
	"os"
	"path/filepath"
	"strings"
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
		{"user", "please refactor X"},       // list-of-content joined
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

func TestSanitizeSyntheticTags(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			"drops system-reminder block",
			"do the thing\n<system-reminder>\nYou have superpowers blah blah\n</system-reminder>",
			"do the thing",
		},
		{
			"unwraps slash command",
			"<command-message>cpr is running…</command-message>\n<command-name>/cpr</command-name>\n<command-args>review this</command-args>",
			"/cpr\nreview this",
		},
		{
			"drops local-command-stdout",
			"ran it\n<local-command-stdout>tons of output\nmore output</local-command-stdout>",
			"ran it",
		},
		{
			"unwraps bash input, drops bash output",
			"<bash-input>ls -la</bash-input>\n<bash-stdout>file1\nfile2</bash-stdout>",
			"$ ls -la",
		},
		{
			"drops task-notification",
			"<task-notification>\n<status>completed</status>\n</task-notification>\nthanks",
			"thanks",
		},
		{
			// The allowlist must NOT touch legitimate angle-bracket content.
			"preserves code/SQL/HTML content",
			"fix this query: SELECT <customerId> FROM t WHERE x < 5 and <div class=\"a\">",
			"fix this query: SELECT <customerId> FROM t WHERE x < 5 and <div class=\"a\">",
		},
		{
			"plain prose unchanged",
			"please also just give us a blue background white text",
			"please also just give us a blue background white text",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeSyntheticTags(c.in); got != c.want {
				t.Errorf("sanitizeSyntheticTags(%q):\n got  %q\n want %q", c.in, got, c.want)
			}
		})
	}
}

// TestExtractCustomTitle_TailScan verifies the tail-read still finds an
// appended custom-title even when the marker sits far past the scan window's
// would-be start in a large file, and that the partial-first-line skip doesn't
// corrupt parsing.
func TestExtractCustomTitle_TailScan(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "big.jsonl")

	var b strings.Builder
	// ~400KB of filler messages (well past customTitleTailBytes) ...
	filler := `{"type":"user","uuid":"u","message":{"role":"user","content":"` + strings.Repeat("x", 200) + `"}}` + "\n"
	for b.Len() < 400<<10 {
		b.WriteString(filler)
	}
	// ... then the appended rename marker as the final line.
	b.WriteString(`{"type":"custom-title","customTitle":"My Renamed Session"}` + "\n")
	if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := extractCustomTitle(file)
	if err != nil {
		t.Fatalf("extractCustomTitle: %v", err)
	}
	if got != "My Renamed Session" {
		t.Errorf("got %q, want %q", got, "My Renamed Session")
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
