# Testing

## Tiers

| Tier | Command | Prerequisites |
|---|---|---|
| Unit, loopback and chaos | `make test` | None. PTY tests skip if a PTY cannot be created |
| PTY required | `make test-pty` | A PTY: its absence fails the run. CI runs this on Linux and macOS |
| Chaos, long | `make chaos` (`CHAOS_ITERATIONS`, default 10) | A PTY. Nightly |
| Race detector | `make test-race` | As above |
| Coverage | `make coverage`, `make coverage-check` | As above |
| Fuzz | `make fuzz` (`FUZZTIME=1m make fuzz`) | None |
| Benchmarks | `make bench` | The repository has no `Benchmark` functions yet, so this runs nothing |
| Cross-compilation | `make cross` | None. Builds only; runs no test |
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

.github/workflows/nightly.yml runs the chaos tests with more rounds on Linux and macOS and every fuzz target for five minutes; failing inputs are uploaded as an artifact.

## What runs where

| Tests | Build constraint | Fault or behaviour |
|---|---|---|
| `TestReadWrite`, `TestReadAfterHangup` | darwin, linux, freebsd, openbsd, netbsd | PTY loopback. `openbsd` and `netbsd` have no programmatic PTY helper, so these skip there; only `TestReadWrite` can use the environment variables above instead |
| `TestChaos*` in `chaos_test.go` | same | Line fragmentation, hangups, random configs, open and close churn, termios restore, write backpressure |
| `TestChaos*` in `chaos_linux_test.go` | linux | Signal storms during blocking reads, descriptors above `FD_SETSIZE` |
| `serial_windows_test.go` | windows | Baud rate check only; the Windows I/O path has no test |

The test jobs of the shared CI workflow run on `ubuntu-latest` only. The `cross-build` job of [ci.yml](.github/workflows/ci.yml) compiles for macOS, Windows, FreeBSD and Linux on arm and arm64, without running tests. Run `make test` on those systems to exercise their backends.

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
