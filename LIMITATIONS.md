# Limitations

Nothing in this library is certified. The README's comparison and feature statements describe the code, not a conformance result.

## Known defects

- **macOS: `TestChaosOpenCloseChurn` fails intermittently** with an error opening `/dev/ptmx`. The cause has not been established.

## Not implemented

- Port enumeration and USB metadata.
- Modem control lines (DTR, RTS, CTS and status bits) and hardware or software flow control settings.
- Break signals, buffer flush and drain.
- Per-call deadlines, cancellation, and a `context` API. `Close` is not a cancel: see the [package documentation](serial.go).
- 1.5 stop bits; mark and space parity.
- Protocol logic. The `modbus` package holds presets only.
- Logging and metrics hooks.

## Platform differences

| Behaviour | Linux | macOS | FreeBSD, NetBSD | OpenBSD | Windows |
|---|---|---|---|---|---|
| Baud rates | Table up to 4000000 | Table up to 230400 | Table up to 460800 | Table up to 230400 | Any positive value; the driver decides |
| Wait mechanism | `poll` | `select`; descriptors at or above `FD_SETSIZE` fail with `EINVAL` | `poll` | `poll` | Blocking `ReadFile` |
| `Config.Timeout` on `Read` | Yes | Yes | Yes | Yes | Yes, truncated to milliseconds |
| `Config.Timeout` on `Write` | No | No | No | No | Passed to `COMMTIMEOUTS` |
| Hangup on `Read` | `io.EOF` or the OS error | Same | Same | Same | Not distinguished: zero bytes returns `ErrTimeout` |
| Partial `Write` | Only with an error | Same | Same | Same | Returns the count from `WriteFile`; no retry loop |

- `ErrTimeout` is returned by `Read` only. No backend returns it from `Write`, although [ERRORS.md](ERRORS.md) describes the write case as well.
- **RS-485.** The POSIX backend sends one ioctl with Linux's `TIOCSRS485` request number on every POSIX system; there is no per-OS handling. If the driver refuses it, `Open` fails. The Windows backend ignores `Config.RS485` without an error. Delays are truncated to milliseconds.

## Timing

go-serial gives no real-time guarantee. Timeouts are bounded by the operating system's scheduler and driver. A timeout is not an inter-character timer, so a protocol that needs one (Modbus RTU framing, for example) must implement it above this library, within the accuracy the host allows.

## Not verified

- **No physical hardware.** No test uses a real UART, USB adapter or RS-485 transceiver. Baud rate, parity and stop bits are not observed on a line.
- **RS-485 driver behaviour.** Tests cover validation and the refusal on a PTY, never an accepted ioctl.
- **Windows I/O.** No test opens a port, and CI only cross-compiles the backend.
- **macOS and the BSDs in CI.** CI builds macOS and FreeBSD, and does not run their tests. `make cross` also builds NetBSD and OpenBSD, which CI does not.
- **Independent implementations.** There is no interop suite.
