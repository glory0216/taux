<p align="center">
  <img src="./taux_banner.svg" alt="taux" width="600">
</p>

<p align="center">
  <strong>Manage, observe, and clean up your AI agent sessions — without leaving your terminal.</strong>
</p>

<p align="center">
  <a href="https://github.com/glory0216/taux/releases"><img src="https://img.shields.io/github/v/release/glory0216/taux?style=flat-square" alt="Release"></a>
  <a href="https://github.com/glory0216/taux/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/glory0216/taux/ci.yml?style=flat-square&label=CI" alt="CI"></a>
  <a href="https://github.com/glory0216/taux/blob/main/LICENSE"><img src="https://img.shields.io/github/license/glory0216/taux?style=flat-square" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/glory0216/taux"><img src="https://goreportcard.com/badge/github.com/glory0216/taux?style=flat-square" alt="Go Report Card"></a>
</p>

<p align="center">
  <a href="./README.ko.md">한국어</a>
</p>

---

## Table of Contents

- [Why taux?](#why-taux)
- [Install](#install)
- [Quick Start](#quick-start)
- [How It Works](#how-it-works)
- [Provider Support](#provider-support)
- [Configuration](#configuration)
- [Requirements](#requirements)
- [License](#license)

---

Running multiple AI coding agents? Losing track of which sessions are alive, how many tokens you've burned, or whether that background agent is eating all your RAM?

taux is a tmux-native TUI dashboard that monitors and manages AI coding agent sessions from a single pane. It's not an agent orchestrator — it focuses on **showing you what's already running**. Just like htop doesn't create processes, taux doesn't create agents.

![taux demo](./docs/demo.gif)

## Why taux?

### Zero tokens, zero API calls

A monitoring tool that burns tokens to monitor token usage is ironic. taux reads local files (`~/.claude/projects/*.jsonl`) and `ps`/`lsof` — nothing else. No network requests. No token consumption. It costs nothing to run.

### Lives inside your tmux

Other tools wrap tmux and impose a new workflow. taux dissolves into the tmux you already use. `prefix + H` opens a popup dashboard. The status bar shows live session counts. Your existing workflow stays untouched.

### memorize — archive conversations, reclaim disk

Sessions pile up and eat disk space, but deleting them loses context. `taux memorize` exports the conversation to markdown, then removes the original. You keep a reference copy while reclaiming storage.

### Git branch awareness

The dashboard and `taux get sessions` show the git branch each session is running on. Claude Code sessions read it from JSONL metadata; other providers detect it from the project directory.

### Session completion notification

When an agent session finishes, taux sends a `tmux display-message` notification within one status refresh cycle (default 10s). No separate daemon needed — the existing `taux status` command tracks active sessions and detects when they disappear.

Disable with `notify_completion = false` in config or `set -g @taux-notify 'off'` in tmux.

### Activity chart

`taux get stats` and the dashboard Stats tab show a 14-day bar chart of daily message activity. See your AI usage trends at a glance — no external dashboards needed.

### replay — browse conversations in the terminal

`taux replay <id>` opens a scrollable TUI viewer for any session's conversation. User/assistant/tool turns are color-coded, tool calls are collapsible with `t`, and you can scroll with vim keys. In the dashboard, press `R` to replay the selected session.

### Task progress tracking

When a session uses `TodoWrite`, `taux describe <id>` shows the task completion status with a checklist. Quickly see whether an agent finished its planned work.

## Install

### One-line installer

```bash
curl -sSL https://raw.githubusercontent.com/glory0216/taux/main/install.sh | sh
```

Supports macOS (Intel/Apple Silicon) and Linux (amd64/arm64). Downloads the binary, installs it, and configures tmux — all in one go.

### TPM (Tmux Plugin Manager)

Add to `~/.tmux.conf`:

```tmux
set -g @plugin 'glory0216/taux'
```

Then press `prefix + I` to install. The binary is downloaded automatically from GitHub Releases.

**Options** (all optional):

```tmux
set -g @taux-key-dashboard 'H'    # Dashboard popup key (default: H)
set -g @taux-key-sessions  'A'    # Active sessions popup key (default: A)
set -g @taux-key-stats     'S'    # Stats popup key (default: S)
set -g @taux-status        'on'   # Show status in status-right (default: on)
set -g @taux-status-interval '10' # Status refresh interval in seconds (default: 10)
set -g @taux-notify        'on'   # Notify on session completion (default: on)
```

### Go install

```bash
go install github.com/glory0216/taux/cmd/taux@latest
taux setup  # configure tmux integration
```

## Quick Start

```bash
taux                      # Launch dashboard (auto-creates tmux session if needed)
```

That's it. One command.

### CLI (kubectl-style)

```bash
taux get sessions         # List all sessions (with git branch)
taux get projects         # Per-project aggregated stats
taux get stats            # Token usage, cache breakdown, activity chart
taux describe <id>        # Full session detail (with task progress)
taux replay <id>          # Browse conversation in scrollable TUI
taux attach <id>          # Resume a session in tmux pane
taux kill <id>            # SIGTERM
taux memorize <id>        # Export to markdown, then delete
taux memorize --keep <id> # Export to markdown (keep original)
taux clean --older 720h   # Remove sessions older than 30 days
taux clean --broken       # Remove corrupted sessions
```

#### kill vs delete vs memorize

| Command | Behavior | Reversibility |
|---------|----------|---------------|
| `kill` | Sends SIGTERM to the running process. Session files remain on disk. | Process cannot be restarted |
| `delete` | Removes session files. Also terminates the process if running. | Irreversible |
| `memorize` | Archives conversation to markdown, then deletes the original. Use `--keep` to preserve the original. | Markdown preserved |

### tmux Shortcuts

| Key | Action |
|-----|--------|
| `prefix + H` | Dashboard popup |
| `prefix + A` | Active sessions popup |
| `prefix + S` | Stats popup |

### Dashboard Keys

| Key | Action |
|-----|--------|
| `j/k`, `↑/↓` | Navigate |
| `Tab` / `Shift+Tab` | Switch tabs (Sessions / Stats / Projects) |
| `Enter` | Detail view |
| `R` | Replay conversation (scrollable viewer) |
| `a` | Attach to session |
| `K` | Kill (with y/N confirm) |
| `M` | Memorize & delete (archive, then remove) |
| `m` | Memorize / keep (archive only, keep original) |
| `C` | Clean broken sessions (with y/N confirm) |
| `n` | Set alias (searchable via `/`) |
| `/` | Filter by ID, alias, project, model |
| `r` | Force refresh |
| `q` | Quit |

## How It Works

**No API calls. No tokens.** Just local file reads.

- `~/.claude/projects/*/*.jsonl` — session metadata
- `~/.claude/stats-cache.json` — cumulative stats (generated by Claude Code)
- `ps` + `lsof` — process detection, memory, CPU
- `~/.config/taux/aliases.json` — session aliases
- Auto-refresh every 30s; `r` for instant refresh

## Provider Support

| Provider | List | Detail | Kill | Delete | Cost | Data Source |
|----------|------|--------|------|--------|------|-------------|
| Claude Code | ✅ | ✅ | ✅ | ✅ | ✅ | JSONL (`~/.claude/`) |
| Cursor | ✅ | ✅ | ❌ | ❌ | ✅ | SQLite (`state.vscdb`) |
| Codex CLI | ✅ | ✅ | ✅ | ✅ | ✅ | JSONL (`~/.codex/`) |
| Gemini CLI | ✅ | ✅ | ✅ | ✅ | ✅ | JSON (`~/.gemini/`) |
| Aider | ✅ | ✅ | ✅ | ✅ | ❌ | Markdown (`.aider.chat.history.md`) |

## Configuration

`~/.config/taux/config.toml`:

```toml
[general]
default_limit = 20
cache_ttl = 10
notify_completion = true   # tmux display-message when a session completes

[providers]
enabled = ["claude", "cursor", "aider"]

[pricing.override.my-custom-model]
input = 5.0
output = 25.0
```

See [`config.example.toml`](./config.example.toml) for all available options.

## Requirements

- Go 1.24+
- tmux 3.0+
- macOS / Linux

> **Note:** Windows is not currently supported.

> New to tmux? [Cheat Sheet](https://tmuxcheatsheet.com/) · [Beginner's Guide (Red Hat)](https://www.redhat.com/en/blog/introduction-tmux-linux)

## License

[MIT](./LICENSE)
