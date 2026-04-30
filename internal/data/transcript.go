package data

import (
	"bufio"
	"encoding/json"
	"os"
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
		return strings.TrimSpace(s)
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
	return strings.TrimSpace(b.String())
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

// extractCustomTitle returns the LAST custom-title value (rename via
// WriteCustomTitle appends, so the latest line is the live title), or ""
// if none.
func extractCustomTitle(sessionFile string) (string, error) {
	f, err := os.Open(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	last := ""
	for sc.Scan() {
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
