# Contributing to otfabric/go-serial

Thank you for your interest in contributing. This document explains how to get started.

## Development setup

- **Go**: 1.26 or later (see [go.mod](go.mod)).

```sh
git clone https://github.com/otfabric/go-serial.git
cd go-serial
go mod download
```

## Running tests

- **Unit tests**: `make test` (runs `go test -shuffle=on ./...`).
- **Race tests**: `make test-race`. CI runs the tests with the race detector on Ubuntu and macOS.
- **Full checks**: `make check` runs `fmt`, `tidy`, `vet`, `lint` (staticcheck), `lint-ci` (golangci-lint), `vuln` (govulncheck), `test`, `test-race`, `coverage-check`, `coverage-pkg-check` and `docs-check` (docsync). It needs `staticcheck`, `golangci-lint` and `govulncheck` on `PATH`.
- **Other platforms**: `make cross` builds and vets every target of the CI matrix. Code for Windows and the BSDs is not run by `make check` on Linux or macOS; [TESTING.md](TESTING.md#what-runs-where) says which job runs what.

Tests do not require a serial device. The tests that need a pseudo-terminal skip where none can be created, and fail instead with `SERIAL_REQUIRE_PTY=1` (`make test-pty`).

## Code style and formatting

- Format code: run `gofmt -w -s .` or use your editor’s format-on-save.
- CI enforces formatting through golangci-lint, which has `gofmt` and `goimports` enabled ([.golangci.yml](.golangci.yml)).
- Lint: `go vet ./...`, `staticcheck ./...` and `golangci-lint run ./...` are part of `make check`, and CI runs `go vet` and golangci-lint on every pull request. Do not add lint suppressions.

Run `make check` before submitting a PR.

## Submitting changes

1. Open an issue or pick an existing one to discuss the change.
2. Fork the repo, create a branch, and make your changes.
3. Add or update tests as needed.
4. Run `make check`.
5. Open a pull request with a clear description and reference to the issue.

## Error handling

Prefer sentinel errors and wrap with `%w` so callers can use `errors.Is` / `errors.As`. Use `*ConfigError` with `Field`, `Value`, and `Reason` for config validation failures.

## Documentation

When you change **public API** (exported functions, types, or parameters), update:

- **Package and symbol doc comments** in the relevant `.go` files.
- **[README.md](README.md)** if it references the changed API or examples.
- **[ERRORS.md](ERRORS.md)** for an exported error, and **[RELEASE.md](RELEASE.md)** for a change a user can see.
- **[docs/api/](docs/api/README.md)**, which is generated: `go run github.com/otfabric/docsync@<DOCSYNC_VERSION of the Makefile> api .`

Keep doc comments in sync with behavior. A change of behaviour comes with a test that fails without it; a change to `Close`, `Read` or `Write` keeps the [lifecycle contract tests](TESTING.md#lifecycle-contract-tests) and their table current.
