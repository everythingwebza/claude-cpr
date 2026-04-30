package ui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// SearchModel is the always-on filter input above the tree.
type SearchModel struct {
	input   textinput.Model
	focused bool
}

func NewSearchModel() SearchModel {
	ti := textinput.New()
	ti.Placeholder = "filter…  (type to filter, / for content search)"
	ti.Prompt = "🔍 "
	ti.CharLimit = 200
	return SearchModel{input: ti}
}

// Focus puts the cursor in the input.
func (m *SearchModel) Focus() tea.Cmd { m.focused = true; return m.input.Focus() }

// Blur removes focus.
func (m *SearchModel) Blur() { m.focused = false; m.input.Blur() }

// Value returns the current filter text.
func (m SearchModel) Value() string { return m.input.Value() }

// Clear empties the input.
func (m *SearchModel) Clear() { m.input.SetValue("") }

// Update routes keys when focused. Esc returns the input but signals "blur me" via a custom msg.
func (m SearchModel) Update(msg tea.Msg, keys KeyMap) (SearchModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(k, keys.Esc) {
			m.Blur()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the filter bar.
func (m SearchModel) View() string {
	return StyleSearchBar.Render(m.input.View())
}
