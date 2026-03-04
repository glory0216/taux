# Contributing to taux

> Manage, observe, and clean up your AI agent sessions — without leaving your terminal.

<p align="center">
  <a href="#contributing-to-taux-한국어">한국어</a>
</p>

Thanks for your interest in contributing to taux! Bug reports, feature requests, and code contributions are all welcome.

## Getting Started

```bash
git clone https://github.com/glory0216/taux.git
cd taux
make build        # Build
make test         # Test
make lint         # Lint (requires golangci-lint)
make fmt          # Format
```

## Project Structure

```
taux/
├── cmd/taux/              # Entrypoint
├── internal/
│   ├── cli/               # Cobra command definitions
│   ├── config/            # TOML config, alias management
│   ├── model/             # Data models (Session, Stats, etc.)
│   ├── pricing/           # Built-in model pricing table
│   ├── provider/          # Provider interface & shared filter
│   │   ├── claude/        # Claude Code — JSONL parser
│   │   ├── cursor/        # Cursor — SQLite (state.vscdb) parser
│   │   ├── codex/         # Codex CLI — JSONL parser
│   │   ├── gemini/        # Gemini CLI — JSON parser
│   │   ├── aider/         # Aider — Markdown parser
│   │   └── copilot/       # GitHub Copilot (stub)
│   ├── stats/             # Stats aggregation (shared by CLI & TUI)
│   ├── cache/             # In-memory TTL cache
│   └── tui/               # Bubbletea dashboard
│       └── view/          # View components
├── docs/                  # Demo GIF, VHS tape, setup scripts
├── Makefile
└── install.sh
```

## How to Contribute

### Bug Reports

Please open an issue and include:
- taux version (`taux --version`)
- OS / tmux version
- Steps to reproduce
- Expected vs actual behavior

### Feature Requests

Open an issue first to discuss. Include a brief description and use case.

### Code Contributions

1. Fork & clone
2. Create a branch: `git checkout -b feat/my-feature`
3. Write code + tests
4. Ensure `make fmt && make lint && make test` passes
5. Open a PR

### Commit Messages

```
<type>: <subject>

feat:     New feature
fix:      Bug fix
refactor: Code refactoring
docs:     Documentation
test:     Tests
chore:    Build, CI, etc.
```

Example: `feat: add Cursor session provider`, `fix: scanner buffer overflow on large JSONL`

## Code Style

- `gofmt -s` (standard Go formatting)
- `go vet` with no warnings
- GoDoc comments on exported functions/types
- Wrap errors with `fmt.Errorf("context: %w", err)`

## Adding a New Provider

taux supports 6 providers (Claude, Cursor, Codex, Gemini, Aider, Copilot). To add another:

1. Create `internal/provider/<name>/` directory
2. Implement the `provider.Provider` interface:
   ```go
   type Provider interface {
       Name() string
       DisplayName() string
       Available() bool
       ListSession(ctx context.Context, filter Filter) ([]model.Session, error)
       GetSession(ctx context.Context, id string) (*model.SessionDetail, error)
       GetStatus(ctx context.Context) (*ProviderStatus, error)
       ActiveSession(ctx context.Context) ([]model.Session, error)
       AttachSession(id string) (cmd string, argList []string, workDir string, err error)
       KillSession(ctx context.Context, id string) error
       DeleteSession(ctx context.Context, id string) error
   }
   ```
3. Register the provider in `internal/config/config.go`
4. Submit a PR

## License

All contributions are released under the [MIT License](LICENSE).

---

# Contributing to taux (한국어)

> AI 에이전트 세션을 관리, 모니터링, 정리하세요 — 터미널을 떠나지 않고.

taux에 기여해주셔서 감사합니다! 버그 리포트, 기능 제안, 코드 기여 모두 환영합니다.

## 시작하기

```bash
git clone https://github.com/glory0216/taux.git
cd taux
make build        # 빌드
make test         # 테스트
make lint         # 린트 (golangci-lint 필요)
make fmt          # 포맷팅
```

## 프로젝트 구조

```
taux/
├── cmd/taux/              # 엔트리포인트
├── internal/
│   ├── cli/               # Cobra 커맨드 정의
│   ├── config/            # TOML 설정, alias 관리
│   ├── model/             # 데이터 모델 (Session, Stats 등)
│   ├── pricing/           # 빌트인 모델 가격표
│   ├── provider/          # Provider 인터페이스 & 공유 필터
│   │   ├── claude/        # Claude Code — JSONL 파서
│   │   ├── cursor/        # Cursor — SQLite (state.vscdb) 파서
│   │   ├── codex/         # Codex CLI — JSONL 파서
│   │   ├── gemini/        # Gemini CLI — JSON 파서
│   │   ├── aider/         # Aider — Markdown 파서
│   │   └── copilot/       # GitHub Copilot (stub)
│   ├── stats/             # 통계 집계 (CLI & TUI 공용)
│   ├── cache/             # 인메모리 TTL 캐시
│   └── tui/               # Bubbletea 대시보드
│       └── view/          # 뷰 컴포넌트
├── docs/                  # 데모 GIF, VHS tape, 셋업 스크립트
├── Makefile
└── install.sh
```

## 기여 방법

### 버그 리포트

Issue에 다음을 포함해주세요:
- taux 버전 (`taux --version`)
- OS / tmux 버전
- 재현 단계
- 예상 동작 vs 실제 동작

### 기능 제안

Issue로 먼저 논의해주세요. 간단한 설명과 유스케이스를 포함하면 좋습니다.

### 코드 기여

1. Fork & clone
2. 브랜치 생성: `git checkout -b feat/my-feature`
3. 코드 작성 + 테스트
4. `make fmt && make lint && make test` 통과 확인
5. PR 생성

### 커밋 메시지

```
<type>: <subject>

feat:     새 기능
fix:      버그 수정
refactor: 리팩토링
docs:     문서
test:     테스트
chore:    빌드, CI 등
```

예시: `feat: add Cursor session provider`, `fix: scanner buffer overflow on large JSONL`

## 코드 스타일

- `gofmt -s` 적용 (표준 Go 포맷)
- `go vet` 경고 없음
- 공개 함수/타입에 GoDoc 주석
- 에러는 `fmt.Errorf("context: %w", err)` 형태로 래핑

## 새 Provider 추가

taux는 6개 provider를 지원합니다 (Claude, Cursor, Codex, Gemini, Aider, Copilot). 새 provider를 추가하려면:

1. `internal/provider/<name>/` 디렉토리 생성
2. `provider.Provider` 인터페이스 구현:
   ```go
   type Provider interface {
       Name() string
       DisplayName() string
       Available() bool
       ListSession(ctx context.Context, filter Filter) ([]model.Session, error)
       GetSession(ctx context.Context, id string) (*model.SessionDetail, error)
       GetStatus(ctx context.Context) (*ProviderStatus, error)
       ActiveSession(ctx context.Context) ([]model.Session, error)
       AttachSession(id string) (cmd string, argList []string, workDir string, err error)
       KillSession(ctx context.Context, id string) error
       DeleteSession(ctx context.Context, id string) error
   }
   ```
3. `internal/config/config.go`에 provider 설정 추가
4. PR 제출

## 라이선스

기여하신 코드는 [MIT License](LICENSE)로 배포됩니다.
