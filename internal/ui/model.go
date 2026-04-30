package ui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/everythingwebza/claude-cpr/internal/data"
)

// previewDebounce is the delay between cursor settling on a session and the
// preview pane firing its async transcript load.
const previewDebounce = 150 * time.Millisecond

type Focus int

const (
	FocusTree Focus = iota
	FocusSearch
	FocusPreview
)

type Model struct {
	store *data.SessionStore
	keys  KeyMap

	tree    TreeModel
	search  SearchModel
	preview PreviewModel
	focus   Focus

	width, height int
	quitting      bool
	err           error

	// Preview debounce state. debounceSeq is bumped on every cursor move so
	// older pending DebounceMsgs ignore themselves on arrival.
	debounceSeq int64
	pendingLoad string // "project|sessionID"
}

func NewModel(store *data.SessionStore) (Model, error) {
	sessions, err := store.Build()
	if err != nil {
		return Model{}, err
	}
	expanded := defaultExpansion(sessions, 2)
	tree := NewTreeModel(sessions, expanded, store.ActiveDirs(), SortRecent)
	return Model{
		store:   store,
		keys:    DefaultKeyMap(),
		tree:    tree,
		search:  NewSearchModel(),
		preview: NewPreviewModel(),
		focus:   FocusTree,
	}, nil
}

// defaultExpansion expands the top-N most-recent projects (until per-project
// state persists in Task 14).
func defaultExpansion(sessions []data.SessionInfo, n int) map[string]bool {
	out := map[string]bool{}
	seen := []string{}
	for _, s := range sessions {
		already := false
		for _, p := range seen {
			if p == s.Project {
				already = true
				break
			}
		}
		if !already {
			seen = append(seen, s.Project)
		}
		if len(seen) == n {
			break
		}
	}
	for _, p := range seen {
		out[p] = true
	}
	return out
}

func (m Model) Init() tea.Cmd {
	// Bubble Tea calls Init once on program start; we defer the first preview
	// load to the WindowSizeMsg path so the model has known dimensions and a
	// settled cursor before scheduling the load.
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, m.scheduleLoad()

	case tea.KeyMsg:
		// global keys first
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Esc):
			if m.focus == FocusSearch {
				m.search.Blur()
				m.focus = FocusTree
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.PreviewFocus) && m.focus != FocusSearch:
			switch m.focus {
			case FocusTree:
				m.focus = FocusPreview
			case FocusPreview:
				m.focus = FocusTree
			}
			return m, nil
		}

		// route by focus
		switch m.focus {
		case FocusSearch:
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg, m.keys)
			m.tree.SetFilter(m.search.Value())
			return m, tea.Batch(cmd, m.scheduleLoad())
		case FocusPreview:
			var cmd tea.Cmd
			m.preview, cmd = m.preview.Update(msg, m.keys)
			return m, cmd
		case FocusTree:
			// intercept Enter for resume; pass other keys to tree
			if key.Matches(msg, m.keys.Enter) {
				return m.handleEnter()
			}
			// Tree-navigation keys (arrows AND their vim-style aliases h/j/k/l)
			// route to the tree even though h/j/k/l are technically printable.
			// Without this, vim-style nav characters would jump to the search
			// bar instead of moving the cursor.
			if isTreeNavKey(msg, m.keys) {
				var cmd tea.Cmd
				m.tree, cmd = m.tree.Update(msg, m.keys)
				return m, tea.Batch(cmd, m.scheduleLoad())
			}
			// Any other printable char focuses the search bar.
			if isPrintable(msg) {
				cmd := m.search.Focus()
				m.focus = FocusSearch
				m.search, _ = m.search.Update(msg, m.keys)
				m.tree.SetFilter(m.search.Value())
				return m, tea.Batch(cmd, m.scheduleLoad())
			}
			// Non-printable special keys (PgUp/PgDn/Home/End/etc.) → tree.
			var cmd tea.Cmd
			m.tree, cmd = m.tree.Update(msg, m.keys)
			return m, tea.Batch(cmd, m.scheduleLoad())
		}

	case tea.MouseMsg:
		return m, nil

	case DebounceMsg:
		// Older debounce ticks ignore themselves once a newer cursor move
		// has bumped the seq.
		if msg.Seq != m.debounceSeq {
			return m, nil
		}
		parts := strings.SplitN(m.pendingLoad, "|", 2)
		if len(parts) != 2 {
			return m, nil
		}
		return m, LoadCmd(m.store, parts[0], parts[1])

	case LoadedMsg:
		// Match the loaded session to a SessionInfo from the current build
		// so the preview header has the right metadata.
		var sess *data.SessionInfo
		sessions, _ := m.store.Build()
		for i := range sessions {
			if sessions[i].SessionID == msg.SessionID && sessions[i].Project == msg.Project {
				sess = &sessions[i]
				break
			}
		}
		m.preview.SetMessages(msg.Messages, sess)
		return m, nil
	}
	return m, nil
}

// scheduleLoad starts a debounce timer for the cursor's current session row.
// If the cursor is on a project (or no row), the preview is cleared.
func (m *Model) scheduleLoad() tea.Cmd {
	row := m.tree.SelectedRow()
	if row.Kind != RowSession {
		// No active session under cursor — clear so a stale preview isn't shown.
		if m.preview.Current() != nil {
			m.preview.Clear()
		}
		m.pendingLoad = ""
		return nil
	}
	// If we're already showing this session, no need to re-load.
	if cur := m.preview.Current(); cur != nil &&
		cur.SessionID == row.Session.SessionID && cur.Project == row.Project {
		return nil
	}
	m.debounceSeq++
	m.pendingLoad = row.Project + "|" + row.Session.SessionID
	return DebounceCmd(previewDebounce, m.debounceSeq)
}

func (m *Model) layout() {
	if m.width < 60 || m.height < 10 {
		return
	}
	leftW := m.width * 45 / 100
	rightW := m.width - leftW - 4
	// Chrome rows: search bar (1 input + 1 bottom border = 2) +
	// pane top/bottom borders (2) + footer (1) = 5.
	bodyH := m.height - 5 // search bar + footer
	// SetSize gets the CONTENT width: pane width minus 2 borders AND minus
	// 2 padding cells (StylePane has Padding(0, 1)). Without subtracting
	// the padding, a row of exactly leftW-2 visible cells will overflow
	// and the terminal will wrap it onto a second line, pushing rows below
	// it past the pane bottom.
	m.tree.SetSize(leftW-4, bodyH)
	m.preview.SetSize(rightW-4, bodyH)
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	row := m.tree.SelectedRow()
	if row.Kind == RowProject {
		m.tree.expanded[row.Project] = !m.tree.expanded[row.Project]
		return m, nil
	}
	if row.Kind != RowSession {
		return m, nil
	}
	// ACTIVE warning is added in Task 13. For now, just exec.
	return m, tea.ExecProcess(buildClaudeCmd(row.Project, row.Session.SessionID), func(err error) tea.Msg {
		if err != nil {
			return errMsg{err}
		}
		return tea.QuitMsg{}
	})
}

type errMsg struct{ err error }

func buildClaudeCmd(project, sessionID string) *exec.Cmd {
	claudeBin, _ := exec.LookPath("claude")
	if claudeBin == "" {
		claudeBin = "claude"
	}
	c := exec.Command(claudeBin, "--resume", sessionID)
	c.Dir = project
	return c
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.err != nil {
		return fmt.Sprintf("error: %v\n", m.err)
	}
	if m.width < 60 || m.height < 10 {
		return "terminal too small (need ≥ 60×10)\n"
	}
	leftW := m.width * 45 / 100
	rightW := m.width - leftW - 4
	// Chrome rows: search bar (1 input + 1 bottom border = 2) +
	// pane top/bottom borders (2) + footer (1) = 5.
	bodyH := m.height - 5

	leftStyle := StylePane.Width(leftW).Height(bodyH)
	rightStyle := StylePane.Width(rightW).Height(bodyH)
	if m.focus == FocusTree {
		leftStyle = StylePaneFocused.Width(leftW).Height(bodyH)
	}
	if m.focus == FocusPreview {
		rightStyle = StylePaneFocused.Width(rightW).Height(bodyH)
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		leftStyle.Render(m.tree.View()),
		rightStyle.Render(m.preview.View()),
	)
	return lipgloss.JoinVertical(lipgloss.Left,
		m.search.View(),
		body,
		m.footer(),
	)
}

func (m Model) footer() string {
	return StyleHelpDesc.Render(
		" ↑↓ nav  ←→ collapse/expand  Enter resume  type=filter  /=content  p=preview  ?=help  Esc=quit",
	)
}

func isPrintable(k tea.KeyMsg) bool {
	if len(k.Runes) == 0 {
		return false
	}
	r := k.Runes[0]
	return r >= ' ' && r != 127
}

// isTreeNavKey returns true for any binding that the tree pane handles directly,
// including vim-style aliases (h/j/k/l) that are otherwise printable runes.
func isTreeNavKey(msg tea.KeyMsg, keys KeyMap) bool {
	return key.Matches(msg, keys.Up) ||
		key.Matches(msg, keys.Down) ||
		key.Matches(msg, keys.Left) ||
		key.Matches(msg, keys.Right)
}
