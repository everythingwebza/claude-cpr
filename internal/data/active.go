package data

import (
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
	cmd := exec.Command(pgrep, "-a", "-x", "claude")
	cmd.WaitDelay = 5 * time.Second
	b, _ := cmd.Output() // exit status 1 (no matches) is fine

	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// pgrep -a output: "<pid> <cmdline>"
		var pidStr string
		if sp := strings.IndexByte(line, ' '); sp > 0 {
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
