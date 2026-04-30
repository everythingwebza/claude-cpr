package ui

import "github.com/charmbracelet/lipgloss"

// Centralised lipgloss styles. Colors mirror the Python theme.
var (
	StylePane = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	StylePaneFocused = StylePane.Copy().
				BorderForeground(lipgloss.Color("39")) // bright cyan

	StyleProject = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).Bold(true)

	StyleSession = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255"))

	StyleDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	StyleSelected = lipgloss.NewStyle().
			Background(lipgloss.Color("24")).
			Foreground(lipgloss.Color("231")).Bold(true)

	StyleActive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).Bold(true) // red

	StyleBranch = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141")) // magenta

	StyleSearchBar = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	StyleHelpKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("231")).Bold(true)

	StyleHelpDesc = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	StyleUserMsg = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).Bold(true) // green

	StyleAssistantMsg = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")).Bold(true) // cyan

	StyleHighlight = lipgloss.NewStyle().
			Background(lipgloss.Color("130")).Foreground(lipgloss.Color("231")).Bold(true)
)
