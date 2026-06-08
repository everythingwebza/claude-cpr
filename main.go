package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/everythingwebza/claude-cpr/internal/data"
	"github.com/everythingwebza/claude-cpr/internal/ui"
	"github.com/mattn/go-isatty"
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

	// Non-TTY (e.g. piped to head/grep) → emit a flat numbered list and exit.
	// Lets `cpr | head` and similar shell pipelines work without crashing on
	// alt-screen escape codes.
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		sessions, err := store.Build()
		if err != nil {
			fmt.Fprintln(os.Stderr, "build:", err)
			os.Exit(1)
		}
		for i, s := range sessions {
			if i >= 50 {
				break
			}
			fmt.Printf("%3d  %-20s  %s\n", i+1, shortName(s.Project), s.Title)
		}
		return
	}

	m, err := ui.NewModel(store)
	if err != nil {
		fmt.Fprintln(os.Stderr, "model:", err)
		os.Exit(1)
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	final, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tui:", err)
		os.Exit(1)
	}

	// If the user chose to resume / start a session, the program quit with an
	// exec request set. p.Run() has already torn down Bubble Tea and restored
	// the terminal (left alt-screen, disabled mouse, cooked mode), so we can
	// replace this process with `claude` via execve — claude inherits a clean
	// terminal and NO cpr parent lingers behind it. This is why resume no
	// longer leaves a parked cpr process per session.
	if fm, ok := final.(ui.Model); ok {
		if req := fm.ExecRequest(); req != nil {
			if req.Dir != "" {
				if err := os.Chdir(req.Dir); err != nil {
					fmt.Fprintf(os.Stderr, "cpr: cannot enter %s: %v\n", req.Dir, err)
					os.Exit(1)
				}
			}
			if err := syscall.Exec(req.Bin, req.Args, os.Environ()); err != nil {
				fmt.Fprintf(os.Stderr, "cpr: exec %s: %v\n", req.Bin, err)
				os.Exit(1)
			}
		}
	}
}

// shortName returns the last 1-2 path segments for the non-TTY listing.
func shortName(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return p
}
