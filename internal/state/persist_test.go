package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s := State{
		Version:    1,
		LastCursor: Cursor{Project: "/p", SessionID: "sid"},
		Expanded:   map[string]bool{"/p": true},
		Pinned:     []string{"/p"},
		SortMode:   "msgcount",
	}
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastCursor.Project != "/p" || got.SortMode != "msgcount" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestState_CorruptFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("expected nil err on corrupt, got %v", err)
	}
	if got.Version != 1 {
		t.Errorf("default Version: got %d, want 1", got.Version)
	}
}

func TestState_MissingFileReturnsDefaults(t *testing.T) {
	got, err := Load("/no/such/file/state.json")
	if err != nil {
		t.Fatal(err)
	}
	if got.Expanded == nil {
		t.Error("Expanded should be non-nil default")
	}
}
