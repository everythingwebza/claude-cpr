package ui

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/everythingwebza/claude-cpr/internal/data"
)

// PreviewModel is filled out in Task 11 (debounced auto-load + LRU).
type PreviewModel struct {
	vp            viewport.Model
	width, height int
	current       *data.SessionInfo
	body          string
}

func NewPreviewModel() PreviewModel {
	return PreviewModel{vp: viewport.New(0, 0)}
}
func (m *PreviewModel) SetSize(w, h int) {
	m.width, m.height = w, h
	m.vp.Width, m.vp.Height = w, h
	m.vp.SetContent(m.body)
}
func (m PreviewModel) Update(msg tea.Msg, keys KeyMap) (PreviewModel, tea.Cmd) {
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}
func (m PreviewModel) View() string {
	if m.body == "" {
		return StyleDim.Render("  (preview will appear here)")
	}
	return m.vp.View()
}
