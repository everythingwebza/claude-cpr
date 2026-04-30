package data

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// SessionStore caches the merged session list and the active-process set.
// Refresh() re-reads source files only when their mtimes have changed.
type SessionStore struct {
	historyPath  string
	projectsRoot string

	mu           sync.Mutex
	sessions     []SessionInfo
	historyAggs  map[string]map[string]*historyAgg
	indexEntries map[indexKey]indexEntry
	historyMtime time.Time
	customTitle  map[string]string // sessionFile path → title cache
	customMtimes map[string]time.Time
	activeDirs   map[string]struct{}

	transcripts *lru.Cache[string, []Message] // key: sessionFilePath
}

func NewSessionStore(historyPath, projectsRoot string) (*SessionStore, error) {
	cache, err := lru.New[string, []Message](16)
	if err != nil {
		return nil, err
	}
	return &SessionStore{
		historyPath:  historyPath,
		projectsRoot: projectsRoot,
		customTitle:  map[string]string{},
		customMtimes: map[string]time.Time{},
		transcripts:  cache,
	}, nil
}

// Build returns the merged session list, refreshing caches if any source mtime changed.
//
// The lock is held across filesystem I/O (parseHistory / parseAllIndices /
// getActiveProjectDirs) for simplicity. This is acceptable for a single-UI-
// goroutine model — Build is called from the Bubble Tea event loop, never in
// hot paths. If a future task adds a background refresh ticker, swap in a
// snapshot/swap pattern so other readers don't block on the rebuild.
func (s *SessionStore) Build() ([]SessionInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// history.jsonl
	if hi, err := os.Stat(s.historyPath); err == nil {
		if !hi.ModTime().Equal(s.historyMtime) {
			aggs, err := parseHistory(s.historyPath)
			if err != nil {
				return nil, err
			}
			s.historyAggs = aggs
			s.historyMtime = hi.ModTime()
			s.sessions = nil // force rebuild
		}
	} else if s.historyAggs == nil {
		s.historyAggs = map[string]map[string]*historyAgg{}
	}

	// sessions-index.json — re-walk; parseAllIndices is cheap enough
	indexed, err := parseAllIndices(s.projectsRoot)
	if err != nil {
		return nil, err
	}
	s.indexEntries = indexed

	s.sessions = s.merge()
	s.activeDirs = getActiveProjectDirs()
	return s.sessions, nil
}

// ActiveDirs returns a copy of the cached active-project set from the most
// recent Build. The copy isolates callers from concurrent rebuilds and
// prevents accidental external mutation of internal state.
func (s *SessionStore) ActiveDirs() map[string]struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]struct{}, len(s.activeDirs))
	for k := range s.activeDirs {
		out[k] = struct{}{}
	}
	return out
}

// Transcript loads (and caches) the messages for a session.
//
// Note: the cached value is whatever the FIRST caller's `max` produced — a
// later call with a larger `max` returns the same truncated slice. In
// practice all callers pass the same value (the PreviewModel's preview cap),
// so this is fine. If a caller needs a different cap, evict via the LRU first.
func (s *SessionStore) Transcript(project, sessionID string, max int) ([]Message, error) {
	file := s.sessionFile(project, sessionID)
	if v, ok := s.transcripts.Get(file); ok {
		return v, nil
	}
	msgs, err := extractMessages(file, max)
	if err != nil {
		return nil, err
	}
	s.transcripts.Add(file, msgs)
	return msgs, nil
}

// InvalidateCustomTitle drops any cached custom-title for the given session
// file path. Called by the rename flow (Task 16) so the next Build re-reads
// the title from the JSONL.
func (s *SessionStore) InvalidateCustomTitle(file string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.customTitle, file)
	delete(s.customMtimes, file)
}

// merge fuses history + index + custom-title into the canonical session list,
// sorted by Modified desc. Caller holds s.mu.
func (s *SessionStore) merge() []SessionInfo {
	out := []SessionInfo{}
	seen := map[indexKey]bool{}

	for proj, sm := range s.historyAggs {
		for sid, agg := range sm {
			if agg.LastTS == 0 {
				continue
			}
			file := s.sessionFile(proj, sid)
			modified := s.fileMtimeRFC3339(file)
			if modified == "" {
				modified = msToRFC3339(agg.LastTS)
			}
			entry := s.indexEntries[indexKey{proj, sid}]
			title := normalizeTitle(s.cachedCustomTitle(file))
			if title == "" {
				title = normalizeTitle(entry.Summary)
			}
			if title == "" {
				title = normalizeTitle(agg.LastPrompt)
			}
			if title == "" {
				title = "(untitled)"
			}
			msgs := agg.MsgCount
			if entry.MessageCount > msgs {
				msgs = entry.MessageCount
			}
			out = append(out, SessionInfo{
				Project: proj, SessionID: sid, Title: title,
				Modified: modified, MsgCount: msgs, Branch: entry.GitBranch,
			})
			seen[indexKey{proj, sid}] = true
		}
	}
	// index-only sessions
	for key, e := range s.indexEntries {
		if seen[key] {
			continue
		}
		if e.Modified == "" {
			continue
		}
		title := normalizeTitle(e.Summary)
		if title == "" {
			title = normalizeTitle(e.FirstPrompt)
		}
		if title == "" {
			title = "(untitled)"
		}
		out = append(out, SessionInfo{
			Project: key.Project, SessionID: key.SessionID, Title: title,
			Modified: e.Modified, MsgCount: e.MessageCount, Branch: e.GitBranch,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Modified > out[j].Modified
	})
	return out
}

func (s *SessionStore) sessionFile(project, sessionID string) string {
	dir := strings.ReplaceAll(project, "/", "-")
	return filepath.Join(s.projectsRoot, dir, sessionID+".jsonl")
}

func (s *SessionStore) fileMtimeRFC3339(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return info.ModTime().UTC().Format(time.RFC3339)
}

func (s *SessionStore) cachedCustomTitle(file string) string {
	info, err := os.Stat(file)
	if err != nil {
		return ""
	}
	if cached, ok := s.customTitle[file]; ok && s.customMtimes[file].Equal(info.ModTime()) {
		return cached
	}
	title, _ := extractCustomTitle(file)
	s.customTitle[file] = title
	s.customMtimes[file] = info.ModTime()
	return title
}

func msToRFC3339(ms int64) string {
	return time.Unix(0, ms*int64(time.Millisecond)).UTC().Format(time.RFC3339)
}
