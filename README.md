# cpr

> Browse, search, and resume your **Claude Code** sessions — across every project — from a single fast TUI.

`cpr` reads the JSONL session logs that [Claude Code](https://docs.claude.com/en/docs/claude-code) writes under `~/.claude/` and gives you a hub view: collapsible project tree on the left, live conversation preview on the right, full-text search across every transcript, and one-key resume.

```
┌─ cpr ─────────────────────── 🔍 filter…  (type to filter, / for content search) ─────┐
│ ▾ * acme/api                            │ ▶ USER:                                    │
│   ▸ refactor auth middleware    2m  87  │   refactor auth middleware                 │
│   ▸ add rate limiting           3h  54  │                                            │
│ ▾   acme/dashboard                      │ ◆ CLAUDE:                                  │
│   ▸ tweak chart colors          12m 23  │   I'll extract the token validator first…  │
│   ▸ fix login bug               1d  41  │                                            │
│ ▸   contoso/web-store           1d  …   │                                            │
│ ▸   side/llm-experiments        2d  …   │                                            │
│ ▸   scripts/utils               3d  …   │                                            │
├─────────────────────────────────────────────────────────────────────────────────────┤
│ ↑↓ nav  ←→ collapse/expand  Enter resume  type=filter  /=content  p=preview  ?=help │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

## Why

If you use Claude Code across more than a couple of projects, finding the right session to resume gets old fast. The built-in resume picker only knows about the project you're standing in, and `~/.claude/projects/` is one folder per project, one JSONL per session — not exactly a UX. `cpr` aggregates all of it: every project, every session, sorted by recency, with a live preview so you can recognise a conversation at a glance before resuming.

## Features

- **2-pane hub.** Collapsible project tree, expanded state persists.
- **Always-on fuzzy filter.** Just start typing — no modal switch.
- **`/` for content search.** Ripgrep-backed full-text across every transcript; results overlay; Enter resumes.
- **Auto-preview** with 150 ms debounce + LRU cache. Settles before loading; revisits are instant.
- **Persistent state.** Cursor position, expansions, pins, sort mode survive across runs (`~/.claude/.cpr-state.json`).
- **Resume in place.** Enter on a session `chdir`s into the project and `exec`s `claude --resume <id>`.
- **Active-session warning.** A red `*` flags projects that already have a `claude` process running; y/N prompt before resume avoids conflicts.
- **Pin / sort / rename** without leaving the hub. Pinned projects float to the top; sort cycles recent → msg-count → alpha; rename writes a `custom-title` line into the JSONL (same mechanism Claude itself uses).
- **Non-TTY fallback.** `cpr | head` emits a plain numbered list — composable in shell pipelines.

## Install

### Prerequisites

- Linux (uses `/proc/<pid>/cwd` for active-session detection; tested on WSL2 + native Ubuntu)
- [Go 1.22+](https://go.dev/dl/) — install Go from go.dev rather than `apt` if your distro lags
- Optional: [`ripgrep`](https://github.com/BurntSushi/ripgrep) for fast content search (`grep` and a pure-Go fallback are used otherwise)

### Quick install

```bash
go install github.com/everythingwebza/claude-cpr@latest
```

This drops a `claude-cpr` binary in `$(go env GOPATH)/bin`. If you'd rather call it `cpr`, symlink:

```bash
ln -sf $(go env GOPATH)/bin/claude-cpr ~/.local/bin/cpr
```

### From source

```bash
git clone https://github.com/everythingwebza/claude-cpr
cd claude-cpr
make install        # builds with -ldflags="-s -w" and installs to ~/.local/bin/cpr
```

Make sure `~/.local/bin` is on your `$PATH`. Then:

```bash
cpr
```

## Keybindings

| Key | Action |
|---|---|
| `↑↓` / `jk` | navigate tree |
| `←→` / `hl` | collapse / expand current project |
| `Enter` | resume session (or expand project if cursor's on a project) |
| any letter | live-filter the tree |
| `/` | content-search overlay (full-text across all transcripts) |
| `p` / `Tab` | focus preview pane (then `↑↓` scrolls inside it) |
| `P` | pin / unpin focused project |
| `s` | cycle sort: recent → msg count → alpha |
| `r` | rename focused session |
| `n` | new session in focused project |
| `?` | help overlay |
| `Esc` / `q` | back / quit (saves state) |

## Files used

| Path | Purpose |
|---|---|
| `~/.claude/history.jsonl` | recent prompts (read) |
| `~/.claude/projects/<encoded>/sessions-index.json` | Claude's per-project index (read) |
| `~/.claude/projects/<encoded>/<session>.jsonl` | full transcripts (read; appended on rename) |
| `~/.claude/.cpr-state.json` | `cpr`'s own state (read/write, atomic) |

`cpr` never modifies session content beyond appending a single `{"type":"custom-title", ...}` line when you rename — the same mechanism Claude itself uses.

## CLI flags

```bash
cpr                    # launch the TUI
cpr --dump-sessions    # print the merged session list as JSON, then exit
cpr | head             # non-TTY: numbered plain list (no escape codes)
```

Set `CPR_DEBUG=1` to enable stderr logs from the data layer.

## How it works

The data layer (`internal/data/`) merges three sources for each session:

1. `history.jsonl` — your prompt history, used for `LastTS` and a fallback title.
2. `sessions-index.json` — Claude's own per-project summary index (gives you titles + branch + msg count when available).
3. `<session>.jsonl` — the full transcript, used for the preview pane and for the user-set `custom-title` (which trumps the auto-generated index summary).

The UI (`internal/ui/`) is [Bubble Tea](https://github.com/charmbracelet/bubbletea) with [Lipgloss](https://github.com/charmbracelet/lipgloss) for layout. Each pane renders into a bounded region and clips its own content — no manual `\033[NA\033[J` clear-and-redraw, so the visual artifacts that plagued the original Python version structurally cannot occur.

## Building / testing

```bash
make build         # binary at bin/cpr
make test          # all packages, race-detector enabled
make run           # build & launch
make uninstall     # remove ~/.local/bin/cpr
```

## Limitations

- **Linux only.** Active-session detection reads `/proc/<pid>/cwd`, which is Linux-specific. macOS and Windows-native support are not on the roadmap (WSL2 works fine).
- **No session deletion.** Intentional — your transcripts are valuable. Use `rm` if you really need to.
- **Search isn't fuzzy at the content layer.** The `/` overlay does substring-match (case-insensitive) via ripgrep/grep. Title filtering on the always-on bar IS fuzzy.

## Contributing

Issues and PRs welcome. The implementation is intentionally compact — under 2,000 lines of Go across 4 internal packages. Run `make test` before submitting.

## License

MIT — see [LICENSE](LICENSE).

---

🤖 Built with [Claude Code](https://docs.claude.com/en/docs/claude-code) — the Go rewrite was itself driven through a multi-task TDD plan executed by Claude. The original Python version (`claude-projects`) lives in this repo for reference.
