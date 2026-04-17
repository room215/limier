# Repository Guidelines

## Build, Test, and Development Commands
Use Go's standard toolchain:

- `go build ./...` checks that all packages compile.
- `go test ./...` runs the full test suite.
- `go test ./cmd ./internal/...` is a quicker loop when editing CLI or internal packages.
- `go run . version` prints the current CLI version.

Run `gofmt -w .` & `go vet` before submitting changes.

## Testing Guidelines
Tests use Go's built-in `testing` package and live beside the code they cover as `*_test.go`. Favor `t.Parallel()` for isolated tests, use `t.TempDir()` for filesystem fixtures, and stub external tools the way `cmd/run_test.go` stubs `npm` through a temporary `bin/` directory.

Prioritize tests for user-visible and business-critical behavior: verdict classification, dependency materialization, report generation, and other flows that would change what the tool tells a user. Prefer a small number of high-signal integration tests over many narrow tests of internal plumbing.

Do not add tests for thin wrappers, straightforward config loading/parsing, CLI flag wiring, or runner mechanics unless that logic contains business rules, has caused regressions, or is hard to verify through a higher-level test. When behavior changes, add or update the smallest test that protects the product outcome rather than chasing line coverage.
