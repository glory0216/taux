# v0.1.0 — First Release

> extend tmux for AI sessions.
>
> Manage, observe, and clean up your AI agent sessions — without leaving your terminal.

AI 코딩 에이전트 세션들을 하나의 tmux 대시보드에서 모니터링하고 관리하는 TUI 도구의 첫 릴리즈입니다.

에이전트를 오케스트레이션하는 도구가 아닙니다. 이미 돌고 있는 세션들을 **잘 보여주는 것**에 집중합니다. htop이 프로세스를 만들지 않듯이, taux는 에이전트를 만들지 않습니다.

## Why taux?

- **토큰 0개, API 호출 0건** — 로컬 파일(`*.jsonl`)과 `ps`/`lsof`만 읽습니다. 모니터링 도구가 토큰을 소모하면 본말전도입니다.
- **tmux 안에 사는 도구** — 다른 도구들은 tmux를 감싸서 새 워크플로우를 강제합니다. taux는 이미 쓰고 있는 tmux에 녹아들어 `prefix + H` 한 방이면 됩니다.
- **memorize** — 세션을 마크다운으로 아카이브하고 원본을 삭제합니다. 컨텍스트는 남기고 디스크는 확보하는, 다른 도구에 없는 워크플로우입니다.

## Highlights

- **TUI 대시보드** — Sessions / Stats / Projects 3개 탭, 30초 자동 갱신
- **kubectl 스타일 CLI** — `taux get sessions`, `taux describe`, `taux logs` 등
- **tmux 네이티브** — `prefix + H/A/S` 팝업, 상태바 통합
- **멀티 프로바이더** — Claude Code, Cursor, Codex CLI, Gemini CLI, Aider 지원
- **비용 추적** — 빌트인 가격표, 모델/기간별 비용 표시, tmux 상태바 통합
- **세션 관리** — attach, kill, delete, memorize, clean
- **토큰 분석** — input/output + cache_read/cache_write 별도 표시
- **세션 별명** — `n` 키로 alias 설정, `/` 검색에 반영
- **확인 다이얼로그** — kill, memorize, clean 시 y/N 확인

## CLI Commands

| 명령 | 설명 |
|------|------|
| `taux` | 대시보드 실행 (tmux 세션 자동 생성) |
| `taux get sessions` | 전체 세션 목록 (`-s active/dead`, `--project`, `-n`) |
| `taux get projects` | 프로젝트별 세션/메모리/CPU 집계 |
| `taux get stats` | 토큰 사용량, 캐시 분석, 디스크 사용량 |
| `taux describe <id>` | 세션 상세 (토큰, 도구 사용, git branch 등) |
| `taux logs <id>` | 대화 원문 출력 (`--tail`, `--no-tools`) |
| `taux attach <id>` | 세션 이어서 작업 |
| `taux kill <id>` | 세션 종료 (SIGTERM) |
| `taux delete <id>` | 세션 파일 삭제 |
| `taux memorize <id>` | 마크다운으로 저장 후 삭제 (`--keep`, `-o`) |
| `taux clean` | 오래된/깨진 세션 정리 (`--older-than`, `--broken`, `--dry-run`) |
| `taux setup` | tmux.conf에 taux 설정 블록 추가 (`--dry-run`) |
| `taux uninstall` | 설정/바이너리 완전 제거 |
| `taux status` | tmux 상태바용 한줄 출력 (<50ms) |

## Dashboard

- **Sessions 탭**: 상태, ID, alias, 환경(CLI/IDE), 프로젝트, 모델, 메시지 수, 크기, 메모리, CPU, 경과 시간
- **Stats 탭**: Today/Week/Month/All Time 통계, 모델별 토큰, 캐시 토큰, 디스크 사용량
- **Projects 탭**: 프로젝트별 세션 수, 활성 수, 디스크 사용량

### 단축키

| 키 | 기능 |
|----|------|
| `j/k`, `↑/↓` | 세션 탐색 |
| `Tab` / `Shift+Tab` | 탭 전환 |
| `Enter` | 세션 상세 |
| `a` | attach |
| `K` | kill (y/N) |
| `M` | memorize & delete (아카이브 후 삭제) |
| `m` | memorize / keep (아카이브만, 원본 유지) |
| `C` | clean broken (y/N) |
| `n` | alias 설정 |
| `/` | 검색 (ID, alias, 프로젝트, 모델) |
| `r` | 새로고침 |
| `q` | 종료 |

## tmux Integration

- `prefix + H` — 대시보드 팝업
- `prefix + A` — 활성 세션 팝업
- `prefix + S` — 통계 팝업
- 상태바: `⬡ 3/8  142msg  12.4k tok`

## Provider Support

| Provider | Status | Data Source |
|----------|--------|-------------|
| Claude Code | Full | JSONL (`~/.claude/`) |
| Cursor | Full | SQLite (`state.vscdb`) |
| Codex CLI | Full | JSONL (`~/.codex/`) |
| Gemini CLI | Full | JSON (`~/.gemini/`) |
| Aider | Basic | Markdown (`.aider.chat.history.md`) |

## Install

```bash
git clone github.com/glory0216/taux.git
cd taux
./install.sh
```

## Requirements

- Go 1.24+
- tmux 3.0+
- macOS / Linux
