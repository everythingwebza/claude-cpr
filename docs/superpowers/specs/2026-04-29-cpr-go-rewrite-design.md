# cpr — Go + Bubble Tea Rewrite

- **Date:** 2026-04-29
- **Status:** Approved (brainstorm); ready for implementation plan
- **Owner:** michael
- **Repo location:** `/home/michael/dev/ai/wsl/cpr/`

## 1. Context

`cpr` is a personal CLI for browsing, searching, and resuming Claude Code sessions. The current implementation is a 1,622-line Python 3 stdlib-only TUI at `/home/michael/scripts/claude-projects` (aliased to `cpr`). It works, but the hand-rolled rendering loop (manual ANSI escape sequences, line-counting redraw, `\033[NA\033[J` cursor-up + clear) suffers from a class of bugs that surface as visual artifacts — most visibly, the project-path line getting "stranded" below the freshly rendered selection when down-arrow scrolling crosses a project boundary on a short terminal.

Beyond the rendering bug, the UX is one-screen-at-a-time: pick → drill → preview → back. The user's actual mental model is a *hub* — see all projects and their recent sessions at a glance, scan, search, resume. The Python tool has the data layer but not the layout for that experience.

Goal: rewrite the UI in Go using Bubble Tea so the redraw class of bugs disappears structurally, then build a hub-style 2-pane interface (project tree + live preview) on top of the existing data model.

## 2. Goals

- **G1.** Eliminate the redraw artifact bug. The new TUI must clip rendering into bounded panes such that no clear/redraw mismatch is possible regardless of terminal size.
- **G2.** Hub layout: project tree (collapsible, with sessions nested) on the left, live conversation preview on the right.
- **G3.** Always-on filter input at the top; `/` escalates to a content-search overlay (transcripts).
- **G4.** Auto-preview with 150 ms debounce + LRU cache, so cursoring through sessions feels live without thrashing I/O.
- **G5.** Persistent state: cursor position, expanded/collapsed projects, pinned projects, sort mode survive across runs.
- **G6.** Single static binary at `~/.local/bin/cpr`. No Python dependency. Drop the `cpr` shell alias.
- **G7.** All current data-source behaviour preserved: history.jsonl + sessions-index.json + per-session JSONL fusion, custom-title detection, ACTIVE-process detection, ripgrep-backed content search.

## 3. Non-goals

- Multi-platform support beyond Linux/WSL2 (`/proc`-based ACTIVE detection is Linux-only and that's accepted).
- Cloud/sync/multi-user functionality.
- Editing or deleting sessions from within `cpr` (rename is in scope; delete is not).
- Multi-select or bulk operations.
- Replacing or wrapping `claude` itself — `cpr` only launches it.

## 4. Decisions (recorded from brainstorm)

| # | Question | Choice |
|---|---|---|
| 1 | Hub layout | **B** — 2-pane: collapsible project tree on left, preview on right |
| 2 | Search behaviour | **A** — always-on filter bar at top; `/` escalates to content-search overlay |
| 3 | Tree default state | **B** — top 2 most-recent projects expanded; rest collapsed; state persisted |
| 4 | Preview update model | **C** — auto-preview, 150 ms debounce + LRU cache (max 16 sessions) |
| 5 | Feature scope | Defined in §8 (port / cut / new) |
| 6 | Distribution | **B** — repo at `cpr/`, binary installed to `~/.local/bin/cpr`, alias dropped |

## 5. Architecture

### 5.1 Tech stack

- Go 1.22+ (single static binary; instant cold start)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — Elm-architecture event loop
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — styling (replaces all manual ANSI handling)
- [Bubbles](https://github.com/charmbracelet/bubbles) — stock `textinput`, `viewport`, `key` components
- Pure stdlib for everything else (JSON parsing, mtime polling on refresh, `pgrep` shell-out)

### 5.2 Bubble Tea root model

```go
type Model struct {
    tree    TreeModel       // left pane: projects + nested sessions
    preview PreviewModel    // right pane: rendered conversation
    search  SearchModel     // top: filter input + mode (filter|content)
    overlay OverlayModel    // help / content-search results / rename modal
    state   PersistedState  // cursor, expansion, pinned, sortMode
    data    *SessionStore   // cached session list + active-process set
    focus   Focus           // tree | search | overlay
    width, height int
}
```

`Update` routes key events by `focus`, then by pane state. `View` composes panes via `lipgloss.JoinHorizontal`/`JoinVertical` with explicit width/height — viewport clipping at the framework level is what makes the redraw artifact bug structurally impossible.

### 5.3 Event flow

```
key event → Update(rootModel)
  ├─ if focus=search:  route to SearchModel; emit FilterChanged when query changes
  ├─ if focus=tree:    route to TreeModel; emit CursorMoved | SessionSelected | Resume
  └─ if focus=overlay: route to overlay (help, content-search, rename)

CursorMoved → debounced 150ms (tea.Tick, cancelled on each move) → PreviewLoad cmd
PreviewLoad → parse session JSONL in goroutine → tea.Msg → preview viewport update
FilterChanged → recompute tree's visible rows in-process (sub-100µs fuzzy match)
"/" key → switch to content-search overlay → rg shell-out cmd → results overlay
Resume → tea.ExecProcess to chdir + exec claude --resume <id>
```

All I/O is async via `tea.Cmd` — cursor movement never blocks waiting for transcript parse.

## 6. File layout

```
cpr/
├── claude-projects                # original Python kept as reference (read-only)
├── README.md                      # short user-facing docs
├── Makefile                       # build, install, test, run, clean, uninstall, bootstrap
├── go.mod / go.sum
├── main.go                        # flag parsing → tea.NewProgram
├── internal/
│   ├── data/
│   │   ├── store.go               # SessionStore: build_session_list equivalent + mtime cache
│   │   ├── history.go             # parse history.jsonl
│   │   ├── index.go               # parse sessions-index.json
│   │   ├── transcript.go          # extract user/assistant messages, dedup by msg ID
│   │   ├── active.go              # /proc + pgrep detection
│   │   └── testdata/              # synthetic fixtures for unit tests
│   ├── ui/
│   │   ├── model.go               # root Bubble Tea model + Update + View
│   │   ├── tree.go                # tree pane (TreeModel)
│   │   ├── preview.go             # preview pane (PreviewModel)
│   │   ├── search.go              # search bar (SearchModel)
│   │   ├── overlay.go             # help / content-search / rename overlays
│   │   ├── styles.go              # Lipgloss styles (colors match Python theme)
│   │   └── keys.go                # KeyMap (bubbles/key-driven, drives help auto-gen)
│   ├── state/
│   │   └── persist.go             # ~/.claude/.cpr-state.json read/write (atomic)
│   └── search/
│       └── content.go             # rg shell-out → grep → pure-Go fallback
└── docs/
    └── superpowers/specs/
        └── 2026-04-29-cpr-go-rewrite-design.md   # this document
```

The `internal/data` package has zero UI dependencies (no Bubble Tea imports) so its tests are pure Go against `testdata/` fixtures.

## 7. Data layer

### 7.1 Source merge (preserved from Python)

For each session, the canonical record is built from three sources, in this order of precedence for each field:

| Field | Source |
|---|---|
| `project` | `history.jsonl` (or sessions-index `originalPath`) |
| `sessionId` | `history.jsonl` (or sessions-index entry) |
| `title` | first `type:custom-title` line in session JSONL → else sessions-index `summary` → else last useful prompt from `history.jsonl` → else `(untitled)` |
| `modified` | session-file mtime → else last `history.jsonl` ts → else sessions-index `modified` |
| `msgCount` | max(history count, index `messageCount`) |
| `branch` | sessions-index `gitBranch` |

Sessions present in either source are included. Sessions with no timestamp at all are dropped.

### 7.2 Caching strategy

`SessionStore` keyed by `(project, sessionId)`. Each entry remembers the file mtime it was built from.

- `history.jsonl` re-parsed only if its mtime changed since last build.
- Per-session metadata (custom-title) re-read only if that session JSONL's mtime changed.
- Transcript LRU cache for preview: max 16 sessions, evict oldest. Each entry stores the parsed message list, keyed by `(project, sessionId, mtime)` so a stale cache entry is automatically invalidated when the session file changes.

### 7.3 Active project detection

Linux-only, accepted scope:

```
pgrep -a -x claude   →   list of (pid, cmdline)
for each pid:        os.Readlink("/proc/<pid>/cwd")  →  set of active project paths
```

Failures (no `pgrep`, permission denied on `/proc`) degrade silently — no `ACTIVE` badge shown, no error to user.

## 8. Feature scope

### 8.1 Ported from Python (kept)

- Source merge described in §7.1
- Active-project detection (§7.3)
- Resume by `chdir(project) && exec claude --resume <id>` (Go: `tea.ExecProcess`, see §10)
- "New session" affordance: `n` on a focused project starts `claude` fresh in that project
- Help overlay (`?`)
- Dark theme matching current cyan/green/red/dim greys
- Non-TTY fallback (when stdin isn't a TTY, emit a plain numbered list to stdout)
- Ripgrep-backed content search (rg → grep → pure-Go)

### 8.2 Cut from Python

- `-n` / `-p` quick-picker count flags (the picker becomes the unified hub; no separate "quick" mode)
- `cpr list` / `cpr search` subcommands (everything is one screen now)
- The triple-line render per session (replaced by single-line rows with branch/msgs inline)

### 8.3 New

- Persistent state file (§9)
- Sort modes for sessions within a project, cycled with `s`: recent (default) / msgCount / alpha
- Pin a project (`P`) — pinned projects always at top, marked with a glyph
- Rename session (`r`) — writes a `type:custom-title` line into the session JSONL (same mechanism Claude itself uses)
- Auto-preview with debounce + LRU cache (Decision #4)
- Resize handling (Bubble Tea `tea.WindowSizeMsg` reflows the layout)
- Mouse: click-to-select rows, scroll-to-scroll preview (Bubble Tea built-in)

### 8.4 Explicitly out of scope

- Deleting sessions (`rm` is fine if needed)
- Multi-select / bulk ops
- Editing session content
- Cloud/sync/multi-user

## 9. Persisted state

File: `~/.claude/.cpr-state.json`

```json
{
  "version": 1,
  "lastCursor": {
    "project": "/home/michael/dev/ai/wsl",
    "sessionId": "abc-123"
  },
  "expanded": {
    "/home/michael/dev/ai/wsl": true,
    "/home/michael/dev/sym/donor": true,
    "/home/michael/dev/lab/scratch": false
  },
  "pinned": ["/home/michael/dev/ai/wsl"],
  "sortMode": "recent"
}
```

**Behaviour:**

- Written on graceful exit AND on each significant change (resume, pin toggle, sort change).
- Atomic write: write to `<file>.tmp`, then rename. Survives crashes mid-write.
- Missing file → defaults: top-2 most-recent projects expanded, no pins, sort=recent.
- Corrupt file (JSON parse error) → log to stderr, fall back to defaults, do not error to user.
- `version` field reserved for future schema migration.

## 10. Resume

```go
// after Update sees an Enter on a session row
return m, tea.ExecProcess(
    exec.Command(claudeBinPath, "--resume", sessionId).WithDir(projectPath),
    func(err error) tea.Msg { … },
)
```

`tea.ExecProcess` restores the terminal cleanly (cursor visible, raw mode off, alt-screen exited) before handing control to `claude` — we don't have to manually undo the TUI state ourselves.

ACTIVE-project warning: if `pgrep` shows a `claude` process already in that cwd, render a y/N prompt overlay before exec.

## 11. Search behaviour

### 11.1 Always-on filter (the top bar)

- Pure-Go fuzzy match (lib: `github.com/sahilm/fuzzy`, fallback hand-rolled if dep is rejected).
- Filters tree in place: only matching projects/sessions are shown; matched parents are kept for context; matched session text is highlighted.
- Empty input = full tree.
- Sub-100 µs over a few hundred items; safe to recompute on every keystroke.

### 11.2 Content search (`/`)

- Modal overlay over the main view. Typing builds the query; Enter runs it.
- Engine selection (in order of preference):
  1. `rg -c -i --no-messages -g '*.jsonl' -g '!*index*' <query> ~/.claude/projects`
  2. `grep -r -c -i --include=*.jsonl <query> ~/.claude/projects`
  3. Pure-Go scan (only if both `rg` and `grep` are missing — virtually never on Linux).
- Timeout: 30 s. On timeout, surface "search took too long" in the overlay and return to tree.
- Results render as a flat list: project + session title + match count + 1-line snippet of the first match. Enter resumes; `p` previews with matches highlighted; Esc closes the overlay and returns to the tree. The content-search modal has its own input (independent of the always-on filter bar). The always-on filter bar's value is preserved while the overlay is open, so the tree still shows the filtered state when the user Escs back.

## 12. Error handling philosophy

- **Data layer:** never fatal. Missing files, malformed JSON lines, permission errors → write to stderr only when `CPR_DEBUG=1` is set in the environment, otherwise swallow silently. Continue with what's available. The user always gets a working tool, even with partial data.
- **UI:** any panic in a sub-model is recovered at the root `Update` and surfaced as a status-bar error message rather than crashing.
- **Search:** see §11.2 for timeouts. rg/grep non-zero exit (no matches) is not an error.
- **Resume failure** (e.g., `claude` not on PATH): show error in status bar, don't quit — let the user retry.
- **State file corruption:** see §9.

## 13. Testing

- **`internal/data/`**: unit tests against checked-in `testdata/` fixtures (small synthetic `history.jsonl`, `sessions-index.json`, `<sid>.jsonl`). Cover session merging, custom-title parsing, dedup of streaming chunks, mtime-based cache invalidation.
- **`internal/state/`**: round-trip tests (write → read → equal); corruption recovery test (write garbage, ensure defaults apply); atomic-write test (truncate mid-write, ensure file is still parseable).
- **`internal/search/`**: integration test that shells out to real `rg` against a temp dir of fixtures; pure-Go-fallback test.
- **`internal/ui/`**: Bubble Tea has an official `teatest` package — script keystrokes, snapshot rendered output. **Specifically include a regression test for the redraw artifact:** render a tree of multiple projects on a 24-row terminal, simulate cursor walking down across a project boundary, assert the output buffer contains exactly one occurrence of each project header line per frame.
- All tests run with `go test ./... -race` from a Makefile target.

## 14. Build, install, distribution

### 14.1 Makefile targets

```makefile
build:      go build -ldflags="-s -w" -o bin/cpr ./
install:    build && install -m 0755 bin/cpr ~/.local/bin/cpr
test:       go test ./... -race
run:        build && ./bin/cpr
clean:      rm -rf bin/
uninstall:  rm -f ~/.local/bin/cpr
bootstrap:  prints one-time setup instructions
```

### 14.2 One-time setup

1. Install Go 1.22+. Either `sudo apt install golang-go` (check it's ≥ 1.22) or download from go.dev.
2. From `cpr/`: `make install`.
3. Edit `~/.bashrc`: remove the line `alias cpr='claude-projects'`. Open a new shell.
4. Type `cpr`. (`~/.local/bin` is already on `$PATH` — verified.)

The Python file at `/home/michael/scripts/claude-projects` is left in place during transition. Once the Go binary has been used and trusted (suggested: one week of regular use), `rm /home/michael/scripts/claude-projects` retires it. The reference copy at `cpr/claude-projects` is preserved indefinitely.

## 15. Implementation phasing (high-level)

These are sequencing notes for the implementation plan, not a final task list — the writing-plans skill will produce the detailed plan.

1. **Phase 1 — Data layer port.** `internal/data/`: history, index, transcript, active, store, with fixtures + tests. No UI yet. CLI subcommand `cpr --dump-sessions` for sanity-checking output against the Python.
2. **Phase 2 — Skeleton TUI.** Root model, tree pane, search bar, hard-coded preview pane, basic key bindings. Renders, navigates, resumes. No persistence, no overlays yet.
3. **Phase 3 — Preview + content search.** Auto-preview with debounce + LRU; content-search overlay; help overlay.
4. **Phase 4 — Persistence + extras.** State file, pin, sort modes, rename, ACTIVE warning on resume.
5. **Phase 5 — Polish + regression test.** Mouse, resize handling, the explicit teatest regression test for the redraw artifact, README, Makefile bootstrap.

Each phase ends in a working binary; phasing supports incremental review and a usable tool by the end of Phase 2.

## 16. Open questions

None at design time. Items deliberately deferred to the implementation plan rather than this design:

- Exact fuzzy-match library choice (`sahilm/fuzzy` vs hand-rolled — decided in Phase 2).
- Exact LRU implementation (stdlib container/list vs a small dep — decided in Phase 3).
- Whether the dump-sessions sanity command becomes a permanent flag or just dev scaffolding.
