# Contributing to otfabric/go-serial

Thank you for your interest in contributing. This document explains how to get started.

## Development setup

- **Go**: 1.20 or later (see [go.mod](go.mod)).

```sh
git clone https://github.com/otfabric/go-serial.git
cd go-serial
go mod download
```

## Running tests

- **Unit tests**: `make test` (runs `go test -shuffle=on ./...`).
- **Race tests**: `make test-race` or rely on CI, which runs race tests on Ubuntu.
- **Full checks**: `make check` runs fmt, tidy, vet, test, test-race, and coverage.

Tests do not require a serial device; the read/write test skips when no loopback is available.

## Code style and formatting

- Format code: run `gofmt -w -s .` or use your editor’s format-on-save.
- CI enforces formatting: `test -z "$(gofmt -l .)"` in the Static checks job.
- Lint: `go vet ./...` is required; `golangci-lint run` is optional (see [Makefile](Makefile)).

Run `make check` (or at least `make test` and `go vet ./...`) before submitting a PR.

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

Keep doc comments in sync with behavior.
