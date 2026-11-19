# Architecture

go-serial is one package of platform-independent configuration above build-tagged operating system backends. It uses the standard library only, without cgo.

## Layers

```text
example (demo)      modbus (presets)      callers such as go-modbus
      |                    |                        |
      +--------------------+------------------------+
                           |
                 serial: Open, Port, Config
                 serial.go  config.go  types.go  errors.go
                           |
                 openPort, isBaudRateSupported
                 (function variables set by a backend's init)
                           |
        +------------------+-------------------+
        |                                      |
   POSIX backend                          Windows backend
   serial_posix.go                        serial_windows.go
   termios, poll or select                DCB, COMMTIMEOUTS, ReadFile, WriteFile
```

`modbus` and `example` import `serial`. `serial` imports no other package of this module.

## Backends

| File | Build constraint | Contents |
|---|---|---|
| `serial_posix.go` | darwin, linux, freebsd, openbsd, netbsd | `port`, `Open` path, `Read`, `Write`, `Close`, `newTermios` (including the flow-control bits), RS-485 ioctl |
| `modem_posix.go` | darwin, linux, freebsd, openbsd, netbsd | `ModemLines` of the POSIX `port`: `TIOCMBIS`, `TIOCMBIC`, `TIOCMGET` through the `modemIoctl` variable, which tests replace |
| `flow_linux.go`, `flow_ccts.go`, `flow_crtscts.go` | linux; darwin, freebsd; netbsd, openbsd | The `CRTSCTS` value of each system, which the `syscall` package does not export |
| `serial_linux.go`, `serial_darwin.go`, `serial_bsd.go`, `serial_openbsd.go` | per OS | Baud rate and character size tables, `tcsetattr`, `tcgetattr` |
| `termios_*.go` | per OS and architecture | Input and output speed fields of `Termios`, Linux MIPS separately |
| `wait_posix.go` | POSIX | `waitFD`: deadline loop that retries `EINTR` |
| `wait_poll*.go` | linux, freebsd, netbsd | `waitFDOnce` with `ppoll(2)` on Linux and `poll(2)` on FreeBSD and NetBSD |
| `wait_select.go` | darwin, openbsd | `waitFDOnce` with `select(2)`: macOS `poll(2)` does not support ttys, and on OpenBSD the `syscall` package has `Select` but no poll, and carries out `syscall.Syscall` for ioctl only |
| `serial_windows.go` | windows | `port` on a `syscall.Handle` |
| `write_chunks.go` | none | The loop of the Windows `Write`. It has no build constraint so that its tests run on every platform; the POSIX backend does not call it |
| `io_gate.go` | none | `ioGate`, the count of active calls of the Windows `port` and the cancel loop of its `Close`. No build constraint, for the same reason |
| `device_name.go` | none | `windowsDeviceName`, the address check of the Windows `Open`. No build constraint, for the same reason |
| `flow.go` | none | `FlowControl`, `ModemLines`, `ModemStatus`, and `lineInUse`, the check both backends make before `SetRTS` and `SetDTR` |
| `commflags.go` | none | The DCB bit field, `EscapeCommFunction` functions and `GetCommModemStatus` bits of the Windows backend. No build constraint, so that the mapping is tested on every platform |
| `zsyscall_windows.go`, `ztypes_windows.go` | windows | Generated from `syscall_windows.go` and `types_windows.go` by [generate.bat](generate.bat); `zsyscall_windows.go` also carries a hand edit noted in its header |

On an operating system without a backend, `openPort` stays nil and `Open` returns `ErrUnsupportedPlatform`.

## POSIX I/O

The descriptor is opened with `O_NOCTTY|O_NDELAY|O_CLOEXEC`, so it does not block. `open` checks with `fstat` that it is a character device before it reads or sets termios. Blocking is done in Go:

- `Read` waits in `waitFD` until the descriptor is ready or the deadline taken from `Config.Timeout` passes, then reads. `EAGAIN` and `EINTR` return to the wait with the same deadline. A zero-byte read of a ready descriptor is a hangup and returns `io.EOF`.
- `Write` loops until every byte is written, and on `EAGAIN` waits for writability with no deadline.
- Every wait also watches the read end of a pipe that belongs to the port. `Close` writes one byte to it, and a `Read` or `Write` that sees it returns `ErrClosed`.

## Windows I/O

`Read` and `Write` are synchronous `ReadFile` and `WriteFile` calls. `Config.Timeout` becomes `COMMTIMEOUTS` values. A `ReadFile` that returns zero bytes reports `ErrTimeout`; with no time-out set, `Read` calls `ReadFile` again after its longest wait (about 49.7 days).

`Write` runs the loop in `write_chunks.go`: one `WriteFile` call for each piece of at most 2^32-1 bytes. A call that writes fewer bytes than it was given ends the loop with the count and an error, `ErrTimeout` when `Config.Timeout` is set.

`Close` cancels both calls with `CancelIoEx(handle, nil)`, which ends the synchronous I/O that other threads have pending on the handle; the call then returns `ErrClosed`. A call can register just after a cancel and only then enter `ReadFile` or `WriteFile`, and a `Write` can wait for a `Read` on the synchronous handle and start once that `Read` was cancelled, so `Close` cancels again every 10 ms until no call is active.

`Open` passes the address to `CreateFile` only when it is a device name, which it opens under `\\.\`; `windowsDeviceName` defines the forms. `Config.UncheckedAddress` skips the check.

## Goroutines and ownership

Production code starts no goroutines. A blocking `Read` or `Write` runs on the caller's goroutine. `Port` holds no internal buffer.

Each port has one mutex, and nothing is called while it is held except the system calls of `Close`:

- POSIX: the mutex guards `closed`, the count of active calls and the wake pipe. `Read`, `Write` and the `ModemLines` methods register under it (`begin`), use the descriptor without it, and unregister (`end`). `Close` sets `closed`, writes to the wake pipe, waits on a condition variable until no call is active, and only then restores termios and closes the descriptor and the pipe. So a descriptor number is never used after it was closed. A second `Close` returns nil at once, also while the first one waits.
- Windows: the mutex of `ioGate` guards `closed` and the count of active calls. `Read`, `Write` and the `ModemLines` methods register under it (`begin`), use the handle without it, and unregister (`end`). `Close` sets `closed`, cancels until no call is active, and only then restores the DCB and timeouts and closes the handle. A second `Close` returns nil at once, also while the first one waits.

`Open` works on a normalized copy of `Config`, so the caller may change its `Config` afterwards. The caller owns the `Port`. The contract for concurrent use is in the [package documentation](serial.go): one `Read` and one `Write` at a time, and the `ModemLines` methods from any goroutine. `Close` may run during any of them: it cancels a `Read` or `Write` and waits for every call to return.

The POSIX `port` has an unexported `hook` field for tests, nil otherwise: `Read` and `Write` call it before each wait and before they unregister, so a test can hold a call at that point without sleeping ([TESTING.md](TESTING.md#lifecycle-contract-tests)).

## State saved and restored

`open` saves the current termios (POSIX) or DCB and timeouts (Windows), and `Close` restores them before closing. On POSIX, failure to read the current termios is tolerated and nothing is restored. With `Config.VerifySettings`, POSIX `open` reads termios back after setting it and compares the speed, character size, stop bits, parity and flow control, and on Linux reads the RS-485 settings back with `TIOCGRS485` after `TIOCSRS485` and compares the two delays; a difference fails `open` with a `*ConfigError`, after it restored the saved termios and closed the descriptor. If setup fails part-way, `open` closes the descriptor or handle before returning the error.

The levels that `SetDTR` and `SetRTS` set are not saved or restored. On Windows, restoring the saved DCB sets DTR and RTS as its `fDtrControl` and `fRtsControl` say; on POSIX the lines stay as the driver leaves them when the descriptor closes.
