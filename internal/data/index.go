package data

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
)

type indexKey struct {
	Project   string
	SessionID string
}

type indexEntry struct {
	Summary      string
	MessageCount int
	GitBranch    string
	Modified     string
	FirstPrompt  string
}

// parseAllIndices walks rootDir for any sessions-index.json files and merges them.
// Missing rootDir returns an empty map without error.
func parseAllIndices(rootDir string) (map[indexKey]indexEntry, error) {
	out := map[indexKey]indexEntry{}
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		return out, nil
	}
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable subtrees
		}
		if d.IsDir() || d.Name() != "sessions-index.json" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var doc struct {
			OriginalPath string `json:"originalPath"`
			Entries      []struct {
				SessionID    string `json:"sessionId"`
				Summary      string `json:"summary"`
				MessageCount int    `json:"messageCount"`
				GitBranch    string `json:"gitBranch"`
				Modified     string `json:"modified"`
				FirstPrompt  string `json:"firstPrompt"`
			} `json:"entries"`
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			return nil
		}
		if doc.OriginalPath == "" {
			return nil
		}
		for _, e := range doc.Entries {
			if !IsValidSessionID(e.SessionID) {
				continue
			}
			out[indexKey{Project: doc.OriginalPath, SessionID: e.SessionID}] = indexEntry{
				Summary:      e.Summary,
				MessageCount: e.MessageCount,
				GitBranch:    e.GitBranch,
				Modified:     e.Modified,
				FirstPrompt:  e.FirstPrompt,
			}
		}
		return nil
	})
	return out, err
}
