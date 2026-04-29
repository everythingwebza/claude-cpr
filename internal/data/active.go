package data

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// getActiveProjectDirs returns the set of cwds for any running `claude` processes.
// Always returns a non-nil set. Errors degrade silently (empty set).
func getActiveProjectDirs() map[string]struct{} {
	out := map[string]struct{}{}

	pgrep, err := exec.LookPath("pgrep")
	if err != nil {
		return out
	}
	// Hard cap on pgrep runtime so an unresponsive /proc (rare on unusual
	// container setups) cannot hang the caller. WaitDelay ensures the pipe
	// is drained promptly after the context fires.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, pgrep, "-a", "-x", "claude")
	cmd.WaitDelay = 5 * time.Second
	b, _ := cmd.Output() // exit status 1 (no matches) is fine

	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// pgrep -a output: "<pid> <cmdline>" — separator is normally a space
		// on Linux but allow a tab as defensive coverage.
		var pidStr string
		if sp := strings.IndexAny(line, " \t"); sp > 0 {
			pidStr = line[:sp]
		} else {
			pidStr = line
		}
		if _, err := strconv.Atoi(pidStr); err != nil {
			continue
		}
		cwd, err := os.Readlink("/proc/" + pidStr + "/cwd")
		if err != nil {
			continue
		}
		out[cwd] = struct{}{}
	}
	return out
}
