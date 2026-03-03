# Contributing to taux

> extend tmux for AI sessions.
> Manage, observe, and clean up your AI agent sessions — without leaving your terminal.

taux에 기여해주셔서 감사합니다! 버그 리포트, 기능 제안, 코드 기여 모두 환영합니다.

## 시작하기

```bash
git clone github.com/glory0216/taux.git
cd taux
make build        # 빌드
make test         # 테스트
make lint         # 린트
make fmt          # 포맷팅
```

## 프로젝트 구조

```
taux/
├── cmd/taux/          # 엔트리포인트
├── internal/
│   ├── cli/           # cobra 커맨드 정의
│   ├── config/        # TOML 설정, alias 관리
│   ├── model/         # 데이터 모델 (Session, Stats)
│   ├── provider/
│   │   └── claude/    # Claude Code JSONL 파서
│   └── tui/           # bubbletea 대시보드
│       └── view/      # 뷰 컴포넌트
├── docs/              # 데모 GIF, VHS tape
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

Claude 외 다른 AI 에이전트를 지원하려면:

1. `internal/provider/<name>/` 디렉토리 생성
2. `provider.Provider` 인터페이스 구현:
   - `ListSessions() ([]model.Session, error)`
   - `GetStats() (*model.AggregatedStats, error)`
3. `internal/config/config.go`에 provider 설정 추가
4. PR 제출

## 라이선스

기여하신 코드는 [MIT License](LICENSE)로 배포됩니다.

---

# Contributing to taux (English)

Thanks for your interest in contributing to taux! Bug reports, feature requests, and code contributions are all welcome.

## Getting Started

```bash
git clone github.com/glory0216/taux.git
cd taux
make build        # Build
make test         # Test
make lint         # Lint
make fmt          # Format
```

## How to Contribute

### Bug Reports

Please include:
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

### Adding a New Provider

To support AI agents beyond Claude:

1. Create `internal/provider/<name>/`
2. Implement the `provider.Provider` interface
3. Add provider config to `internal/config/config.go`
4. Submit a PR

## License

All contributions are released under the [MIT License](LICENSE).
