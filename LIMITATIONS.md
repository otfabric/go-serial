# Limitations

Nothing in this library is certified. The README's comparison and feature statements describe the code, not a conformance result.

## Known defects

- **macOS: `TestChaosOpenCloseChurn` fails intermittently** with an error opening `/dev/ptmx`. The cause has not been established.

## Not implemented

- Port enumeration and USB metadata.
- Modem control lines (DTR, RTS, CTS and status bits) and hardware or software flow control settings.
- Break signals, buffer flush and drain.
- Per-call deadlines and a `context` API. On POSIX systems `Close` cancels blocked I/O; on Windows it does not: see the [package documentation](serial.go).
- Line error reporting. A received parity or framing error, or a break, is not reported by `Read`: POSIX delivers such a byte as `0x00`, Windows as the received value. Protocols with a checksum (Modbus RTU CRC) detect the corruption.
- 1.5 stop bits; mark and space parity.
- Protocol logic. The `modbus` package holds presets only.
- Logging and metrics hooks.

## Platform differences

| Behaviour | Linux | macOS | FreeBSD, NetBSD | OpenBSD | Windows |
|---|---|---|---|---|---|
| Baud rates | Table up to 4000000 | Table up to 230400 | Table up to 460800 | Table up to 230400 | Any positive value; the driver decides |
| Wait mechanism | `poll` | `select`; descriptors at or above `FD_SETSIZE` fail with `EINVAL` | `poll` | `poll` | Blocking `ReadFile` |
| `Config.Timeout` on `Read` | Yes | Yes | Yes | Yes | Yes, in milliseconds: rounded up, at most 2^32-2 ms (about 49.7 days) |
| `Config.Timeout` on `Write` | No | No | No | No | Passed to `COMMTIMEOUTS` |
| Hangup on `Read` | `io.EOF` or the OS error | Same | Same | Same | Not distinguished: zero bytes returns `ErrTimeout` |
| Partial `Write` | Only with an error | Same | Same | Same | Returns the count from `WriteFile`; no retry loop |
| Concurrent `Read` and `Write` | Independent | Same | Same | Same | One synchronous handle: a `Write` may wait for a pending `Read` |
| `Close` during `Read`/`Write` | Cancels them with `ErrClosed` | Same | Same | Same | Not supported |

- `ErrTimeout` is returned by `Read` only. No backend returns it from `Write`, although [ERRORS.md](ERRORS.md) describes the write case as well.
- **RS-485.** Linux only: `Open` sends `TIOCSRS485` (with the MIPS request number on MIPS) and fails if the driver refuses it. On macOS and the BSDs `Open` with `Config.RS485.Enabled` fails with `ErrUnsupportedPlatform`. The Windows backend ignores `Config.RS485` without an error. Delays are truncated to milliseconds.
- **Timeouts beyond the wait primitive.** A `Config.Timeout` longer than `select`, `poll` or `ppoll` can take in one call (macOS: 10^8 s; BSD: 2^31-1 ms) is waited in several calls, so `Read` still waits the full time.

## Timing

go-serial gives no real-time guarantee. Timeouts are bounded by the operating system's scheduler and driver. A timeout is not an inter-character timer, so a protocol that needs one (Modbus RTU framing, for example) must implement it above this library, within the accuracy the host allows.

## Not verified

- **No physical hardware.** No test uses a real UART, USB adapter or RS-485 transceiver. Baud rate, parity and stop bits are not observed on a line.
- **RS-485 driver behaviour.** Tests cover validation and the refusal on a PTY, never an accepted ioctl.
- **Windows I/O.** No test opens a port, and CI only cross-compiles the backend.
- **macOS and the BSDs in CI.** CI builds macOS and FreeBSD, and does not run their tests. `make cross` also builds NetBSD and OpenBSD, which CI does not.
- **Independent implementations.** There is no interop suite.
