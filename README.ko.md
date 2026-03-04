<p align="center">
  <img src="./taux_banner.svg" alt="taux" width="600">
</p>

<p align="center">
  <strong>Manage, observe, and clean up your AI agent sessions — without leaving your terminal.</strong>
</p>

<p align="center">
  <a href="./README.md">English</a>
</p>

---

## 목차

- [왜 taux인가요?](#왜-taux인가요)
- [설치](#설치)
- [사용법](#사용법)
- [통계](#통계)
- [어떻게 동작하나요?](#어떻게-동작하나요)
- [Provider 지원 현황](#provider-지원-현황)
- [설정](#설정)
- [요구 사항](#요구-사항)
- [라이선스](#라이선스)

---

AI 코딩 에이전트를 여러 개 돌리다 보면, 이 세션이 살아있는 건지 죽은 건지, 토큰은 얼마나 썼는지, 메모리는 괜찮은 건지... 터미널 탭을 헤매게 됩니다.

taux는 AI 코딩 에이전트 세션을 **하나의 tmux 대시보드**에서 모니터링하고 관리하는 TUI 도구입니다. 에이전트를 오케스트레이션하는 도구가 아닙니다. 이미 돌고 있는 세션들을 **잘 보여주는 것**에 집중합니다. htop이 프로세스를 만들지 않듯이, taux는 에이전트를 만들지 않습니다.

![taux demo](./docs/demo.gif)

## 왜 taux인가요?

### 토큰 0개, API 호출 0건

에이전트 모니터링 도구가 또 다른 API 호출을 만들면 본말전도입니다. taux는 로컬 파일(`~/.claude/projects/*.jsonl`)과 `ps`/`lsof`만 읽습니다. 네트워크 요청 없음. 토큰 소모 없음. 돈이 안 듭니다.

### tmux 안에 사는 도구

다른 도구들은 tmux를 "감싸서" 새로운 워크플로우를 만듭니다. taux는 이미 쓰고 있는 tmux에 **녹아듭니다**. `prefix + H`로 팝업, 상태바에 세션 현황 표시. 기존 작업 흐름을 바꿀 필요가 없습니다.

### memorize — 대화를 남기고 세션을 비우기

세션이 쌓이면 디스크가 아깝고, 그냥 지우자니 컨텍스트가 아깝습니다. `taux memorize`는 대화 내용을 마크다운으로 추출한 뒤 원본을 삭제합니다. 나중에 참고할 수 있는 형태로 보관하면서 디스크는 확보하는, 아카이브 워크플로우입니다.

## 설치

```bash
curl -sSL https://raw.githubusercontent.com/glory0216/taux/main/install.sh | sh
```

macOS (Intel/Apple Silicon), Linux (amd64/arm64) 모두 지원. 바이너리 다운로드, 설치, tmux 연동까지 한 번에.

또는 Go가 설치되어 있다면:

```bash
go install github.com/glory0216/taux/cmd/taux@latest
taux setup  # tmux 연동
```

## 사용법

### 실행

```bash
taux              # tmux 밖이면 taux 세션 만들어서 대시보드 실행
                  # tmux 안이면 바로 대시보드 실행
```

그냥 `taux` 한 방이면 됩니다.

### CLI (kubectl 스타일)

조회:
```bash
taux get sessions         # 전체 세션 목록
taux get projects         # 프로젝트별 세션/메모리/CPU 집계
taux get stats            # 토큰 사용량, 캐시 분석, 디스크 사용량
taux describe <id>        # 세션 상세 (토큰, 도구 사용, git branch 등)
taux logs <id>            # 세션 원본 JSONL 출력
```

관리:
```bash
taux attach <id>          # 세션 이어서 작업 (tmux pane에서 열림)
taux kill <id>            # 세션 프로세스 종료 (SIGTERM)
taux delete <id>          # 세션 파일 삭제
taux memorize <id>        # 대화 내용을 마크다운으로 저장 후 삭제
taux memorize --keep <id> # 마크다운으로 저장 (원본 유지)
taux clean --older 720h   # 30일 지난 세션 정리
taux clean --broken       # 타임스탬프 깨진 세션 정리
```

#### kill / delete / memorize 차이

| 명령어 | 동작 | 되돌리기 |
|--------|------|---------|
| `kill` | 실행 중인 프로세스에 SIGTERM 전송. 세션 파일은 유지. | 프로세스 재시작 불가 |
| `delete` | 세션 파일 삭제. 프로세스도 종료됨. | 복구 불가 |
| `memorize` | 대화를 마크다운으로 아카이브 후 원본 삭제. `--keep`으로 원본 유지 가능. | 마크다운 보존 |

### tmux 단축키

설치하면 tmux에서 바로 사용 가능:

| 키 | 기능 |
|-----|------|
| `prefix + H` | 대시보드 팝업 |
| `prefix + A` | 활성 세션 팝업 |
| `prefix + S` | 통계 팝업 |

### 대시보드 단축키

| 키 | 기능 | 비고 |
|-----|------|------|
| `j/k`, `↑/↓` | 세션 탐색 | vim 스타일 |
| `Tab` / `Shift+Tab` | 탭 전환 | Sessions / Stats / Projects |
| `Enter` | 세션 상세 보기 | |
| `a` | 세션 attach | 이어서 대화 |
| `K` | 세션 kill | y/N 확인 |
| `M` | memorize & delete | 아카이브 후 삭제 |
| `m` | memorize (keep) | 아카이브만, 원본 유지 |
| `C` | 깨진 세션 정리 | y/N 확인 |
| `n` | 세션 별명 설정 | 검색에도 활용 |
| `/` | 검색 | ID, 별명, 프로젝트, 모델 |
| `r` | 새로고침 | |
| `q` | 종료 | |

## 통계

```
═══ taux Stats ═══

Today       6 sessions   4,177 messages   107k tokens
                                           + 129.29M cache_read, 3.88M cache_write
This Week   9 sessions   2,502 messages   92.6k tokens
This Month  49 sessions  14,146 messages  577.8k tokens
All Time    70 sessions  26,278 messages  1.39M tokens

Model Usage (input + output)
  claude-opus-4-5-20251101    916.1k tokens
  claude-opus-4-6             392.6k tokens
  claude-sonnet-4-5-20250929  79.6k tokens

Cache Tokens
  cache_read   998.13M tokens  (reused context, 90% discount)
  cache_write  82.19M tokens   (new context cached)
```

## 어떻게 동작하나요?

**API 호출 없음. 토큰 소모 없음.** 로컬 파일만 읽습니다.

- `~/.claude/projects/*/*.jsonl` — 세션 메타데이터
- `~/.claude/stats-cache.json` — 누적 통계 (Claude Code가 생성)
- `ps` + `lsof` — 프로세스 활성 여부, 메모리, CPU
- `~/.config/taux/aliases.json` — 세션 별명
- 대시보드 30초마다 자동 갱신, `r` 키로 즉시 갱신

## Provider 지원 현황

| Provider | 목록 | 상세 | Kill | Delete | 비용 | 데이터 소스 |
|----------|------|------|------|--------|------|-------------|
| Claude Code | ✅ | ✅ | ✅ | ✅ | ✅ | JSONL (`~/.claude/`) |
| Cursor | ✅ | ✅ | ❌ | ❌ | ✅ | SQLite (`state.vscdb`) |
| Codex CLI | ✅ | ✅ | ✅ | ✅ | ✅ | JSONL (`~/.codex/`) |
| Gemini CLI | ✅ | ✅ | ✅ | ✅ | ✅ | JSON (`~/.gemini/`) |
| Aider | ✅ | ✅ | ✅ | ✅ | ❌ | Markdown (`.aider.chat.history.md`) |

## 설정

`~/.config/taux/config.toml`:

```toml
[general]
default_limit = 20
cache_ttl = 10

[providers]
enabled = ["claude", "cursor", "aider"]

[pricing.override.my-custom-model]
input = 5.0
output = 25.0
```

전체 설정 옵션은 [`config.example.toml`](./config.example.toml)을 참고하세요.

## 요구 사항

- Go 1.24+
- tmux 3.0+
- macOS / Linux

> tmux가 처음이라면: [tmux 치트시트](https://tmuxcheatsheet.com/) · [입문 가이드 (Red Hat)](https://www.redhat.com/en/blog/introduction-tmux-linux)

## 라이선스

[MIT](./LICENSE)
