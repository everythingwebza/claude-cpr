package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/everythingwebza/claude-cpr/internal/data"
	"github.com/everythingwebza/claude-cpr/internal/ui"
)

func main() {
	var dump bool
	flag.BoolVar(&dump, "dump-sessions", false, "print merged session list as JSON, then exit")
	flag.Parse()

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "home dir:", err)
		os.Exit(1)
	}
	historyPath := filepath.Join(home, ".claude", "history.jsonl")
	projectsRoot := filepath.Join(home, ".claude", "projects")

	store, err := data.NewSessionStore(historyPath, projectsRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "store init:", err)
		os.Exit(1)
	}

	if dump {
		sessions, err := store.Build()
		if err != nil {
			fmt.Fprintln(os.Stderr, "build:", err)
			os.Exit(1)
		}
		json.NewEncoder(os.Stdout).Encode(sessions)
		return
	}

	m, err := ui.NewModel(store)
	if err != nil {
		fmt.Fprintln(os.Stderr, "model:", err)
		os.Exit(1)
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui:", err)
		os.Exit(1)
	}
}
