# Repository Guidelines

## Project Structure & Module Organization
`cmd/webhook/main.go` is the canonical CLI entrypoint; root `webhook.go` preserves `go build` / `go run .` compatibility. Application startup, CLI flags, configuration, Admin, and platform-specific behavior live in `internal/app/`. Core hook parsing, rule evaluation, and request mapping live in `internal/hook/`. HTTP logging and request helpers live in `internal/middleware/`, and PID file handling lives in `internal/pidfile/`. Integration fixtures live in `internal/app/testdata/` (`hookecho.go`, `hooks.*.tmpl`). Chinese user-facing docs are indexed in `docs/README.md`; runnable demos and sample configs are in `examples/`. Dependencies use `go.mod` and `go.sum`; `vendor/` is optional and ignored.

## Build, Test, and Development Commands
- `go build` builds the local `webhook` binary.
- `go test ./...` runs all unit and integration tests; use this as the default verification command.
- `make build` builds the binary under `build/` from `./cmd/webhook`.
- `make test` runs the full test suite without upgrading dependencies.
- `make demo` builds the server and safe demo command.
- `go mod download` downloads locked dependencies; `go mod tidy` maintains module metadata.
- `make release`, `make release-linux`, `make release-windows` create release archives under `build/`.

For manual local runs, follow the README pattern: `./webhook -hooks hooks.json -verbose`.

## Coding Style & Naming Conventions
Format all Go code with `gofmt`. Follow standard Go naming: exported identifiers use `CamelCase`, unexported identifiers use `camelCase`, and tests use `TestXxx`. Keep OS-specific behavior in suffixed files such as `signals_windows.go` or `droppriv_unix.go`. Prefer small functions and table-driven tests for parsing and rule logic.

## Testing Guidelines
Tests use Go's built-in `testing` package. Keep unit tests beside the package they cover in `*_test.go` files. Add server-level behavior checks to `internal/app/webhook_test.go`; it compiles `internal/app/testdata/hookecho.go` and exercises both JSON and YAML hook templates. No explicit coverage threshold is enforced, but each behavior change should add or update tests.

## Commit & Pull Request Guidelines
Recent commits favor short, imperative subjects such as `Update README.md`, `Fix broken link...`, and occasional prefixes like `feat: add ...`. Keep commits focused and reference issues or PR numbers when relevant. Pull requests should explain what changed, why it matters, how to verify it, and include tests. Smaller PRs review faster; update `docs/` or example hook files when flags, config fields, or behavior change.

## Security & Configuration Tips
Do not commit real secrets or production hook definitions. Start from `examples/config/` or the runnable demos in `examples/`. If you change dependencies, keep `go.mod` and `go.sum` in sync; generate `vendor/` with `go mod vendor` only for an explicit offline distribution.
