package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/everythingwebza/claude-cpr/internal/data"
	"github.com/everythingwebza/claude-cpr/internal/search"
	"github.com/everythingwebza/claude-cpr/internal/state"
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
	overlay OverlayModel
	focus   Focus

	width, height int
	quitting      bool
	err           error

	// Preview debounce state. debounceSeq is bumped on every cursor move so
	// older pending DebounceMsgs ignore themselves on arrival.
	debounceSeq int64
	pendingLoad string // "project|sessionID"

	state     state.State
	statePath string

	// pendingExec, when set, tells main() to replace the cpr process with
	// `claude` via syscall.Exec AFTER the Bubble Tea program has torn down and
	// restored the terminal. This is the difference between "cpr stays parked
	// as claude's parent" (the old tea.ExecProcess behaviour) and "cpr becomes
	// claude" (no lingering process), which is what the README always promised.
	pendingExec *ExecRequest
}

// ExecRequest is a resolved request to replace the cpr process with a `claude`
// invocation. main() reads it after p.Run() returns and performs the exec.
type ExecRequest struct {
	Bin  string   // absolute path to the claude binary (no PATH search at exec time)
	Args []string // full argv, including Args[0] == Bin
	Dir  string   // working directory to chdir into before exec
}

// ExecRequest returns the pending exec target set by a resume / new-session
// action, or nil if the user quit normally.
func (m Model) ExecRequest() *ExecRequest { return m.pendingExec }

func NewModel(store *data.SessionStore) (Model, error) {
	sessions, err := store.Build()
	if err != nil {
		return Model{}, err
	}
	home, _ := os.UserHomeDir()
	statePath := filepath.Join(home, ".claude", ".cpr-state.json")
	st, _ := state.Load(statePath)

	expanded := defaultExpansion(sessions, 2)
	if len(st.Expanded) > 0 {
		expanded = st.Expanded
	}
	tree := NewTreeModel(sessions, expanded, store.ActiveDirs(), SortMode(st.SortMode))
	if SortMode(st.SortMode) == "" {
		tree.sort = SortRecent
	}
	for _, p := range st.Pinned {
		tree.pinned[p] = true
	}

	// Restore cursor by finding the row matching the saved LastCursor.
	if st.LastCursor.SessionID != "" || st.LastCursor.Project != "" {
		rows := tree.flatten("")
		for i, r := range rows {
			if r.Kind == RowSession &&
				r.Session.SessionID == st.LastCursor.SessionID &&
				r.Project == st.LastCursor.Project {
				tree.cursor = i
				break
			}
			if r.Kind == RowProject && st.LastCursor.SessionID == "" &&
				r.Project == st.LastCursor.Project {
				tree.cursor = i
				break
			}
		}
	}

	return Model{
		store:     store,
		keys:      DefaultKeyMap(),
		tree:      tree,
		search:    NewSearchModel(),
		preview:   NewPreviewModel(),
		overlay:   NewOverlay(),
		focus:     FocusTree,
		state:     st,
		statePath: statePath,
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

// saveState mirrors current model state to disk. Errors are silent (the user
// shouldn't be blocked by state-file failures). No-op if statePath is empty,
// which happens in tests that construct Model{} without a real config path.
func (m *Model) saveState() {
	if m.statePath == "" {
		return
	}
	row := m.tree.SelectedRow()
	switch row.Kind {
	case RowSession:
		m.state.LastCursor = state.Cursor{Project: row.Project, SessionID: row.Session.SessionID}
	case RowProject:
		m.state.LastCursor = state.Cursor{Project: row.Project}
	}
	m.state.Expanded = m.tree.expanded
	m.state.SortMode = string(m.tree.sort)
	m.state.Pinned = collectPinned(m.tree.pinned)
	_ = state.Save(m.statePath, m.state)
}

func collectPinned(m map[string]bool) []string {
	out := []string{}
	for k, v := range m {
		if v {
			out = append(out, k)
		}
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
		// Active overlay swallows all key input until it closes itself.
		if m.overlay.Kind != OverlayNone {
			var ocmd tea.Cmd
			var res OverlayResult
			m.overlay, ocmd, res = m.overlay.Update(msg, m.keys)
			if res.ResumeRequest != nil {
				return m.execResume(*res.ResumeRequest, res.ResumeConfirmed)
			}
			if res.NewSessionRequest != "" {
				return m.execNewSessionForced(res.NewSessionRequest)
			}
			if res.RenameRequest != nil {
				m.applyRename(*res.RenameRequest)
			}
			return m, ocmd
		}

		// global keys first
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			m.saveState()
			return m, tea.Quit
		case key.Matches(msg, m.keys.Esc):
			if m.err != nil {
				m.err = nil
				return m, nil
			}
			if m.focus == FocusSearch {
				m.search.Blur()
				m.focus = FocusTree
				return m, nil
			}
			m.quitting = true
			m.saveState()
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.overlay.OpenHelp()
			return m, nil
		case key.Matches(msg, m.keys.Search):
			sessions, _ := m.store.Build()
			return m, m.overlay.OpenContent(sessions)
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
			// New session in the focused project (or the cursor session's
			// parent project, if the cursor is on a session row). Runs
			// `claude` (no --resume) in the project directory.
			if key.Matches(msg, m.keys.NewSess) {
				row := m.tree.SelectedRow()
				if row.Project != "" {
					return m.execNewSession(row.Project)
				}
				return m, nil
			}
			// Pin/unpin the focused project and persist.
			if key.Matches(msg, m.keys.Pin) {
				row := m.tree.SelectedRow()
				if row.Kind == RowProject {
					m.tree.pinned[row.Project] = !m.tree.pinned[row.Project]
					if !m.tree.pinned[row.Project] {
						delete(m.tree.pinned, row.Project)
					}
					m.saveState()
				}
				return m, nil
			}
			// Rename the focused session by writing a new custom-title line.
			if key.Matches(msg, m.keys.Rename) {
				row := m.tree.SelectedRow()
				if row.Kind == RowSession {
					return m, m.overlay.OpenRename(row.Session)
				}
				return m, nil
			}
			// Cycle sort mode and persist: recent → msgcount → alpha → recent.
			if key.Matches(msg, m.keys.Sort) {
				switch m.tree.sort {
				case SortRecent:
					m.tree.sort = SortMsgCount
				case SortMsgCount:
					m.tree.sort = SortAlpha
				default:
					m.tree.sort = SortRecent
				}
				m.saveState()
				return m, nil
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

	case openActiveWarnMsg:
		m.overlay.OpenActiveWarn(msg.sess, msg.newSession)
		return m, nil

	case search.ResultsMsg:
		// Async content-search result. Forward to the overlay if it's open.
		if m.overlay.Kind == OverlayContentResults || m.overlay.Kind == OverlayContentInput {
			var ocmd tea.Cmd
			var res OverlayResult
			m.overlay, ocmd, res = m.overlay.Update(msg, m.keys)
			if res.ResumeRequest != nil {
				return m.execResume(*res.ResumeRequest, res.ResumeConfirmed)
			}
			return m, ocmd
		}
		return m, nil

	case LoadedMsg:
		// Drop stale loads: if the cursor has moved on while this transcript
		// was being parsed, pendingLoad will already point elsewhere. Without
		// this guard the user sees the wrong transcript flash up.
		expected := msg.Project + "|" + msg.SessionID
		if m.pendingLoad != "" && m.pendingLoad != expected {
			return m, nil
		}
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

	case errMsg:
		// Surface resume / new-session failures (e.g. claude not on PATH or
		// project dir gone). Stays visible until the next model action that
		// clears m.err.
		m.err = msg.err
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
	return m.execResume(row.Session, false)
}

// execNewSession runs `claude` (no --resume) in the project directory, starting
// a fresh session. Same ACTIVE-warning rule as execResume — if a claude
// process is already running there, prompt y/N first.
func (m Model) execNewSession(project string) (tea.Model, tea.Cmd) {
	if _, active := m.store.ActiveDirs()[project]; active {
		return m, func() tea.Msg {
			return openActiveWarnMsg{sess: data.SessionInfo{Project: project}, newSession: true}
		}
	}
	return m.execNewSessionForced(project)
}

// execNewSessionForced is the post-confirmation path — bypasses the active
// check (used after the user has answered y in the warning overlay).
func (m Model) execNewSessionForced(project string) (tea.Model, tea.Cmd) {
	return m.requestExec(project, nil)
}

// execResume requests a `claude --resume` exec, OR opens the ACTIVE-warning
// overlay if a claude process is already running in the project AND the user
// hasn't already confirmed via the warning.
func (m Model) execResume(sess data.SessionInfo, confirmed bool) (tea.Model, tea.Cmd) {
	if !confirmed {
		if _, active := m.store.ActiveDirs()[sess.Project]; active {
			return m, func() tea.Msg { return openActiveWarnMsg{sess: sess} }
		}
	}
	return m.requestExec(sess.Project, []string{"--resume", sess.SessionID})
}

// requestExec resolves the claude binary, records the exec target on the model,
// persists state, and quits the Bubble Tea program. main() then chdirs and
// replaces the process via syscall.Exec, so no cpr parent lingers behind the
// claude session. If claude isn't on PATH the error is surfaced in the TUI and
// no quit happens (resolving here, not in main, lets us stay interactive).
func (m Model) requestExec(project string, extraArgs []string) (tea.Model, tea.Cmd) {
	bin, err := exec.LookPath("claude")
	if err != nil || bin == "" {
		m.err = fmt.Errorf("claude not found on PATH")
		return m, nil
	}
	m.pendingExec = &ExecRequest{
		Bin:  bin,
		Args: append([]string{bin}, extraArgs...),
		Dir:  project,
	}
	m.quitting = true
	m.saveState()
	return m, tea.Quit
}

// openActiveWarnMsg asks the root Update to surface the ACTIVE-warning
// overlay for a pending resume or new-session start. We use a message
// instead of mutating the model directly so the call site (execResume,
// execNewSession) can stay a value-receiver helper that returns only a
// tea.Cmd. newSession=true means the user wants to launch `claude` (no
// --resume) instead of resuming sess.SessionID.
type openActiveWarnMsg struct {
	sess       data.SessionInfo
	newSession bool
}

// applyRename writes a new custom-title to the session JSONL, invalidates
// the cached title in the SessionStore, rebuilds the session list, and
// refreshes the tree so the new title shows immediately.
func (m *Model) applyRename(op RenameOp) {
	// Defence-in-depth: parsers already reject invalid session IDs, but the
	// rename flow joins the ID into a filesystem path so re-validate here.
	if !data.IsValidSessionID(op.Session.SessionID) {
		return
	}
	dir := strings.ReplaceAll(op.Session.Project, "/", "-")
	sessionsRoot := filepath.Join(filepath.Dir(m.statePath), "projects")
	file := filepath.Join(sessionsRoot, dir, op.Session.SessionID+".jsonl")
	if err := data.WriteCustomTitle(file, op.NewTitle); err != nil {
		return // silent: failure leaves the title unchanged
	}
	m.store.InvalidateCustomTitle(file)
	if sessions, err := m.store.Build(); err == nil {
		m.tree.sessions = sessions
	}
}

type errMsg struct{ err error }

func (m Model) View() string {
	if m.quitting {
		return ""
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
	main := lipgloss.JoinVertical(lipgloss.Left,
		m.search.View(),
		body,
		m.footer(),
	)
	if m.overlay.Kind != OverlayNone {
		// Center the overlay over the main view.
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.overlay.View(),
			lipgloss.WithWhitespaceChars(" "))
	}
	return main
}

func (m Model) footer() string {
	if m.err != nil {
		return StyleActive.Render(fmt.Sprintf(" error: %v ", m.err)) +
			StyleHelpDesc.Render("(Esc to dismiss)")
	}
	return StyleHelpDesc.Render(
		fmt.Sprintf(" ↑↓ nav  ←→ fold  Enter resume  n new  /=search  s=sort:%s  p=preview  ?=help  q=quit",
			sortLabel(m.tree.sort)),
	)
}

// sortLabel renders a SortMode for the footer indicator so the active sort is
// always visible (otherwise msgcount/alpha order reads as "random").
func sortLabel(s SortMode) string {
	switch s {
	case SortMsgCount:
		return "msgs"
	case SortAlpha:
		return "a-z"
	default:
		return "recent"
	}
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
