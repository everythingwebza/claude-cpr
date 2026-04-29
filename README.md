# cpr — Claude session hub

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
