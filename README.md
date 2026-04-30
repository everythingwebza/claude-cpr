# cpr — Claude session hub

A Bubble Tea TUI to browse, search, and resume Claude Code sessions across all your projects.

## Features

- **2-pane hub.** Project tree on the left (collapsible, expanded state persists). Live conversation preview on the right.
- **Always-on filter.** Just start typing — fuzzy-match project names and session titles in place. Esc clears.
- **`/` for content search.** Ripgrep-backed full-text search across every transcript; results overlay shows match counts; Enter resumes.
- **Auto-preview** with 150 ms debounce + LRU cache. Cursor settles → preview loads. Doesn't thrash on rapid scrolling; revisits are instant.
- **State persists.** Cursor position, expanded projects, pinned projects, sort mode survive across runs (`~/.claude/.cpr-state.json`).
- **Resume in place.** Enter on a session `chdir`s into the project and `exec`s `claude --resume <id>`.
- **Active-session detection.** Projects with a running `claude` process are flagged with a red `*` and prompt y/N before resume.
- **Pin / sort / rename** without leaving the hub. Pinned projects stick to the top; sort cycles recent → msgcount → alpha; rename writes a `custom-title` line into the session JSONL.
- **Non-TTY fallback.** `cpr | head` emits a plain numbered list — composable in shell pipelines.

## Install

Prerequisites:

- Go 1.22+ (download from [go.dev/dl](https://go.dev/dl) if your distro lags)
- Optional: `ripgrep` for fast content search (`sudo apt install ripgrep`); `grep` and a pure-Go fallback are used otherwise

Then:

```bash
git clone https://github.com/everythingwebza/claude-cpr.git
cd claude-cpr
make install        # builds and installs to ~/.local/bin/cpr
```

Drop the legacy alias from `~/.bashrc` if you have it:

```bash
sed -i "/alias cpr='claude-projects'/d" ~/.bashrc
```

Open a new shell and type `cpr`.

## Keys

| Key | Action |
|---|---|
| `↑↓` / `jk` | navigate tree |
| `←/→` / `hl` | collapse / expand current project |
| `Enter` | resume session (or expand project) |
| any letter | live-filter the tree |
| `/` | content-search overlay (full-text across transcripts) |
| `p` / `Tab` | focus preview pane (then `↑↓` scrolls inside it) |
| `P` | pin / unpin focused project |
| `s` | cycle sort: recent → msg count → alpha |
| `r` | rename focused session (writes `custom-title`) |
| `?` | help overlay |
| `Esc` / `q` | back / quit (saves state) |

## Files used

- `~/.claude/history.jsonl` — recent prompts (read)
- `~/.claude/projects/<encoded-path>/sessions-index.json` — Claude's index (read)
- `~/.claude/projects/<encoded-path>/<session>.jsonl` — full transcripts (read; appended-to on rename)
- `~/.claude/.cpr-state.json` — `cpr`'s own state (read/write, atomic)

## CLI

```bash
cpr                    # launch the TUI
cpr --dump-sessions    # print merged session list as JSON
cpr | head             # non-TTY: numbered plain list
```

## Debugging

Set `CPR_DEBUG=1` to enable stderr logs from the data layer.

## Project layout

```
cpr/
├── claude-projects          # original Python script (reference, kept read-only)
├── main.go                  # flag parsing, non-TTY fallback, tea.NewProgram
├── internal/
│   ├── data/                # SessionStore + parsers (history, index, transcript, active)
│   ├── ui/                  # Bubble Tea models (root, tree, search, preview, overlay)
│   ├── state/               # ~/.claude/.cpr-state.json read/write
│   └── search/              # rg → grep → pure-Go content-search engine
├── docs/superpowers/
│   ├── specs/               # design doc
│   └── plans/               # 20-task implementation plan
├── Makefile
├── go.mod
└── README.md
```

## License

MIT.
