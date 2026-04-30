package search

import (
	"bufio"
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Result is one matching session and its match count.
type Result struct {
	Project   string
	SessionID string
	Count     int
}

// ResultsMsg is dispatched as a tea.Msg by callers that wrap Search.
type ResultsMsg struct {
	Results []Result
	Err     error
}

// Search runs full-text content search against rootDir. Engine selection:
// rg → grep → pure-Go scan. Caller is responsible for resolving the on-disk
// directory name back to its originalPath; here we return only the dir name
// (Project) and SessionID (file stem).
func Search(ctx context.Context, rootDir, query string) ([]Result, error) {
	if rg, _ := exec.LookPath("rg"); rg != "" {
		return searchRg(ctx, rg, rootDir, query)
	}
	if grep, _ := exec.LookPath("grep"); grep != "" {
		return searchGrep(ctx, grep, rootDir, query)
	}
	return SearchPureGo(rootDir, query)
}

func searchRg(ctx context.Context, rg, rootDir, query string) ([]Result, error) {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// -F: fixed-string match (not regex). --: end of options, so a query
	// like "--pre=/bin/sh" is treated as a literal pattern, not a flag.
	cmd := exec.CommandContext(cctx, rg,
		"-c", "-i", "-F", "--no-messages",
		"-g", "*.jsonl", "-g", "!*index*",
		"--", query, rootDir,
	)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	_ = cmd.Run() // exit 1 == no match is fine
	return parseGrepOutput(out.String()), nil
}

func searchGrep(ctx context.Context, grep, rootDir, query string) ([]Result, error) {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// -F + --: same hardening as the rg path above.
	cmd := exec.CommandContext(cctx, grep,
		"-r", "-c", "-i", "-F", "--include=*.jsonl",
		"--", query, rootDir,
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()
	return parseGrepOutput(out.String()), nil
}

func parseGrepOutput(s string) []Result {
	out := []Result{}
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.LastIndexByte(line, ':')
		if idx < 0 {
			continue
		}
		path := line[:idx]
		n, err := strconv.Atoi(line[idx+1:])
		if err != nil || n == 0 {
			continue
		}
		if strings.Contains(path, "index") {
			continue
		}
		out = append(out, Result{
			Project:   filepath.Base(filepath.Dir(path)),
			SessionID: strings.TrimSuffix(filepath.Base(path), ".jsonl"),
			Count:     n,
		})
	}
	return out
}

// SearchPureGo is the fallback used when neither rg nor grep is available.
func SearchPureGo(rootDir, query string) ([]Result, error) {
	out := []Result{}
	needle := []byte(strings.ToLower(query))
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".jsonl" || strings.Contains(d.Name(), "index") {
			return nil
		}
		// Inline closure so each file's defer fires before the next iteration —
		// otherwise WalkDir would accumulate one open FD per matched file and
		// blow past the per-process limit on large transcript collections.
		count := countMatches(path, needle)
		if count > 0 {
			out = append(out, Result{
				Project:   filepath.Base(filepath.Dir(path)),
				SessionID: strings.TrimSuffix(d.Name(), ".jsonl"),
				Count:     count,
			})
		}
		return nil
	})
	return out, err
}

// countMatches opens path, counts occurrences of needle (already lowercased)
// across all lines, and closes the file before returning.
func countMatches(path string, needle []byte) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	n := 0
	for sc.Scan() {
		line := bytes.ToLower(sc.Bytes())
		n += bytes.Count(line, needle)
	}
	return n
}
