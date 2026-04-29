package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/everythingwebza/claude-cpr/internal/data"
)

func main() {
	var dump bool
	flag.BoolVar(&dump, "dump-sessions", false, "print merged session list as JSON, then exit")
	flag.Parse()

	home, _ := os.UserHomeDir()
	historyPath := filepath.Join(home, ".claude", "history.jsonl")
	projectsRoot := filepath.Join(home, ".claude", "projects")

	store, err := data.NewSessionStore(historyPath, projectsRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "store init:", err)
		os.Exit(1)
	}
	sessions, err := store.Build()
	if err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}

	if dump {
		json.NewEncoder(os.Stdout).Encode(sessions)
		return
	}

	fmt.Printf("cpr — placeholder. %d sessions loaded across %d projects.\n",
		len(sessions), countProjects(sessions))
}

func countProjects(s []data.SessionInfo) int {
	set := map[string]struct{}{}
	for _, ss := range s {
		set[ss.Project] = struct{}{}
	}
	return len(set)
}
