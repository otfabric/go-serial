# Limitations

Nothing in this library is certified. The README's comparison and feature statements describe the code, not a conformance result.

## Not implemented

- Port enumeration and USB metadata.
- Modem control lines (DTR, RTS, CTS and status bits) and hardware or software flow control settings.
- Break signals, buffer flush and drain.
- Per-call deadlines and a `context` API. `Close` cancels I/O in progress: see the [package documentation](serial.go).
- Line error reporting. A received parity or framing error, or a break, is not reported by `Read`: POSIX delivers such a byte as `0x00`, Windows as the received value. Protocols with a checksum (Modbus RTU CRC) detect the corruption.
- 1.5 stop bits; mark and space parity.
- Protocol logic. The `modbus` package holds presets only.
- Logging and metrics hooks.

## Platform differences

What is tested on each platform, and what is only built, is in the [README](README.md#supported-platforms) and under [Not verified](#not-verified).

| Behaviour | Linux | macOS | FreeBSD, NetBSD | OpenBSD | Windows |
|---|---|---|---|---|---|
| `Config.Address` | A path. `Open` fails unless it is a character device | Same | Same | Same | A device name: `COM3`, `COM3:` or `\\.\COM10`, opened in the device namespace, so `COM10` and higher work without the prefix. Any other form fails with `ErrInvalidConfig` before anything is opened; `Config.UncheckedAddress` passes the address to `CreateFile` as given |
| Exclusive open | No: another process can open the same device | Same | Same | Same | Yes |
| Baud rates | Table up to 4000000 | Table up to 230400 | Table up to 460800 | Table up to 230400 | Any positive value; the driver decides |
| Wait mechanism | `ppoll` | `select`; descriptors at or above `FD_SETSIZE` (1024) fail with `EINVAL` | `poll` | `select`, with the same limit as macOS | Blocking `ReadFile` |
| `Config.Timeout` on `Read` | Yes | Yes | Yes | Yes | Yes, in milliseconds: rounded up, at most 2^32-2 ms (about 49.7 days) |
| `Config.Timeout` on `Write` | No | No | No | No | Passed to `COMMTIMEOUTS`; it applies to each `WriteFile` call |
| Hangup on `Read` | `io.EOF` or the OS error | Same | Same | Same | Not distinguished: zero bytes returns `ErrTimeout` |
| Partial `Write` | Only with an error | Same | Same | Same | Only with an error: `ErrTimeout` when `Config.Timeout` is set, otherwise an error wrapping `io.ErrShortWrite` |
| Concurrent `Read` and `Write` | Independent | Same | Same | Same | One synchronous handle: a `Write` may wait for a pending `Read` |
| `Close` during `Read`/`Write` | Cancels them with `ErrClosed` | Same | Same | Same | Cancels them with `CancelIoEx`; they return `ErrClosed` |
| `Config.VerifySettings` | Reads termios and the RS-485 delays back. A pseudo-terminal sets 8 data bits and no parity whatever it is given, so `Open` fails there for other data bits or a parity | Reads termios back | Same as macOS | Same as macOS | No effect |

- `ErrTimeout` comes from `Read` on every platform and from `Write` on Windows only: the POSIX `Write` has no deadline. After a `Write` that returned `ErrTimeout`, `n` bytes were handed to the driver and the rest were not; the library does not retry.
- **RS-485.** Linux only: `Open` sends `TIOCSRS485` (with the MIPS request number on MIPS) and fails if the driver refuses it. A driver may change the delays it was given; with `Config.VerifySettings`, `Open` reads them back with `TIOCGRS485` and fails when they differ. On macOS and the BSDs `Open` with `Config.RS485.Enabled` fails with `ErrUnsupportedPlatform`. The Windows backend ignores `Config.RS485` without an error. Delays are truncated to milliseconds.
- **Timeouts beyond the wait primitive.** A `Config.Timeout` longer than `select`, `poll` or `ppoll` can take in one call (macOS and OpenBSD: 10^8 s; FreeBSD and NetBSD: 2^31-1 ms; Linux: 2^31-1 s) is waited in several calls, so `Read` still waits the full time.

## Timing

go-serial gives no real-time guarantee. Timeouts are bounded by the operating system's scheduler and driver. A timeout is not an inter-character timer, so a protocol that needs one (Modbus RTU framing, for example) must implement it above this library, within the accuracy the host allows.

## Not verified

- **No physical hardware.** No test uses a real UART, USB adapter or RS-485 transceiver. Baud rate, parity and stop bits are not observed on a line.
- **RS-485 driver behaviour.** Tests cover validation, the refusal on a PTY and the read-back against a replaced ioctl, never an accepted ioctl of a driver.
- **Windows I/O.** No test opens a port: `Open`, the `ReadFile` loop, `Close` and the behaviour of a driver are not exercised. CI runs on `windows-latest` the tests that need no port: the time-out and DCB conversions, the address check against a replaced `CreateFile`, `Write` on a replaced `WriteFile`, and `Close` cancelling a `Read` or `Write` on a replaced `ReadFile`, `WriteFile` and `CancelIoEx`. That `CancelIoEx` ends a `ReadFile` or `WriteFile` of a serial driver on a synchronous handle is taken from the Win32 documentation, not from a run; with a driver that does not cancel, `Close` waits until the call ends by itself: data, a time-out, or for a `Write` without `Config.Timeout`, the last byte sent. That a `WriteFile` which runs out of time reports success with a short count is taken from the Win32 documentation of `COMMTIMEOUTS`, not from a run.
- **The BSDs.** CI builds and vets FreeBSD, NetBSD and OpenBSD and runs none of their tests. FreeBSD has a PTY helper for the tests; NetBSD and OpenBSD have none.
- **OpenBSD.** No test has run on OpenBSD. Its wait uses `select` through the same code as macOS, where the tests run; whether `Open`, `Read` and `Write` work on an OpenBSD system has not been observed.
- **32-bit systems.** The tests run as a linux/386 binary in the nightly workflow only. Other 32-bit targets (linux/arm, windows/386, the 32-bit BSDs) are built at most.
- **Independent implementations.** There is no interop suite.
