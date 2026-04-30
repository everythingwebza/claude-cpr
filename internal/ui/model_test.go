package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/everythingwebza/claude-cpr/internal/data"
)

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
