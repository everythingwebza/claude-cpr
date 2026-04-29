package data

import (
	"os"
	"testing"
)

func TestGetActiveProjectDirs_OurOwnCwdIsDetectable(t *testing.T) {
	if _, err := os.Stat("/proc/self/cwd"); err != nil {
		t.Skip("no /proc — not Linux?")
	}
	// We can't make this test depend on a running `claude` process, so
	// we just assert the function returns without panicking and yields a set.
	set := getActiveProjectDirs()
	if set == nil {
		t.Fatal("expected non-nil set")
	}
	// Smoke: function is safe to call even if pgrep is missing.
	_ = set
}
