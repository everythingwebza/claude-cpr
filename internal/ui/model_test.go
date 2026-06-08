package ui

import (
	"os/exec"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/everythingwebza/claude-cpr/internal/data"
)

// newTestModel builds a Model with no store and an empty statePath (so
// saveState no-ops) — enough to exercise the resume/new-session state machine.
func newTestModel() Model {
	return Model{
		tree:    NewTreeModel(nil, nil, nil, SortRecent),
		search:  NewSearchModel(),
		preview: NewPreviewModel(),
		keys:    DefaultKeyMap(),
		focus:   FocusTree,
	}
}

// quitsWith reports whether invoking cmd yields a tea.QuitMsg.
func quitsWith(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestExecResume_SetsExecRequestAndQuits(t *testing.T) {
	bin, err := exec.LookPath("claude")
	if err != nil {
		t.Skip("claude not on PATH; skipping exec-request shape test")
	}
	m := newTestModel()
	sess := data.SessionInfo{Project: "/home/u/proj", SessionID: "abc-123"}

	next, cmd := m.execResume(sess, true) // confirmed=true avoids the store/active check
	nm := next.(Model)

	if !quitsWith(cmd) {
		t.Errorf("execResume should return a tea.Quit cmd")
	}
	if !nm.quitting {
		t.Errorf("execResume should set quitting=true so the final render blanks")
	}
	req := nm.ExecRequest()
	if req == nil {
		t.Fatal("ExecRequest() is nil; expected a resume target")
	}
	if req.Bin != bin {
		t.Errorf("Bin = %q, want absolute path %q", req.Bin, bin)
	}
	if req.Dir != "/home/u/proj" {
		t.Errorf("Dir = %q, want /home/u/proj", req.Dir)
	}
	wantArgs := []string{bin, "--resume", "abc-123"}
	if !reflect.DeepEqual(req.Args, wantArgs) {
		t.Errorf("Args = %v, want %v", req.Args, wantArgs)
	}
}

func TestExecNewSession_SetsExecRequestWithoutResume(t *testing.T) {
	bin, err := exec.LookPath("claude")
	if err != nil {
		t.Skip("claude not on PATH; skipping exec-request shape test")
	}
	m := newTestModel()

	next, cmd := m.execNewSessionForced("/home/u/proj")
	nm := next.(Model)

	if !quitsWith(cmd) {
		t.Errorf("execNewSessionForced should return a tea.Quit cmd")
	}
	req := nm.ExecRequest()
	if req == nil {
		t.Fatal("ExecRequest() is nil; expected a new-session target")
	}
	wantArgs := []string{bin} // no --resume for a fresh session
	if !reflect.DeepEqual(req.Args, wantArgs) {
		t.Errorf("Args = %v, want %v", req.Args, wantArgs)
	}
}

func TestModelUpdate_ArrowDownMovesTreeCursor(t *testing.T) {
	sessions := []data.SessionInfo{
		{Project: "/p1", SessionID: "s1", Title: "A", Modified: "2026-04-29T10:00:00Z"},
		{Project: "/p2", SessionID: "s2", Title: "B", Modified: "2026-04-29T09:00:00Z"},
	}
	tree := NewTreeModel(sessions, map[string]bool{"/p1": true, "/p2": true}, nil, SortRecent)
	m := Model{
		tree:    tree,
		search:  NewSearchModel(),
		preview: NewPreviewModel(),
		keys:    DefaultKeyMap(),
		focus:   FocusTree,
		width:   100, height: 30,
	}

	msg := tea.KeyMsg{Type: tea.KeyDown}
	next, _ := m.Update(msg)
	nm := next.(Model)

	if nm.tree.cursor != 1 {
		t.Errorf("after Down: cursor=%d, want 1", nm.tree.cursor)
	}
}

func TestModelUpdate_OldDebounceMsgIsIgnored(t *testing.T) {
	// A DebounceMsg whose Seq is older than the model's current debounceSeq
	// must be a no-op (otherwise rapid cursor scrolling would fire many
	// expensive transcript loads).
	m := Model{
		tree:        NewTreeModel(nil, nil, nil, SortRecent),
		search:      NewSearchModel(),
		preview:     NewPreviewModel(),
		keys:        DefaultKeyMap(),
		focus:       FocusTree,
		debounceSeq: 5,
		pendingLoad: "/p|s",
	}
	next, cmd := m.Update(DebounceMsg{Seq: 3}) // older
	if cmd != nil {
		t.Errorf("stale DebounceMsg should not produce a cmd, got %T", cmd)
	}
	if next.(Model).pendingLoad != "/p|s" {
		t.Errorf("pendingLoad mutated unexpectedly")
	}
}

func TestModelView_FitsExactlyInTerminalHeight(t *testing.T) {
	// Drive a WindowSizeMsg through Update, then assert View() output is no
	// taller than the reported terminal height. Lipgloss's alt-screen will
	// scroll any overflow off the top, hiding the search bar.
	sessions := []data.SessionInfo{}
	for i := 0; i < 50; i++ {
		sessions = append(sessions, data.SessionInfo{
			Project:   "/p" + string(rune('a'+i%26)) + string(rune('a'+i/26)),
			SessionID: "s",
			Title:     "session " + string(rune('a'+i%26)),
			Modified:  "2026-04-29T10:00:00Z",
		})
	}
	tree := NewTreeModel(sessions, map[string]bool{"/paa": true, "/pba": true}, nil, SortRecent)
	m := Model{
		tree:    tree,
		search:  NewSearchModel(),
		preview: NewPreviewModel(),
		keys:    DefaultKeyMap(),
		focus:   FocusTree,
	}
	const W, H = 100, 30
	next, _ := m.Update(tea.WindowSizeMsg{Width: W, Height: H})
	nm := next.(Model)

	out := nm.View()
	lines := 1
	for _, c := range out {
		if c == '\n' {
			lines++
		}
	}
	if lines > H {
		t.Errorf("View at %dx%d emitted %d lines, want ≤ %d", W, H, lines, H)
	}
}

func TestModelUpdate_LowercaseKDoesNotJumpToSearch(t *testing.T) {
	// Vim-style nav 'k' (Up) should reach the tree, not the search bar.
	sessions := []data.SessionInfo{
		{Project: "/p1", SessionID: "s1", Title: "A", Modified: "2026-04-29T10:00:00Z"},
		{Project: "/p2", SessionID: "s2", Title: "B", Modified: "2026-04-29T09:00:00Z"},
	}
	tree := NewTreeModel(sessions, map[string]bool{"/p1": true, "/p2": true}, nil, SortRecent)
	tree.cursor = 1 // start on second row so Up has somewhere to go

	m := Model{
		tree:    tree,
		search:  NewSearchModel(),
		preview: NewPreviewModel(),
		keys:    DefaultKeyMap(),
		focus:   FocusTree,
		width:   100, height: 30,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	next, _ := m.Update(msg)
	nm := next.(Model)

	if nm.focus != FocusTree {
		t.Errorf("after 'k': focus=%v, want FocusTree (vim-style nav should stay in tree)", nm.focus)
	}
	if nm.tree.cursor != 0 {
		t.Errorf("after 'k': cursor=%d, want 0 (Up should decrement)", nm.tree.cursor)
	}
}
