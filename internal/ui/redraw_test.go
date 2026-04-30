package ui

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/everythingwebza/claude-cpr/internal/data"
)

// makeModelForRegression builds a Model with a synthetic 4-project × 3-session
// session list, all projects expanded, in a known cursor position. Used to
// drive the redraw test without needing a real SessionStore on disk.
func makeModelForRegression(t *testing.T) Model {
	t.Helper()
	sessions := []data.SessionInfo{}
	for p := 0; p < 4; p++ {
		for s := 0; s < 3; s++ {
			sessions = append(sessions, data.SessionInfo{
				Project:   fmt.Sprintf("/proj/%d", p),
				SessionID: fmt.Sprintf("p%d-s%d", p, s),
				Title:     fmt.Sprintf("Session %d-%d", p, s),
				Modified:  fmt.Sprintf("2026-04-2%d", 9-p),
				MsgCount:  s,
			})
		}
	}
	expanded := map[string]bool{
		"/proj/0": true, "/proj/1": true, "/proj/2": true, "/proj/3": true,
	}
	tree := NewTreeModel(sessions, expanded, nil, SortRecent)
	return Model{
		keys:    DefaultKeyMap(),
		tree:    tree,
		search:  NewSearchModel(),
		preview: NewPreviewModel(),
		overlay: NewOverlay(),
		focus:   FocusTree,
	}
}

// TestRedraw_NoDuplicateProjectHeaderInAnyFrame asserts that a project header
// path never appears twice in the same frame. The Python original suffered
// from a hand-rolled clear-and-redraw scheme where the project label could
// be left as residue when the cursor scrolled across project boundaries on
// a too-small terminal. The Bubble Tea / Lipgloss model renders into bounded
// pane regions and cannot produce that artifact — this test guards against
// regressions if the layout/render code ever drifts.
func TestRedraw_NoDuplicateProjectHeaderInAnyFrame(t *testing.T) {
	m := makeModelForRegression(t)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(60, 24))

	// Walk the cursor down past project boundaries.
	for i := 0; i < 8; i++ {
		tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc}) // triggers Quit
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	raw, _ := io.ReadAll(tm.FinalOutput(t))
	out := string(raw)

	// Each rendered frame is separated by an alt-screen full-clear marker
	// (\x1b[2J). For any frame, the SAME project header should appear at
	// most once — duplication would mean a stale header line was left as
	// residue from a previous render.
	frames := strings.Split(out, "\x1b[2J")
	for i, frame := range frames {
		for p := 0; p < 4; p++ {
			needle := fmt.Sprintf("/proj/%d", p)
			if c := strings.Count(frame, needle); c > 1 {
				t.Errorf("frame %d contains %q %d times (regression)", i, needle, c)
			}
		}
	}
}
