# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-03-04

First public release.

### Added

- **TUI Dashboard** with Sessions / Stats / Projects tabs, 30-second auto-refresh
- **kubectl-style CLI**: `taux get sessions`, `taux get projects`, `taux get stats`, `taux describe`, `taux logs`, `taux attach`, `taux kill`, `taux delete`, `taux memorize`, `taux clean`, `taux setup`, `taux uninstall`, `taux status`
- **tmux native integration**: `prefix + H/A/S` popups, status bar widget (`taux status` — sessions, messages, tokens, cost in <50ms)
- **Multi-provider support**:
  - Claude Code (JSONL, full support)
  - Cursor (SQLite, full support)
  - OpenAI Codex CLI (JSONL, full support)
  - Google Gemini CLI (JSON, full support)
  - Aider (Markdown, basic support)
- **Cost tracking**: built-in pricing table for Claude, GPT, Gemini models; per-model and per-period cost display in CLI, TUI, and tmux status bar; user-configurable price overrides via `config.toml`
- **Session memorize**: archive conversations to markdown and optionally delete originals (`--keep` to preserve)
- **Session alias**: press `n` to set a nickname, searchable via `/`
- **Confirm dialogs**: y/N confirmation for kill, memorize, clean
- **Search/filter**: `/` to filter by ID, alias, project, model, status
- **Token analysis**: input/output breakdown with cache_read/cache_write stats
- **Cross-platform installer**: `install.sh` with GitHub Release download + source build fallback
- **Release pipeline**: GoReleaser + GitHub Actions CI/CD (darwin/linux, amd64/arm64)
