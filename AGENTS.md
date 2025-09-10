# Repository Guidelines

## Project Structure & Module Organization
- Root Go module: plugin sources such as `plugin.go`, `config.go`, `rpc.go`, `metrics.go`.
- Packages: `api/` (public interfaces), `aggregatedpool/`, `canceller/`, `dataconverter/`, `queue/`, `registry/`, `internal/` (internal helpers).
- Tests: `tests/` is a separate Go module with integration tests in `tests/general/`, helper code in `tests/helpers/`, configs in `tests/configs/`, TLS fixtures in `tests/env/`, and PHP worker fixtures in `tests/php_test_files/`.

## Build, Test, and Development Commands
- Build: `go build ./...` — compile all packages.
- Lint/format: `golangci-lint run` (uses `.golangci.yml`), and `go fmt ./...` if needed.
- Vet: `go vet ./...` — basic static checks.
- Run integration tests:
  - Start Temporal locally (e.g., `temporal server start-dev` or docker on `127.0.0.1:7233`).
  - Install PHP deps: `cd tests/php_test_files && composer install`.
  - From repo root: `go test ./tests/general -v`.
  - Example RR config used by tests: `tests/configs/.rr-proto.yaml`.

## Coding Style & Naming Conventions
- Language: Go 1.25+. Use `gofmt`/`goimports` (enforced via golangci-lint formatters).
- Indentation: tabs; line length target ~120 (see `.golangci.yml`).
- Naming: packages lower_snakecase (prefer short, meaningful), exported identifiers `CamelCase`, unexported `camelCase`.
- Avoid globals; follow linter guidance (e.g., `gochecknoglobals`, `errorlint`, `gosec`).

## Testing Guidelines
- Frameworks: Go `testing` with `testify` assertions.
- Scope: Integration tests exercise the plugin against a running Temporal server and a PHP worker launched by RoadRunner.
- Naming: `_test.go` files under `tests/general/`; test functions `TestXxx`.
- Coverage: run `go test -cover ./...` where practical; note integration focus.

## Commit & Pull Request Guidelines
- Commit style: Conventional Commits preferred (e.g., `feat: ...`, `fix: ...`, `chore(deps): ... (#123)`).
- PRs must include:
  - Clear description, context, and linked issues.
  - Notes on behavior changes or config updates (`tests/configs/*.yaml`).
  - Evidence: logs, before/after, or test output when relevant.
- CI: ensure `golangci-lint run` and `go build ./...` pass; run `go test ./tests/general` when integration changes.

## Security & Config Tips
- Do not commit secrets. TLS certs under `tests/env/temporal_tls/` are test-only.
- Network settings (e.g., `NO_PROXY`) are respected by the plugin; document any env var changes in PRs.
