package data

import "testing"

func TestIsValidSessionID(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		// Valid: real Claude Code session IDs are UUID-style
		{"fe7ac19f-8934-4f0b-83ec-c781e7179008", true},
		{"abc", true},
		{"X", true},
		{"a_b-c", true},

		// Empty / too long
		{"", false},
		{string(make([]byte, 200)), false},

		// Path-traversal candidates
		{"../etc/passwd", false},
		{"a/b", false},
		{"foo.jsonl", false}, // dot not allowed
		{"a b", false},

		// Option-injection candidates (leading dash)
		{"-rf", false},
		{"--config=/tmp/x", false},
		{"-h", false},
	}
	for _, c := range cases {
		got := IsValidSessionID(c.in)
		if got != c.want {
			t.Errorf("IsValidSessionID(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
