package data

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
)

// Message is one user or assistant turn in a session transcript.
type Message struct {
	Role string // "user" or "assistant"
	Text string
}

// extractMessages reads up to maxMessages messages, deduping streaming chunks
// by message ID (assistant) or uuid (user). Order is preserved by first-seen.
func extractMessages(sessionFile string, maxMessages int) ([]Message, error) {
	f, err := os.Open(sessionFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type slot struct {
		role string
		id   string
		text string
	}
	// User UUIDs and assistant message IDs occupy distinct namespaces in the
	// JSONL, so a single map keyed by either is collision-safe in practice.
	var order []string
	seen := map[string]*slot{}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var d struct {
			Type    string `json:"type"`
			UUID    string `json:"uuid"`
			Message struct {
				Role    string          `json:"role"`
				ID      string          `json:"id"`
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &d); err != nil {
			continue
		}

		switch {
		case d.Type == "user" && d.Message.Role == "user":
			if d.UUID == "" {
				continue
			}
			text := contentToText(d.Message.Content)
			if text == "" {
				continue
			}
			// User messages are not streamed; a duplicate UUID indicates a
			// re-appended entry — first occurrence is canonical.
			if _, ok := seen[d.UUID]; !ok {
				seen[d.UUID] = &slot{role: "user", id: d.UUID, text: text}
				order = append(order, d.UUID)
			}

		case d.Message.Role == "assistant":
			if d.Message.ID == "" {
				continue
			}
			text := contentToText(d.Message.Content)
			if text == "" {
				continue
			}
			if _, ok := seen[d.Message.ID]; !ok {
				seen[d.Message.ID] = &slot{role: "assistant", id: d.Message.ID, text: text}
				order = append(order, d.Message.ID)
			} else {
				// streaming chunk: keep the last (longest) version
				seen[d.Message.ID].text = text
			}
		}
	}

	out := make([]Message, 0, len(order))
	for _, id := range order {
		s := seen[id]
		out = append(out, Message{Role: s.role, Text: s.text})
		if len(out) >= maxMessages {
			break
		}
	}
	return out, nil
}

// contentToText handles both string content and []{type:text,text:...} content.
func contentToText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return sanitizeSyntheticTags(s)
	}
	// fall back to array of {type, text}
	var arr []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &arr); err != nil {
		return ""
	}
	// Claude text-content blocks are whole tokens; fragments already contain
	// any inter-word whitespace. A space separator would double-space runs
	// that end with trailing whitespace, so none is added here.
	var b strings.Builder
	for _, c := range arr {
		if c.Type != "text" {
			continue
		}
		b.WriteString(c.Text)
	}
	return sanitizeSyntheticTags(b.String())
}

// syntheticBlockTags are the wrapper tags Claude Code injects into message
// content — hook output, slash-command plumbing, system reminders,
// background-task notifications. The entire block (open tag, body, close tag)
// is pure noise for someone skimming a transcript preview.
var syntheticBlockTags = []string{
	"system-reminder",
	"command-message",
	"local-command-stdout",
	"local-command-caveat",
	"task-notification",
	"bash-stdout",
	"bash-stderr",
}

var (
	// syntheticBlocks removes each wrapper block whole. Go's RE2 has no
	// back-references, so the pattern is an explicit alternation of
	// <tag>…</tag> pairs (one per syntheticBlockTags entry) rather than a
	// captured-tag back-reference. (?is) = case-insensitive + dot-matches-newline.
	syntheticBlocks = regexp.MustCompile(buildSyntheticBlockPattern())

	// commandName/commandArgs/bashInput wrap plumbing whose INNER text IS
	// meaningful — a reader wants to know which command ran — so they are
	// unwrapped to a readable form rather than dropped. command-name already
	// carries its leading slash (e.g. "/cpr"); bash-input is prefixed with a
	// shell-style "$ ".
	commandNameTag = regexp.MustCompile(`(?is)<command-name>\s*(.*?)\s*</command-name>`)
	commandArgsTag = regexp.MustCompile(`(?is)<command-args>\s*(.*?)\s*</command-args>`)
	bashInputTag   = regexp.MustCompile(`(?is)<bash-input>\s*(.*?)\s*</bash-input>`)

	// blankLineRun collapses the runs of blank lines left behind by block
	// removal down to a single blank-line separator.
	blankLineRun = regexp.MustCompile(`\n[ \t]*\n[ \t\n]*`)
)

// buildSyntheticBlockPattern assembles the alternation of <tag>…</tag> block
// patterns from syntheticBlockTags, with a trailing optional newline so block
// removal doesn't leave a dangling blank line.
func buildSyntheticBlockPattern() string {
	alts := make([]string, len(syntheticBlockTags))
	for i, t := range syntheticBlockTags {
		alts[i] = "<" + t + ">.*?</" + t + ">"
	}
	return `(?is)(?:` + strings.Join(alts, "|") + `)\n?`
}

// sanitizeSyntheticTags strips the synthetic wrapper tags Claude Code injects
// into message content so the preview shows human-meaningful prose. It uses a
// strict allowlist: only known plumbing tags are touched. Legitimate
// angle-bracket content in user messages — code, SQL, HTML/XML such as <div>,
// <select>, <customerId> — is left exactly as written.
func sanitizeSyntheticTags(s string) string {
	s = syntheticBlocks.ReplaceAllString(s, "")
	s = commandArgsTag.ReplaceAllString(s, "$1")
	s = commandNameTag.ReplaceAllString(s, "$1")
	s = bashInputTag.ReplaceAllString(s, "$ $1")
	s = blankLineRun.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// WriteCustomTitle appends a {"type":"custom-title", ...} line to the session
// JSONL. Same mechanism Claude itself uses; first occurrence wins on read.
func WriteCustomTitle(sessionFile, title string) error {
	f, err := os.OpenFile(sessionFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(map[string]string{
		"type":        "custom-title",
		"customTitle": title,
	})
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

// customTitleTailBytes bounds how far back from EOF extractCustomTitle scans
// for the rename marker. Session JSONLs reach tens of MB; reading every one in
// full on each cold start cost ~5s across a few hundred sessions. The marker
// is appended (WriteCustomTitle / Claude's own rename), so it lives at the
// tail — scanning the last 256KB finds it cheaply. The only loss: if a session
// is renamed and then *heavily* continued (256KB ≈ hundreds of messages of new
// content appended after the marker), the title reverts to the auto-generated
// one. Acceptable degradation for the startup-latency win.
const customTitleTailBytes = 256 << 10

// extractCustomTitle returns the LAST custom-title value (rename appends, so
// the latest line is the live title), or "" if none. It scans only the tail of
// the file (see customTitleTailBytes).
func extractCustomTitle(sessionFile string) (string, error) {
	f, err := os.Open(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	var start int64
	if info.Size() > customTitleTailBytes {
		start = info.Size() - customTitleTailBytes
		if _, err := f.Seek(start, io.SeekStart); err != nil {
			return "", err
		}
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	skipPartial := start > 0 // a mid-file seek lands inside a line; drop the first fragment
	last := ""
	for sc.Scan() {
		if skipPartial {
			skipPartial = false
			continue
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var d struct {
			Type        string `json:"type"`
			CustomTitle string `json:"customTitle"`
		}
		if err := json.Unmarshal([]byte(line), &d); err != nil {
			continue
		}
		if d.Type == "custom-title" {
			last = d.CustomTitle
		}
	}
	return last, nil
}
