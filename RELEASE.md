# go-serial Releases

## v0.1.5

Patch release: stronger automated test coverage and POSIX loopback I/O tests. No API or behavior changes.

### Changes

- **Tests**
  - **`loopback_linux_test.go`**, **`loopback_freebsd_test.go`**, **`loopback_darwin_test.go`**, **`loopback_stub_test.go`**: platform-specific PTY helpers (proper `*_test.go` naming so cross-builds do not compile test code into the library).
  - **`TestReadWrite`** creates two bridged pseudo-terminal pairs via `/dev/ptmx` when available; skips cleanly when PTY creation is not permitted.
  - Optional manual loopback via **`SERIAL_LOOPBACK_PTY1`** and **`SERIAL_LOOPBACK_PTY2`** (e.g. from `socat`).
  - **`serial_test.go`**: additional **`ConfigError.Error()`** coverage.
  - FreeBSD: use **`TIOCGPTN`** only (no **`TIOCSPTLCK`** in Go's FreeBSD syscall package); OpenBSD/NetBSD skip programmatic PTY and rely on env vars.
- **Coverage**
  - **`Makefile`**: **`make coverage`** excludes the **`example/`** demo package and uses **`-coverpkg`** for library-only coverage (aligned with shared CI **`test-exclude-regex`**).
  - Statement coverage for library packages is now ~**75%** (up from ~**43%** before loopback I/O tests).
- **README**
  - Updated **Automated tests** and **Manual loopback testing** sections for PTY loopback and coverage workflow.
  - Removed deprecated **Go Report Card** badge.

### Go version

- Unchanged: requires **Go 1.23+** (see `go.mod`).

---

## v0.1.4

Patch release: open-source license hygiene, README badge normalization, and public repository readiness. No API or behavior changes.

### Changes

- **License**
  - **`LICENSE`**: MIT license at repo root (`Copyright (c) 2026 OT Fabric`).
  - **Go sources**: `// SPDX-License-Identifier: MIT` added to first-party `.go` files (build tags preserved; generated Windows syscall/types files unchanged).
  - **`README.md`**: added a **License** section linking to [LICENSE](LICENSE).
- **README badges**
  - Standardized badge block for public GitHub: Go version, pkg.go.dev, License, CI, Go Report Card, Codecov, Release.
  - Added **Go Reference** (pkg.go.dev) badge using module path `github.com/otfabric/go-serial`.
  - Removed Codecov badge URL token; badge uses public `codecov.io/gh/otfabric/go-serial` links.
- **Repository visibility**
  - Prepared for **public** release: badges and docs assume a public `otfabric/go-serial` repository (CI, pkg.go.dev, Go Report Card, Codecov, and Release badges may need a first successful CI run, release tag, or coverage upload after going public).

### Go version

- Unchanged: requires **Go 1.23+** (see `go.mod`).

---

## v0.1.3

Minor release: raise the minimum supported Go version and refresh CI.

### Changes

- **Go toolchain**
  - **`go.mod`**: `go 1.23` — the module now requires **Go 1.23 or later**. Upgrade your toolchain before building or testing dependents.
- **GitHub Actions**
  - **`ci.yml`** (single workflow): calls reusable **`otfabric/.github/.github/workflows/go-ci.yml@v2`** with **`go-versions`**: `1.23`, `1.24`, `1.25`, `1.26` (JSON array string), and defines an in-repo **`cross-build` job** that runs **`go build ./...`** with **Go 1.23** (`setup-go`) across the `GOOS`/`GOARCH` matrix (`linux/arm` still sets **`GOARM=7`**; **`CGO_ENABLED=0`**). The former standalone **`build.yml`** has been merged into **`ci.yml`**.

### Go version

- Requires **Go 1.23+** (see `go.mod`).

---

## v0.1.2

Patch release: no changes.

### Changes
- None

---

## v0.1.1

Patch release: tooling, tests, CI, and small correctness fixes. No intentional API breaks.

### Changes

- **Module path**: canonical import is `github.com/otfabric/go-serial` (README, badges, docs aligned).
- **CI**
  - **`ci.yml`**: primary test/lint/coverage via reusable `otfabric/.github` workflow.
  - Cross-compilation smoke (`GOOS`/`GOARCH` matrix, `linux/arm` with `GOARM=7`, `CGO_ENABLED=0`) shipped in a separate **`build.yml`** in v0.1.1; that job now lives in **`ci.yml`** as **`cross-build`**.
  - Coverage job waits on compat; example package excluded from coverage profile; static checks include `gofmt`.
- **Tests**
  - `TestReadWrite` skips when the PTY cannot be opened (permission / busy), so local `make check` and CI stay green without loopback hardware.
  - Unsupported-baud tests split: POSIX-only (`serial_posix_test.go`) vs Windows driver-level acceptance (`serial_windows_test.go`).
  - **`types_test.go`**: broad coverage for `types.go` (marshal/unmarshal, `String`, error paths).
- **Code**
  - **`termios_flag_darwin.go` / `termios_flag_posix.go`**: platform `termiosFlag` alias so `newTermios` avoids an unused `termios.Cflag` read (staticcheck SA4006).
  - **errcheck / godot**: explicit `syscall.Close` handling on termios setup failure; deferred `Close()` in tests; comment punctuation.
- **Makefile**: `golangci-lint run ./...` (import paths from `go list` are not valid linter path arguments).

### Go version

- Unchanged: **Go 1.20+**.

---

## v0.1.0 (first release)

Initial public release of `github.com/otfabric/go-serial`, a generic cross-platform Go library for serial (UART) communication over RS-232 and RS-485.

### Highlights

- **Transport API**
  - `Open(*Config) (Port, error)` — validate, normalize, and open a serial port.
  - `Port` implements `io.ReadWriteCloser`; use with any protocol (custom, Modbus RTU, etc.).
  - `DefaultConfig(address)` for generic 9600 8N1.

- **Typed configuration**
  - `Config` with typed fields: `BaudRate`, `DataBits`, `StopBits`, `Parity`, `Timeout`, `RS485`.
  - Constants and `ParseBaudRate`, `ParseParity`, `ParseDataBits`, `ParseStopBits` for CLI/config integration.
  - Zero-value normalization at open (e.g. BaudRate 0 → 9600); `Config.Normalized()` to inspect effective config.
  - Text marshal/unmarshal on config types for config files.

- **Validation and errors**
  - `Validate()` / `ValidateConfig()`; `*ConfigError` with `Field`, `Value`, `Reason` for invalid config.
  - Sentinel errors: `ErrInvalidConfig`, `ErrTimeout`, `ErrUnsupportedPlatform`.
  - `IsUnsupportedBaudRate(err)` for baud-specific failures (POSIX); on Windows, baud is driver-dependent.

- **Modbus RTU preset**
  - `serial/modbus`: `DefaultRTUConfig(address)` (19200 8E1); `DefaultModbusRTUConfig` alias.

- **Platform support**
  - Linux, macOS, Windows, FreeBSD, NetBSD, OpenBSD.
  - Optional RS-485 options where the OS/driver supports them.

- **Tooling and docs**
  - `README.md` with usage, examples, migration from goburrow/serial.
  - `Makefile`: `test`, `test-race`, `coverage`, `verify`, `check`.
  - GitHub Actions: static checks (tidy, gofmt, vet), test matrix (Linux/macOS/Windows + Go 1.20 compat), coverage with example excluded, Codecov.

### Go version

- Requires **Go 1.20+** (see `go.mod`).

### License

- MIT. See [LICENSE](LICENSE).
