# go-serial Releases

## v0.6.0

**Date:** 2026-11-19
**Previous release:** v0.5.0

## Summary

Flow control and modem lines. `Config.FlowControl` sets RTS/CTS or XON/XOFF flow control on every platform and DTR/DSR on Windows; the port `Open` returns implements the new `ModemLines` interface, which sets DTR and RTS and reads CTS, DSR, RI and DCD. The API change is additive: one `Config` field, the `FlowControl` type, the `ModemLines` interface, `ModemStatus` and `ErrLineInUse`. `Port` is unchanged, so code that implements it keeps compiling.

Contributed by @MatthewReed303 in [#18](https://github.com/otfabric/go-serial/pull/18). Thank you.

## Changes

### Added

- `Config.FlowControl` (`FlowNone`, `FlowRTSCTS`, `FlowDTRDSR`, `FlowXONXOFF`) and `ParseFlowControl`. POSIX: `CRTSCTS`, or `IXON` and `IXOFF` with `VSTART` and `VSTOP` set to `XON` and `XOFF`; `FlowDTRDSR` fails `Open` with `ErrUnsupportedPlatform`. Windows: the DCB's handshake flags, or `fOutX` and `fInX` with the driver's XON and XOFF thresholds. `Validate` refuses `FlowRTSCTS` with `RS485.Enabled`, and `Config.VerifySettings` compares the flow control too.
- `ModemLines` (`SetDTR`, `SetRTS`, `ModemStatus`) and `ModemStatus`, implemented by the port `Open` returns: `TIOCMBIS`, `TIOCMBIC` and `TIOCMGET` on POSIX systems, `EscapeCommFunction` and `GetCommModemStatus` on Windows.
- `ErrLineInUse`: `SetRTS` under RTS/CTS flow control or RS-485, `SetDTR` under DTR/DSR flow control.
- `XON` and `XOFF`.
- `ExampleParseFlowControl` and `ExampleModemLines`.

The `ModemLines` methods on a closed port return `ErrClosed`, also for a line that flow control or RS-485 drives. `Close` restores the flow control with the saved termios or DCB, but not the levels `SetDTR` and `SetRTS` set.

### Documentation

- The package documentation (concurrency of the `ModemLines` methods), README (features and the comparison table), API.md, ARCHITECTURE.md (the new files, the `ModemLines` methods under the close gate, what `Close` restores), ERRORS.md, LIMITATIONS.md (flow control and modem lines per platform, and what no test proves on hardware), SECURITY.md and TESTING.md.

### Tests and CI

- Every platform: `FlowControl` parsing, text and validation, and the Windows DCB flags, `EscapeCommFunction` functions and `GetCommModemStatus` bits against the Win32 header values.
- POSIX: the flow-control termios bits and their read-back, `Open` with each flow control over a PTY, XON/XOFF holding output over a PTY until XON, and the modem-line ioctls against a replaced ioctl.
- Windows: the DCB of each flow control, and the modem-line methods of a closed port.
- Chaos: `TestChaosModemLinesDuringClose` (the modem-line methods from several goroutines during `Read`, `Write` and a racing `Close`); random configs and the termios restore now include the flow control.
- Fuzz: `FuzzParseFlowControl`; the flow control in `FuzzTextRoundTrip`, `FuzzConfigValidate` and `FuzzTermiosAgreesWithValidate`.

---

## v0.5.0

**Date:** 2026-11-16
**Previous release:** v0.4.1

## Summary

On Windows `Close` cancels a `Read` or `Write` in progress and `Open` takes a device name only. `ErrTimeout` matches `os.ErrDeadlineExceeded`, the errors of `Close` and of the RS-485 ioctl name the operation, and the new `Config.VerifySettings` makes `Open` on POSIX systems check that the port took the settings. The API change is additive: two `Config` fields.

## Changes

### Breaking

- `ErrTimeout` matches `os.ErrDeadlineExceeded` with `errors.Is` and has a `Timeout()` method that reports true, like the time-outs of `net` and `os`. It is the same variable and does not match `context.DeadlineExceeded`; `errors.Is(err, serial.ErrTimeout)` is unchanged. Who notices: code that tests for `os.ErrDeadlineExceeded` or a `Timeout()` method and handles a match differently from `serial.ErrTimeout`. Restore path: test `serial.ErrTimeout` first.
- The errors of `Close` and of the RS-485 ioctl name the operation: `serial: close: …` (with `restore termios`, `restore comm timeouts` or `restore comm state` for a failed restore) and `serial: enable RS485: …`. `errors.Is` still reaches the cause. Who notices: code that compares the text of these errors. Restore path: match the cause, for example a `syscall.Errno`, with `errors.Is`.
- Windows: `Open` takes a device name only: `COM3`, `COM3:`, `\\.\COM10`, a `\\?\` device interface name, `\\.\Global\NAME` or `\\.\GLOBALROOT\Device\NAME`. It opens the name in the device namespace, so `COM10` and higher work without the prefix. Any other form fails with a `*ConfigError` for `Address` wrapping `ErrInvalidConfig` before anything is opened. Who notices: a program that opens a port through another form, such as a device interface path with a reference string or a link name with a path. Restore path: `Config.UncheckedAddress` passes the address to `CreateFile` as given, as before; or use the port's `COM` name.
- Windows: `Close` cancels a `Read` or `Write` in progress with `CancelIoEx`, again every 10 ms until the call has returned, and only then restores the settings and closes the handle. The call returns `ErrClosed`, and so do `Read` and `Write` after `Close`. Before, `Close` during I/O was not supported, and I/O after `Close` failed with an error of the closed handle. Who notices: code that matched that error, or that relied on a `Close` returning while a call was still in the driver. Restore path: none that keeps a closed handle in use; match `errors.Is(err, serial.ErrClosed)`, which the POSIX backend returns since v0.4.0.

### Added

- `Config.VerifySettings`: on POSIX systems `Open` reads termios back after setting it and fails with a `*ConfigError` wrapping `ErrInvalidConfig` when the port did not take the speed, data bits, stop bits or parity. On Linux with `RS485.Enabled` it also reads the RS-485 settings back (`TIOCGRS485`) and fails when the driver changed a delay. The error names the field and what the port reports; `Open` restores the settings it found and closes the descriptor first. Off by default, so `Open` behaves as before: a Linux pseudo-terminal, for example, sets 8 data bits and no parity whatever it is given. No effect on Windows.
- `Config.UncheckedAddress`: on Windows `Open` passes `Address` to `CreateFile` as given, as before v0.5.0. No effect on POSIX systems.

### Documentation

- The package documentation, README, API.md, ERRORS.md, LIMITATIONS.md, SECURITY.md and ARCHITECTURE.md: `Close` on Windows, the Windows address rule, `Config.VerifySettings`, `ErrTimeout` and `os.ErrDeadlineExceeded`, and the operation in error texts.
- SECURITY.md: 0.5.x is the supported line from its release; 0.4.x until then.
- TESTING.md: the new tests, and Windows tests for rows L1, L3, L4 and L14 of the lifecycle table.

### Tests and CI

- Windows, run by the `windows-latest` job against replaced `ReadFile`, `WriteFile`, `CancelIoEx` and `CreateFile`: `Close` during a `Read` and a `Write`, a call that the first cancel missed, `Close` between two `WriteFile` calls, I/O after `Close`, and the address check before `CreateFile`.
- Every platform: the gate of the Windows `Close` with `testing/synctest`, the Windows address check as a table test and as `FuzzWindowsDeviceName`, and `ErrTimeout` against `os.ErrDeadlineExceeded`, `context.DeadlineExceeded` and the other sentinels.
- POSIX: the comparison of termios as set and as read back, the RS-485 read-back against a replaced ioctl, `Open` with `Config.VerifySettings` over a PTY, and the operation prefix of the `Close` and RS-485 errors.

---

## v0.4.1

**Date:** 2026-11-15
**Previous release:** v0.4.0

## Summary

The Windows `Write` returns an error with a short count, OpenBSD waits for I/O with `select`, and the README states what is tested on each platform. No exported identifier changes.

## Changes

### Fixes

- Windows: `Write` returned a short count with a nil error when `Config.Timeout` passed before all bytes were written. It now returns the count with `ErrTimeout`, as ERRORS.md and the comment on `ErrTimeout` said. A short count with no time-out set comes with an error wrapping `io.ErrShortWrite`. Code that took a nil error as "all bytes sent" now sees the error; code that wrote the rest again after a short count gets `ErrTimeout` instead and decides for itself.
- Windows: a buffer longer than one `WriteFile` call takes (2^32-1 bytes) is written in several calls. Before, `Write` wrote the first part and returned its length with a nil error.
- OpenBSD: `Read`, and a `Write` that had to wait for room, failed with "function not implemented". The wait called `poll` by its system call number, and on OpenBSD Go's `syscall` package carries out such calls for ioctl only. The wait now uses `syscall.Select`, with the code the macOS backend uses. This is cross-compiled and vetted, not run on OpenBSD. A descriptor at or above `FD_SETSIZE` (1024) fails with `EINVAL` there, as on macOS.

### Changed

- POSIX: `Open` checks that the opened path is a character device before it reads or changes a terminal setting. Any other file is refused with an error wrapping `ENOTTY`. Such a path could not be opened as a port before either: it failed one step later, with the error of the termios call. `/dev/null` and other character devices that are not terminals fail at the termios call, as before.

### Documentation

- README: a table with the level of each platform (tested over pseudo-terminals, tested without a port, built and not run) and a list of what is known not to work.
- SECURITY.md: the supported line is 0.4.x; new sections on what the library trusts, its resource use and its known limits.
- ARCHITECTURE.md describes the mutex and the wake pipe of v0.4.0; before, it said that production code holds no locks and that `Close` must not run during I/O.
- TESTING.md: where each group of tests runs, and the lifecycle contract tests (rows L1 to L16) with what applies to `Port`.
- LIMITATIONS.md: the Windows port is opened exclusively and the POSIX ones are not; on Windows, port numbers above 9 need the form `\\.\COM10`; what CI runs on macOS, Windows and the BSDs. The "Known defects" entry is gone: the macOS failure of `TestChaosOpenCloseChurn` was in the test.
- CONTRIBUTING.md lists what `make check` runs.

### Tests and CI

- `TestCloseUnblocksRead` and `TestCloseUnblocksWrite` no longer sleep. They hold the call at an unexported hook and assert that `Close` does not return, and leaves the descriptor open, until the `Read` or `Write` has let go of it.
- `TestChaosOpenCloseChurn` failed on current macOS: the tests opened `/dev/ptmx` from eight goroutines at once. The test helper now opens it from one at a time, and the channel that collects the errors holds them all.
- CI vets every cross-build target, which compiles its test files, and builds NetBSD and OpenBSD too. A `windows-latest` job runs the tests that need no port. The nightly workflow runs the tests as a linux/386 binary.
- The release workflow builds and tests go-modbus against the release commit before it creates the tag. go-canbus is not listed: its SocketCAN tests need a kernel with AF_CAN, which that job does not set up.

---

## v0.4.0

**Date:** 2026-11-14
**Previous release:** v0.3.0

## Summary

`Close` cancels blocked I/O on POSIX systems, long timeouts are honoured on every platform, and the Windows backend returns available data with no timeout set.

## Changes

### Added

- `ErrClosed`, returned on POSIX systems by `Read` and `Write` on a closed port, and by a `Read` or `Write` that was blocked when `Close` ran in another goroutine.

### Changed

- POSIX: `Close` may run concurrently with `Read` and `Write`. It wakes them, waits until they have let go of the descriptor, then restores termios and closes it. Each port holds a pipe for this, so it uses two more descriptors.
- POSIX: `Read` and `Write` after `Close` return `ErrClosed` instead of an `EBADF` error.
- macOS and the BSDs: `Open` with `Config.RS485.Enabled` fails with `ErrUnsupportedPlatform` instead of sending Linux's ioctl number.
- Windows: `Close` is safe to call from several goroutines.

### Fixes

- macOS: `Read` failed at once with "invalid argument" when `Config.Timeout` was above 10^8 seconds, which `select` does not accept. Longer waits are now made in several calls.
- FreeBSD, NetBSD, OpenBSD: a `Config.Timeout` of 2^31 ms or more did not fit `poll`'s timeout argument. It is now clamped per call and waited in several calls.
- Linux: the `ppoll` timeout is clamped so 32-bit systems cannot truncate it.
- Linux on MIPS: RS-485 used the wrong ioctl request number and could not be enabled.
- Windows: with `Timeout == 0`, `Read` waited for the whole buffer to fill. It now returns as soon as data arrives, as on POSIX.
- Windows: a timeout below one millisecond turned into no timeout, and timeouts of 2^32 ms or more wrapped around. They are now rounded up and capped at 2^32-2 ms.
- Windows: `Open` rejects baud rates above 2^32-1 instead of truncating them.
- Windows: the DCB passed to `SetCommState` has its `DCBlength` set; if setting the timeouts fails, `Open` restores the previous DCB before closing.
- Windows: `Read` with an empty buffer returns `0, nil` instead of `ErrTimeout`.
- The test package builds on 32-bit platforms.

### Documentation

- LIMITATIONS.md: on Windows a `Write` may wait for a pending `Read` (one synchronous handle), and `Read` does not report line errors (parity, framing, break).

---

## v0.3.0

**Date:** 2026-11-12
**Previous release:** v0.2.1

## Summary

Requires Go 1.26 (the OT Fabric floor) and rejects out-of-range values that were silently truncated.

## Changes

### Fixes

- `ParseDataBits` and `ParseStopBits` checked the value after converting it to `uint8`, so out-of-range input such as `ParseDataBits(264)` was accepted as 8. They now check the `int` first.
- `Config.Validate` rejects RS-485 RTS delays above the new `MaxRS485Delay` (2^32-1 ms), which the 32-bit kernel field cannot hold; `Open` refuses them too. Before, they wrapped around.
- The poll-based wait rejects a file descriptor outside the `int32` range with `EBADF` instead of truncating it.

### Added

- `MaxRS485Delay`.

### Breaking changes

- `go.mod` now says `go 1.26.0`; dependants need Go 1.26 or later. CI tests Go 1.26 and 1.27, and the cross-build uses the `go.mod` version.

---

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
