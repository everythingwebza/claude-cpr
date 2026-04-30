package ui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/everythingwebza/claude-cpr/internal/data"
	"github.com/everythingwebza/claude-cpr/internal/search"
)

// OverlayKind identifies which overlay (if any) is currently open.
type OverlayKind int

const (
	OverlayNone OverlayKind = iota
	OverlayHelp
	OverlayContentInput
	OverlayContentResults
	OverlayActiveWarn
	OverlayRename
)

// RenameOp carries a session and the new title the user typed.
type RenameOp struct {
	Session  data.SessionInfo
	NewTitle string
}

// OverlayResult is what the overlay returns to the root model — actions the
// root needs to take (resume, rename) once the overlay's interaction settles.
type OverlayResult struct {
	ResumeRequest *data.SessionInfo
	RenameRequest *RenameOp
}

type OverlayModel struct {
	Kind          OverlayKind
	width, height int

	// Content search state
	input     textinput.Model
	results   []search.Result
	cursor    int
	query     string
	sessByDir map[string]map[string]data.SessionInfo

	// ACTIVE-warning prompt state
	pendingResume *data.SessionInfo

	// Rename state
	renameTarget *data.SessionInfo
}

func NewOverlay() OverlayModel {
	ti := textinput.New()
	ti.Prompt = "search content: "
	ti.CharLimit = 200
	return OverlayModel{input: ti}
}

func (m *OverlayModel) SetSize(w, h int) { m.width, m.height = w, h }

// OpenContent opens the content-search input. The session map enables a
// later result-row to be resolved back to a full SessionInfo (Task 13's
// search package returns only the on-disk dir name, not the originalPath).
func (m *OverlayModel) OpenContent(sessions []data.SessionInfo) tea.Cmd {
	m.Kind = OverlayContentInput
	m.input.SetValue("")
	m.results = nil
	m.cursor = 0
	m.sessByDir = map[string]map[string]data.SessionInfo{}
	for _, s := range sessions {
		dir := strings.ReplaceAll(s.Project, "/", "-")
		if _, ok := m.sessByDir[dir]; !ok {
			m.sessByDir[dir] = map[string]data.SessionInfo{}
		}
		m.sessByDir[dir][s.SessionID] = s
	}
	return m.input.Focus()
}

// OpenHelp shows the help overlay; any key dismisses it.
func (m *OverlayModel) OpenHelp() { m.Kind = OverlayHelp }

// OpenActiveWarn prompts y/N before resuming into a project that already has
// a running claude process.
func (m *OverlayModel) OpenActiveWarn(sess data.SessionInfo) {
	m.Kind = OverlayActiveWarn
	m.pendingResume = &sess
}

// OpenRename opens an input prefilled with the session's current title.
func (m *OverlayModel) OpenRename(sess data.SessionInfo) tea.Cmd {
	m.Kind = OverlayRename
	m.renameTarget = &sess
	m.input.Prompt = "rename session: "
	m.input.SetValue(sess.Title)
	m.input.CursorEnd()
	return m.input.Focus()
}

// Close dismisses any overlay.
func (m *OverlayModel) Close() {
	m.Kind = OverlayNone
	m.input.Blur()
}

// Update routes keys when an overlay is open. Returns the (possibly updated)
// overlay, any pending command, and an OverlayResult describing actions the
// root model should take.
func (m OverlayModel) Update(msg tea.Msg, keys KeyMap) (OverlayModel, tea.Cmd, OverlayResult) {
	res := OverlayResult{}
	if m.Kind == OverlayNone {
		return m, nil, res
	}
	switch t := msg.(type) {
	case tea.KeyMsg:
		switch m.Kind {
		case OverlayHelp:
			m.Close()
			return m, nil, res

		case OverlayContentInput:
			if key.Matches(t, keys.Esc) {
				m.Close()
				return m, nil, res
			}
			if t.Type == tea.KeyEnter {
				m.query = m.input.Value()
				m.Kind = OverlayContentResults
				m.cursor = 0
				return m, runSearchCmd(m.query), res
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd, res

		case OverlayContentResults:
			switch {
			case key.Matches(t, keys.Esc):
				m.Close()
				return m, nil, res
			case key.Matches(t, keys.Up):
				if m.cursor > 0 {
					m.cursor--
				}
			case key.Matches(t, keys.Down):
				if m.cursor < len(m.results)-1 {
					m.cursor++
				}
			case key.Matches(t, keys.Enter):
				if m.cursor < len(m.results) {
					r := m.results[m.cursor]
					if sess, ok := m.sessByDir[r.Project][r.SessionID]; ok {
						res.ResumeRequest = &sess
					}
				}
			}
			return m, nil, res

		case OverlayActiveWarn:
			switch t.String() {
			case "y", "Y":
				if m.pendingResume != nil {
					res.ResumeRequest = m.pendingResume
				}
				m.Close()
			case "n", "N", "esc":
				m.Close()
			}
			return m, nil, res

		case OverlayRename:
			if key.Matches(t, keys.Esc) {
				m.Close()
				m.input.Prompt = "search content: " // restore default
				return m, nil, res
			}
			if t.Type == tea.KeyEnter {
				if m.renameTarget != nil {
					res.RenameRequest = &RenameOp{
						Session:  *m.renameTarget,
						NewTitle: m.input.Value(),
					}
				}
				m.Close()
				m.input.Prompt = "search content: "
				return m, nil, res
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd, res
		}

	case search.ResultsMsg:
		m.results = t.Results
		return m, nil, res
	}
	return m, nil, res
}

// View renders the overlay (caller composes it on top of the main view).
func (m OverlayModel) View() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(1, 2)

	switch m.Kind {
	case OverlayHelp:
		return style.Render(helpText())

	case OverlayContentInput:
		return style.Render(m.input.View() + "\n\n" + StyleDim.Render("Enter to search · Esc to cancel"))

	case OverlayContentResults:
		var b strings.Builder
		b.WriteString(StyleProject.Render("Results for ") + StyleSession.Render(`"`+m.query+`"`) + "\n\n")
		if len(m.results) == 0 {
			b.WriteString(StyleDim.Render("(no matches)"))
			return style.Render(b.String())
		}
		// Cap rows shown so the overlay never grows unbounded.
		visible := 12
		end := len(m.results)
		if end > visible {
			end = visible
		}
		for i := 0; i < end; i++ {
			r := m.results[i]
			line := fmt.Sprintf("%s · %s · %d", r.Project, r.SessionID, r.Count)
			// Truncate to a reasonable overlay-internal width.
			if m.width > 0 {
				line = ansi.Truncate(line, m.width-8, "...")
			}
			if i == m.cursor {
				b.WriteString(StyleSelected.Render("▸ " + line))
			} else {
				b.WriteString("  " + line)
			}
			b.WriteString("\n")
		}
		if len(m.results) > visible {
			b.WriteString(StyleDim.Render(fmt.Sprintf("\n  …and %d more", len(m.results)-visible)))
		}
		b.WriteString("\n" + StyleDim.Render("↑↓ navigate · Enter resume · Esc back"))
		return style.Render(b.String())

	case OverlayActiveWarn:
		if m.pendingResume == nil {
			return ""
		}
		return style.Render(fmt.Sprintf(
			"%s\n\n%s\n\n%s",
			StyleActive.Render("⚠ A claude process is already running in this project."),
			StyleSession.Render(m.pendingResume.Project),
			StyleDim.Render("Resume anyway? (y/N)"),
		))

	case OverlayRename:
		return style.Render(m.input.View() + "\n\n" + StyleDim.Render("Enter to save · Esc to cancel"))
	}
	return ""
}

func helpText() string {
	rows := [][2]string{
		{"↑↓/jk", "navigate"},
		{"←/→", "collapse / expand"},
		{"Enter", "resume / expand project"},
		{"type", "filter the tree"},
		{"/", "content search"},
		{"p / Tab", "focus preview pane"},
		{"P", "pin / unpin project"},
		{"s", "cycle sort mode"},
		{"r", "rename session"},
		{"n", "new session in project"},
		{"?", "this help"},
		{"Esc / q", "back / quit"},
	}
	var b strings.Builder
	b.WriteString(StyleProject.Render("cpr — keys") + "\n\n")
	for _, r := range rows {
		b.WriteString(StyleHelpKey.Render(fmt.Sprintf("%-10s", r[0])))
		b.WriteString("  ")
		b.WriteString(StyleHelpDesc.Render(r[1]))
		b.WriteString("\n")
	}
	b.WriteString("\n" + StyleDim.Render("Press any key to close."))
	return b.String()
}

// runSearchCmd builds the tea.Cmd that runs content search and emits a
// search.ResultsMsg.
func runSearchCmd(query string) tea.Cmd {
	return func() tea.Msg {
		home, err := osHome()
		if err != nil {
			return search.ResultsMsg{Err: err}
		}
		results, err := search.Search(context.Background(), home+"/.claude/projects", query)
		if err != nil {
			return search.ResultsMsg{Err: err}
		}
		return search.ResultsMsg{Results: results}
	}
}

// osHome is wrapped so tests can stub it.
var osHome = func() (string, error) {
	return os.UserHomeDir()
}
