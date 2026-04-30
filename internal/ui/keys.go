package ui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Left         key.Binding
	Right        key.Binding
	Enter        key.Binding
	Quit         key.Binding
	Esc          key.Binding
	Help         key.Binding
	Search       key.Binding // /
	PreviewFocus key.Binding // p or tab
	Pin          key.Binding // P
	Sort         key.Binding // s
	Rename       key.Binding // r
	NewSess      key.Binding // n
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:           key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:         key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "collapse")),
		Right:        key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "expand")),
		Enter:        key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/resume")),
		Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Esc:          key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back/quit")),
		Help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Search:       key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "content search")),
		PreviewFocus: key.NewBinding(key.WithKeys("p", "tab"), key.WithHelp("p/tab", "preview focus")),
		Pin:          key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "pin project")),
		Sort:         key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort sessions")),
		Rename:       key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename session")),
		NewSess:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new session")),
	}
}
