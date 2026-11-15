# Testing

## Tiers

| Tier | Command | Prerequisites |
|---|---|---|
| Unit, loopback and chaos | `make test` | None. PTY tests skip if a PTY cannot be created |
| PTY required | `make test-pty` | A PTY: its absence fails the run. CI runs this on Linux and macOS |
| Chaos, long | `make chaos` (`CHAOS_ITERATIONS`, default 10) | A PTY. Nightly |
| Race detector | `make test-race` | As above |
| Coverage | `make coverage`, `make coverage-check` | As above |
| Per-package floor | `make coverage-pkg-check` | As above. `COVER_PKG_MIN` 90; lowest measured on Linux at v0.4.1: 94.6 %, the root package |
| Fuzz | `make fuzz` (`FUZZTIME=1m make fuzz`) | None |
| Benchmarks | `make bench` | The repository has no `Benchmark` functions yet, so this runs nothing |
| Cross-compilation | `make cross` | None. Builds and vets every target of the CI matrix; `go vet` compiles the test files too. Runs no test |
| 32-bit | `GOARCH=386 go test -count=1 ./...` | A linux/amd64 host, which executes 386 binaries. No race detector. Nightly |
| Local gate | `make check` | `staticcheck`, `golangci-lint`, `govulncheck` on `PATH` |

There is no `e2e` target: the library has no separate end-to-end suite and no peer of its own. The PTY tests in the root package exercise `Open`, `Read`, `Write` and `Close` against a real kernel tty layer. There is no interop suite and no hardware test.

## Environment variables

| Variable | Effect |
|---|---|
| `SERIAL_REQUIRE_PTY` | `1` fails PTY tests instead of skipping them |
| `SERIAL_CHAOS_SEED` | Fixes the seed of the chaos tests. Each run logs the seed it used |
| `SERIAL_CHAOS_ITERATIONS` | Multiplies the chaos rounds |
| `SERIAL_LOOPBACK_PTY1`, `SERIAL_LOOPBACK_PTY2` | Device paths of an external pair (for example from `socat`) for `TestReadWrite`, instead of the PTY it creates |

`-short` reduces the chaos rounds.

```bash
SERIAL_CHAOS_SEED=<seed> go test -run TestChaos -v .
SERIAL_CHAOS_ITERATIONS=10 go test -race -run TestChaos .
```

`SERIAL_REQUIRE_PTY=1` turns a missing PTY, or an unusable external loopback pair, into a failure instead of a skip (`make test-pty`, `make chaos`). The remaining skips are not prerequisites of a CI job: `/dev/null` missing, `RLIMIT_NOFILE` too low for the descriptor test, or a kernel that accepts the RS485 ioctl on a PTY.

.github/workflows/nightly.yml runs the chaos tests with more rounds on Linux and macOS, every fuzz target for five minutes (failing inputs are uploaded as an artifact), and all tests as a linux/386 binary with `SERIAL_REQUIRE_PTY=1`.

## What runs where

| Tests | Build constraint | Fault or behaviour |
|---|---|---|
| `TestReadWrite`, `TestReadAfterHangup` | darwin, linux, freebsd, openbsd, netbsd | PTY loopback. `openbsd` and `netbsd` have no programmatic PTY helper, so these skip there; only `TestReadWrite` can use the environment variables above instead |
| `close_posix_test.go` | same | `Close` during `Read` and `Write`, after them, twice and from several goroutines: the [lifecycle contract tests](#lifecycle-contract-tests) |
| `TestChaos*` in `chaos_test.go` | same | Line fragmentation, hangups, random configs, open and close churn, termios restore, write backpressure |
| `TestChaos*` in `chaos_linux_test.go` | linux | Signal storms during blocking reads, descriptors above `FD_SETSIZE` |
| `wait_select_test.go` | darwin, openbsd | Descriptors that do not fit an `FdSet` are refused; the bit arithmetic of the set |
| `wait_poll_bsd_test.go` | freebsd, netbsd | The conversion of a time-out to `poll`'s argument. No job runs it |
| `serial_windows_test.go` | windows | Baud rate check, the conversion of a time-out to `COMMTIMEOUTS`, the size of the DCB, and `Write` on a replaced `WriteFile`: a short count, a count out of range, a buffer written in several calls, an error. No test opens a port |
| `TestWriteChunks` | none | The loop of the Windows `Write`, on every platform: the split at the per-call limit, a short count with and without a time-out, an error on any call |
| `TestOpenBSDSystemCallsByNumber` | none | Reads the source files built for OpenBSD: they make no system call by number other than ioctl, which is all `syscall.Syscall` carries out there. No test runs on OpenBSD |

Where the tests run:

| System | Job | What runs |
|---|---|---|
| Linux (`ubuntu-latest`) | `ci` of [ci.yml](.github/workflows/ci.yml), which is the shared workflow, on Go 1.26 and 1.27; `pty`; `coverage-floor` | All tests: with and without the race detector in `ci`, with it and `SERIAL_REQUIRE_PTY=1` in `pty`, with coverage in `coverage-floor` |
| macOS (`macos-latest`) | `pty` | All tests with the race detector and `SERIAL_REQUIRE_PTY=1` |
| Windows (`windows-latest`) | `windows` | `go test -count=1 ./...`: the tests without a build constraint and `serial_windows_test.go` |
| linux/386 | `test-386` of [nightly.yml](.github/workflows/nightly.yml) | All tests, without the race detector |
| FreeBSD, NetBSD, OpenBSD | none | Nothing. `cross-build` builds and vets them |

The `cross-build` job builds and vets (`go vet ./...`, which compiles the tests) for linux on amd64, arm and arm64, macOS on amd64 and arm64, Windows on amd64 and arm64, FreeBSD on amd64 and arm64, NetBSD on amd64 and OpenBSD on amd64. Run `make test` on a system no job covers to exercise its backend.

## Lifecycle contract tests

The rows are those of the contract test matrix in otfabric/engineering `standards/lifecycle.md`. The library has one long-lived type, `Port`. The tests below run on Linux and macOS; no test opens a port on Windows, where `Close` during I/O is not supported ([LIMITATIONS.md](LIMITATIONS.md#platform-differences)).

| Row | Scenario | Applies | Tests and what they assert |
|---|---|---|---|
| L1 | Close twice | MUST | `TestCloseIdempotent`, `TestIOAfterCloseReturnsErrClosed`: the second `Close` returns nil |
| L2 | Two goroutines call Close | MUST | `TestConcurrentClose`: four goroutines, all return nil. `TestCloseUnblocksRead`, `TestCloseUnblocksWrite`: a second `Close` while the first waits for I/O returns nil and leaves the descriptor open |
| L3 | Close during a blocked read | MUST on POSIX | `TestCloseUnblocksRead`: the `Read` returns `ErrClosed`; until it has unregistered, `Close` has not returned and the descriptor is open; afterwards `Close` returns nil and the port holds no descriptor |
| L4 | Close during a write | MUST on POSIX | `TestCloseUnblocksWrite`: the same for a `Write` that waits for room, which returns a partial count with `ErrClosed` |
| L5 | Close during handshake | n/a | No handshake |
| L6 | Cancel a pending request | n/a | No request and response, and no context API |
| L7 | Response after a time-out | n/a | No correlation of requests and responses |
| L8 | Response after reconnect | n/a | No reconnect |
| L9, L10, L11 | Callbacks | n/a | No callbacks |
| L12 | Queue at capacity | n/a | No queue. The wake pipe takes one byte, written once by `Close` |
| L13 | Concurrent sessions | n/a | No sessions. One reader and one writer on a port: `TestChaosFullDuplexFragmentingRelay` |
| L14 | Failure during construction | MUST | `TestOpenNonexistentDevice`, `TestOpenNotACharacterDevice`, `TestOpenNotATerminal`, `TestOpenRS485Unsupported`: the port is reset and the descriptor closed. `TestChaosOpenCloseChurn`, `TestCloseDoesNotLeakDescriptors`: the count of open descriptors. Not tested: a failure to create the wake pipe |
| L15 | `Shutdown(ctx)` | n/a | No graceful shutdown |
| L16 | Close racing reconnect | n/a | No reconnect |

I/O after `Close`: `TestIOAfterCloseReturnsErrClosed`.

Rows that apply and have no test: L1, L2 and L14 on Windows, where no test opens a port, and L14 for a failure to create the wake pipe.

`TestCloseUnblocksRead` and `TestCloseUnblocksWrite` do not sleep. The POSIX `port` calls an unexported hook, nil outside tests, before each wait for the descriptor and before a call unregisters. The test starts `Close` when the call has reached its wait, holds the call in the hook once `Close` has woken it, and then takes the port's mutex: `Close` gives the mutex up only while it waits for the call, or when it returns, so the state the test reads is one of those two. With the wait removed from `Close` both tests fail.

## Coverage policy

The gate is 90%: `coverage-min` in [ci.yml](.github/workflows/ci.yml) and `COVER_MIN` in the [Makefile](Makefile). Never lower it.

`make coverage` passes `-coverpkg` for all packages but `example`, and writes `coverage.out` and `coverage.txt`. CI excludes the same package with `test-exclude-regex` and sets no `coverpkg`.

| Excluded | Justification |
|---|---|
| `example/` | Demo program; it needs a serial device |

Coverage is measured on the host. A Linux run does not count Windows, macOS or BSD files, because their build constraints exclude them.

## Fuzzing

Targets are in `chaos_fuzz_test.go` and `chaos_fuzz_posix_test.go`, and `make fuzz` discovers them. CI runs each for 10 seconds.

When a fuzz run fails:

1. Go writes the input to `testdata/fuzz/<FuzzName>/`. Commit that file with the fix; `go test` then replays it on every run.
2. Reproduce with `go test -run '<FuzzName>/<file>' .`
3. Fix the code, confirm the replay passes, and add a test by name for the behaviour if it is not obvious from the corpus entry.

## When CI runs

| Event | Workflow | What runs |
|---|---|---|
| Pull request | `pr.yml` | `docs.yml` (docsync, otfabric-policy) always; `ci.yml` unless every changed file is documentation; then `OT Fabric Required Checks` |
| Push to a branch | none | nothing; dispatch `ci.yml` to test a branch without a pull request |
| Push to `main` | `docs.yml` | the light checks; the merged commit was tested as the pull request's merge commit |
| Schedule | `nightly.yml` | the scheduled runs, on `main` |
| Release | `release.yml` | the release workflow tests the release commit again |

Documentation means an added or modified `*.md` file outside `.github/`, `testdata/` and `vendor/`, other than `AGENTS.md` and `CLAUDE.md`. A deleted or renamed document, or any other file, runs everything. `OT Fabric Required Checks` fails unless every job that applies succeeded; its summary lists them.
