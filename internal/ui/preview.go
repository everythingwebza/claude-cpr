package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/everythingwebza/claude-cpr/internal/data"
)

// PreviewModel renders a session's conversation in a scrollable viewport.
// Content is loaded asynchronously via LoadCmd → LoadedMsg → SetMessages.
type PreviewModel struct {
	vp            viewport.Model
	width, height int
	current       *data.SessionInfo
	body          string
	highlight     string
}

func NewPreviewModel() PreviewModel {
	return PreviewModel{vp: viewport.New(0, 0)}
}

func (m *PreviewModel) SetSize(w, h int) {
	m.width, m.height = w, h
	// The viewport reserves 2 lines for the project/title header we render
	// in View(). Keep the viewport itself one line shorter so the combined
	// output fits in the pane.
	vpH := h - 3
	if vpH < 1 {
		vpH = 1
	}
	m.vp.Width = w
	m.vp.Height = vpH
	m.vp.SetContent(m.body)
}

// SetHighlight sets the term to highlight inside rendered messages
// (used by content-search preview in Task 13).
func (m *PreviewModel) SetHighlight(h string) { m.highlight = h }

// LoadedMsg is dispatched by LoadCmd when an async transcript load completes.
type LoadedMsg struct {
	Project   string
	SessionID string
	Messages  []data.Message
}

// LoadCmd loads a transcript via the store and emits LoadedMsg.
// Errors are swallowed: missing/unreadable session yields zero messages.
func LoadCmd(store *data.SessionStore, project, sessionID string) tea.Cmd {
	return func() tea.Msg {
		msgs, err := store.Transcript(project, sessionID, 200)
		if err != nil {
			return LoadedMsg{Project: project, SessionID: sessionID}
		}
		return LoadedMsg{Project: project, SessionID: sessionID, Messages: msgs}
	}
}

// DebounceMsg is dispatched after the debounce window elapses. The receiver
// compares Seq against its current pending sequence to detect supersession.
type DebounceMsg struct{ Seq int64 }

// DebounceCmd schedules a DebounceMsg after delay.
func DebounceCmd(delay time.Duration, seq int64) tea.Cmd {
	return tea.Tick(delay, func(_ time.Time) tea.Msg {
		return DebounceMsg{Seq: seq}
	})
}

// SetMessages renders the messages into the viewport body.
func (m *PreviewModel) SetMessages(messages []data.Message, sess *data.SessionInfo) {
	m.current = sess
	if len(messages) == 0 {
		m.body = StyleDim.Render("  (no messages)")
		m.vp.SetContent(m.body)
		m.vp.GotoTop()
		return
	}
	var b strings.Builder
	for _, msg := range messages {
		if msg.Role == "user" {
			b.WriteString(StyleUserMsg.Render("▶ USER:"))
		} else {
			b.WriteString(StyleAssistantMsg.Render("◆ CLAUDE:"))
		}
		b.WriteString("\n")
		text := msg.Text
		if m.highlight != "" {
			text = highlightAll(text, m.highlight)
		}
		for _, line := range strings.Split(text, "\n") {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	m.body = b.String()
	m.vp.SetContent(m.body)
	m.vp.GotoTop()
}

// Clear empties the preview (used when the cursor moves to a non-session row).
func (m *PreviewModel) Clear() {
	m.current = nil
	m.body = ""
	m.vp.SetContent("")
}

// Current returns the currently-displayed session, or nil if cleared.
func (m PreviewModel) Current() *data.SessionInfo { return m.current }

func (m PreviewModel) Update(msg tea.Msg, keys KeyMap) (PreviewModel, tea.Cmd) {
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m PreviewModel) View() string {
	if m.body == "" {
		return StyleDim.Render("  (cursor a session to preview)")
	}
	header := ""
	if m.current != nil {
		title := m.current.Title
		if m.width > 0 && len(title) > m.width-4 {
			title = title[:m.width-7] + "..."
		}
		hLine1 := StyleProject.Render(shortProjectName(m.current.Project)) + "  " + StyleSession.Render(title)
		meta := fmt.Sprintf("%s · %d msgs", m.current.Modified, m.current.MsgCount)
		if m.current.Branch != "" {
			meta += " · " + m.current.Branch
		}
		hLine2 := StyleDim.Render(meta)
		if m.width > 0 {
			hLine1 = ansi.Truncate(hLine1, m.width, "")
			hLine2 = ansi.Truncate(hLine2, m.width, "")
		}
		header = hLine1 + "\n" + hLine2 + "\n\n"
	}
	return header + m.vp.View()
}

// highlightAll wraps every (case-insensitive) occurrence of `term` in `text`
// with the highlight style. Returns the text unchanged when term is empty.
func highlightAll(text, term string) string {
	if term == "" {
		return text
	}
	lo := strings.ToLower(text)
	lt := strings.ToLower(term)
	var b strings.Builder
	i := 0
	for {
		idx := strings.Index(lo[i:], lt)
		if idx < 0 {
			b.WriteString(text[i:])
			return b.String()
		}
		b.WriteString(text[i : i+idx])
		b.WriteString(StyleHighlight.Render(text[i+idx : i+idx+len(term)]))
		i += idx + len(term)
	}
}
