package search

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearch_PureGoFallback(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "-foo-bar")
	if err := os.MkdirAll(proj, 0755); err != nil {
		t.Fatal(err)
	}
	sess := filepath.Join(proj, "abc.jsonl")
	body := `{"type":"user","message":{"role":"user","content":"please postgres pool tuning"}}` + "\n" +
		`{"type":"assistant","message":{"role":"assistant","id":"a1","content":[{"type":"text","text":"sure"}]}}` + "\n"
	if err := os.WriteFile(sess, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	res, err := SearchPureGo(dir, "POSTGRES POOL")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("got %d results, want 1: %+v", len(res), res)
	}
	if res[0].SessionID != "abc" {
		t.Errorf("SessionID: got %q, want abc", res[0].SessionID)
	}
	if res[0].Count != 1 {
		t.Errorf("Count: got %d, want 1", res[0].Count)
	}
}
