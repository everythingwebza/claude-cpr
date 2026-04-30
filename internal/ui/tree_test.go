package ui

import (
	"testing"

	"github.com/everythingwebza/claude-cpr/internal/data"
)

func TestTreeModel_FlattenWithExpansion(t *testing.T) {
	sessions := []data.SessionInfo{
		{Project: "/p1", SessionID: "s1a", Title: "A", Modified: "2026-04-29T10:00:00Z"},
		{Project: "/p1", SessionID: "s1b", Title: "B", Modified: "2026-04-29T09:00:00Z"},
		{Project: "/p2", SessionID: "s2a", Title: "C", Modified: "2026-04-28T10:00:00Z"},
	}
	tm := NewTreeModel(sessions, map[string]bool{"/p1": true, "/p2": false}, nil, "recent")

	rows := tm.flatten("")
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4 (p1 header + 2 sessions + p2 header)", len(rows))
	}
	if rows[0].Kind != RowProject || rows[0].Project != "/p1" {
		t.Errorf("row 0: got %+v", rows[0])
	}
	if rows[1].Kind != RowSession || rows[1].Session.SessionID != "s1a" {
		t.Errorf("row 1: got %+v", rows[1])
	}
	if rows[3].Kind != RowProject || rows[3].Project != "/p2" {
		t.Errorf("row 3: got %+v", rows[3])
	}
}

func TestTreeModel_FlattenWithFilter(t *testing.T) {
	sessions := []data.SessionInfo{
		{Project: "/p1", SessionID: "s1a", Title: "Alpha refactor", Modified: "2026-04-29T10:00:00Z"},
		{Project: "/p1", SessionID: "s1b", Title: "Beta tests", Modified: "2026-04-29T09:00:00Z"},
		{Project: "/p2", SessionID: "s2a", Title: "Refactor again", Modified: "2026-04-28T10:00:00Z"},
	}
	tm := NewTreeModel(sessions, map[string]bool{"/p1": true, "/p2": true}, nil, "recent")
	rows := tm.flatten("refactor")
	titles := []string{}
	for _, r := range rows {
		if r.Kind == RowSession {
			titles = append(titles, r.Session.Title)
		}
	}
	want := []string{"Alpha refactor", "Refactor again"}
	if len(titles) != len(want) {
		t.Fatalf("got %v, want %v", titles, want)
	}
	for i, w := range want {
		if titles[i] != w {
			t.Errorf("[%d]: got %q want %q", i, titles[i], w)
		}
	}
}
