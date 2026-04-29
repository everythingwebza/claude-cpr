# cpr — Claude session hub

> **Status:** Bootstrap only. The TUI described below is the planned interface; the binary currently prints a placeholder. Implementation lands in subsequent tasks of [the plan](docs/superpowers/plans/2026-04-29-cpr-go-rewrite.md).

Browse, search, and resume Claude Code sessions. 2-pane hub: project tree on the left, live preview on the right.

## Install

    make install

Then drop the alias from `~/.bashrc`:

    sed -i "/alias cpr='claude-projects'/d" ~/.bashrc

## Keys

| Key | Action |
|---|---|
| `↑↓` / `jk` | navigate tree |
| `←→` | collapse / expand project |
| `Enter` | resume session / start new |
| type | filter the tree |
| `/` | full-text content search |
| `p` / `Tab` | focus preview pane (scroll) |
| `?` | help |
| `Esc` / `q` | quit |
