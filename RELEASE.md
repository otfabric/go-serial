# go-serial Releases

## v0.2.1

**Date:** 2026-10-31
**Previous release:** v0.2.0

### Changed

- Policy 1.2: build output in `bin/` and `release/`, a per-package coverage floor, and more tests and CI. No change in behaviour.

---

## v0.2.0

**Date:** 2026-10-30
**Previous release:** v0.1.8

## Summary

Fixes the serial mode on macOS and the BSDs.

## Changes

### Fixes

- **Baud rate no longer changes the mode on macOS and the BSDs.** `Open` ORed
  the speed constant into `Cflag`, where only Linux has a speed field. There
  the constant is the numeric rate: at 9600 baud `Cflag` had `CSTOPB` (two
  stop bits) and `PARODD` set, and at 1200 baud `CSTOPB`. The speed now goes
  in `Ispeed` and `Ospeed` only. Tested on macOS; the BSDs are built, not run.

### Breaking changes

- None.

---

## v0.1.8

**Date:** 2026-10-29
**Previous release:** v0.1.7

## Summary

Patch release: two POSIX I/O bugs found by a new chaos test suite are fixed,
the module is `go vet`/lint clean on every supported platform (NetBSD now
compiles), and library test coverage rises from 77% to 96%.

## Changes

### Fixes

- **`Read` on file descriptors ≥ 1024.** `Read` waited with `select(2)` and a
  fixed 1024-entry `fd_set`. In a busy process the port can land above that
  limit; `Read` then returned a spurious `io.EOF` at once and the kernel
  accessed memory past the set. Linux and the BSDs now wait with
  `poll`/`ppoll`, which has no descriptor limit. macOS keeps `select` (its
  `poll` does not support ttys) and rejects such descriptors with an error.
- **`Write` under backpressure.** The descriptor is non-blocking, so once
  the tty output buffer filled, `Write` returned a short count with a nil
  error, or `n == -1` with `EAGAIN` — silent data loss. `Write` now waits
  until the driver accepts more data and returns only when every byte is
  written or an error occurs.
- **Timeouts under signals.** Interrupted waits (`EINTR`) retry against one
  deadline, so signals no longer shorten or extend a `Read` timeout.
  Spurious `EAGAIN` wakeups in `Read` are retried.
- **I/O after `Close`** fails with `EBADF` instead of blocking.
- **NetBSD** builds again (`FdSet` field and termios speed types).
- **Windows:** `CloseHandle` result checked on failed open; a port name
  containing NUL returns an error instead of panicking; `SyscallN` replaces
  the deprecated `Syscall`.

### Tests

- POSIX error-path tests: termios options, failed opens, close, read and
  write errors. Coverage 77.1% → 96.1%.
- Chaos suite (`TestChaos*`), seeded and reproducible via
  `SERIAL_CHAOS_SEED`, scalable via `SERIAL_CHAOS_ITERATIONS`: fragmenting
  full-duplex relay, random hangups, signal storms during reads, ports above
  `FD_SETSIZE`, write backpressure, random configs, concurrent open/close
  churn with an fd-leak check, termios restore.
- Fuzz targets for parsers, text round-trips, `Validate`, and agreement
  between `Validate` and the termios layer.
- `TestReadWrite` moved to the POSIX-only test file so Windows test builds
  compile.

### Unchanged

- The API. POSIX `Write` still has no timeout; it now blocks until all data
  is written, as documented.

---

## v0.1.7

**Date:** 2026-10-28
**Previous release:** v0.1.6

## Summary

Patch release: POSIX `Read` now honours the `io.Reader` contract when a
read fails or the device goes away. Found while building go-canbus's slcan
transport, which drives USB CAN adapters through this package.

## Changes

### Fixes

- **`Read` never returns a negative count.** A failing `read(2)` used to
  return `n == -1` with the error; callers doing `buf[:n]` panicked. It now
  returns `n == 0`.
- **`Read` reports `io.EOF` after a hangup.** When `select(2)` reports the
  device readable but `read(2)` returns no data, the other end hung up (an
  unplugged USB adapter, a closed pseudo-terminal). `Read` used to return
  `(0, nil)` forever; it now returns `io.EOF`.

### Tests

- **`TestReadAfterHangup`** closes the pseudo-terminal's other end and checks
  that `Read` ends with an error and `n == 0`. It fails without the fix.

### Unchanged

- Windows behaviour, the API and timeouts.

---

## v0.1.6

**Date:** 2026-07-30
**Previous release:** v0.1.5

## Summary

Patch release: document Port ownership, concurrency, timeouts, and Close
semantics; harden the Makefile. No API or behaviour changes.

## Changes

### Documentation

- **Package docs (`serial.go`)** — Ownership after `Open`, one-reader/one-writer concurrency, timeout platform differences, Close vs blocked I/O.
- **README** — New **Ownership, concurrency, and Close** section; **Project structure** summary; TOC updated.

### Build / tooling

- **Makefile `vuln`** — `govulncheck ./...`, included in `make check`.
- **Makefile** — exports `GOWORK=off` so a parent `go.work` that omits this module does not break checks.

### Unchanged

- No API, wire, or I/O behaviour changes. Concurrent Close with I/O remains unsupported (now documented).

---

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

## v0.1.0

First release.

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
