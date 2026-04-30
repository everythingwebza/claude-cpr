package data

import "testing"

func TestNormalizeTitle(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"already clean single line", "Refactor the parser", "Refactor the parser"},
		{
			"pasted-text marker followed by real prompt",
			"[Pasted text #1 +8 lines]\n\nsudo works without pass so u can go",
			"sudo works without pass so u can go",
		},
		{
			"multiple pasted markers",
			"[Pasted text #2 +3 lines] and [Pasted text #4 +12 lines] now do X",
			"and now do X",
		},
		{
			"newlines inside prompt — first non-empty line wins",
			"first line\nsecond line\nthird line",
			"first line",
		},
		{
			"leading whitespace and tabs",
			"   \t  hello world\n",
			"hello world",
		},
		{
			"internal multi-space and tab collapse",
			"hello   world\twith\t\ttabs",
			"hello world with tabs",
		},
		{
			"only whitespace and pasted markers",
			"[Pasted text #1 +5 lines]\n\n   \n",
			"",
		},
		{
			"pasted marker mid-line gets removed cleanly",
			"please [Pasted text #1 +8 lines] refactor X",
			"please refactor X",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeTitle(c.in)
			if got != c.want {
				t.Errorf("normalizeTitle(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
