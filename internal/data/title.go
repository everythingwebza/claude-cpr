package data

import (
	"regexp"
	"strings"
)

var (
	// pastedTextMarker matches Claude Code's "[Pasted text #N +M lines]"
	// placeholder which appears inline inside user prompts when they paste
	// large text blobs. It is noise for our display purposes.
	pastedTextMarker = regexp.MustCompile(`\[Pasted text #\d+ \+\d+ lines\]`)

	// whitespaceRun collapses any sequence of whitespace into a single space.
	whitespaceRun = regexp.MustCompile(`\s+`)
)

// normalizeTitle cleans up a session title for display. It:
//   - removes "[Pasted text #N +M lines]" markers anywhere in the string
//   - takes the first non-empty line of what remains
//   - collapses internal whitespace runs to a single space
//   - trims leading/trailing whitespace
//
// Empty input returns empty output. Already-clean single-line titles pass
// through unchanged (modulo whitespace collapse, which is a no-op for them).
func normalizeTitle(s string) string {
	if s == "" {
		return ""
	}
	s = pastedTextMarker.ReplaceAllString(s, "")
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = whitespaceRun.ReplaceAllString(line, " ")
		return strings.TrimSpace(line)
	}
	return ""
}
