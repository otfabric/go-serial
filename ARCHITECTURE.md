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
| `serial_posix.go` | darwin, linux, freebsd, openbsd, netbsd | `port`, `Open` path, `Read`, `Write`, `Close`, `newTermios`, RS-485 ioctl |
| `serial_linux.go`, `serial_darwin.go`, `serial_bsd.go`, `serial_openbsd.go` | per OS | Baud rate and character size tables, `tcsetattr`, `tcgetattr` |
| `termios_*.go` | per OS and architecture | Input and output speed fields of `Termios`, Linux MIPS separately |
| `wait_posix.go` | POSIX | `waitFD`: deadline loop that retries `EINTR` |
| `wait_poll*.go` | linux, BSDs | `waitFDOnce` with `poll(2)` |
| `wait_select_darwin.go` | darwin | `waitFDOnce` with `select(2)`, because macOS `poll(2)` does not support ttys |
| `serial_windows.go` | windows | `port` on a `syscall.Handle` |
| `zsyscall_windows.go`, `ztypes_windows.go` | windows | Generated from `syscall_windows.go` and `types_windows.go` by [generate.bat](generate.bat); `zsyscall_windows.go` also carries a hand edit noted in its header |

On an operating system without a backend, `openPort` stays nil and `Open` returns `ErrUnsupportedPlatform`.

## POSIX I/O

The descriptor is opened with `O_NOCTTY|O_NDELAY|O_CLOEXEC`, so it does not block. Blocking is done in Go:

- `Read` waits in `waitFD` until the descriptor is ready or the deadline taken from `Config.Timeout` passes, then reads. `EAGAIN` and `EINTR` return to the wait with the same deadline. A zero-byte read of a ready descriptor is a hangup and returns `io.EOF`.
- `Write` loops until every byte is written, and on `EAGAIN` waits for writability with no deadline.

## Windows I/O

`Read` and `Write` are synchronous `ReadFile` and `WriteFile` calls. `Config.Timeout` becomes `COMMTIMEOUTS` values. A `Read` that returns zero bytes reports `ErrTimeout`.

## Goroutines and ownership

Production code starts no goroutines and holds no locks. A blocking `Read` or `Write` runs on the caller's goroutine. `Port` holds no internal buffer.

`Open` works on a normalized copy of `Config`, so the caller may change its `Config` afterwards. The caller owns the `Port`. The contract for concurrent use is in the [package documentation](serial.go): one `Read` and one `Write` at a time, and no `Close` during either.

## State saved and restored

`open` saves the current termios (POSIX) or DCB and timeouts (Windows), and `Close` restores them before closing. On POSIX, failure to read the current termios is tolerated and nothing is restored. If setup fails part-way, `open` closes the descriptor or handle before returning the error.
