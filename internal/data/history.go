package data

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
)

// parseHistory reads history.jsonl and returns map[project][sessionID]*historyAgg.
// Errors on individual lines are skipped; an unreadable file returns an error.
func parseHistory(path string) (map[string]map[string]*historyAgg, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]map[string]*historyAgg{}, nil
		}
		return nil, err
	}
	defer f.Close()

	out := map[string]map[string]*historyAgg{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24) // up to 16MB lines

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var d struct {
			SessionID string `json:"sessionId"`
			Project   string `json:"project"`
			Timestamp int64  `json:"timestamp"`
			Display   string `json:"display"`
		}
		if err := json.Unmarshal([]byte(line), &d); err != nil {
			continue
		}
		if d.SessionID == "" || d.Project == "" {
			continue
		}
		proj := out[d.Project]
		if proj == nil {
			proj = map[string]*historyAgg{}
			out[d.Project] = proj
		}
		agg := proj[d.SessionID]
		if agg == nil {
			agg = &historyAgg{FirstTS: 1 << 62}
			proj[d.SessionID] = agg
		}
		agg.MsgCount++
		if d.Timestamp < agg.FirstTS {
			agg.FirstTS = d.Timestamp
		}
		if d.Timestamp > agg.LastTS {
			agg.LastTS = d.Timestamp
		}
		if isUsefulPrompt(d.Display) {
			agg.LastPrompt = strings.TrimSpace(d.Display)
		}
	}
	if err := sc.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	return out, nil
}

// isUsefulPrompt mirrors the Python noise filter.
func isUsefulPrompt(p string) bool {
	p = strings.TrimSpace(p)
	if len(p) <= 5 {
		return false
	}
	lower := strings.ToLower(p)
	noise := []string{"/", "[request interrupted", "exit", "quit", "yes", "no", "ok"}
	for _, n := range noise {
		if strings.HasPrefix(lower, n) {
			return false
		}
	}
	return true
}
