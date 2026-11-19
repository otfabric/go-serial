# Errors

| Error | Returned when | Retryable |
|---|---|---|
| `ErrTimeout` | `Read`: no data arrived within `Config.Timeout`. `Write` on Windows: `Config.Timeout` passed before all bytes were written; `n` is the number that were, and the rest of the buffer was not sent. The POSIX `Write` has no deadline and does not return it. A zero timeout means no deadline. It matches `os.ErrDeadlineExceeded` with `errors.Is` and has a `Timeout()` method that reports true; it does not match `context.DeadlineExceeded` | `Read`: yes. `Write`: send the bytes after the first `n` again, if the protocol allows a message to arrive in parts |
| `ErrInvalidConfig` | Config validation failed, e.g. an empty address or unsupported parity. Also from `Open` on Windows for an address that is not a device name (see `Config.Address`), and from `Open` with `Config.VerifySettings` when the port did not take a setting | No |
| `ErrUnsupportedPlatform` | `Open` on an operating system that is not supported, with `Config.RS485.Enabled` on a system without an RS-485 ioctl (macOS, the BSDs), or with `Config.FlowControl` `FlowDTRDSR` on a POSIX system | No |
| `ErrClosed` | `Read` or `Write` on a closed port, or a `Read` or `Write` that was in progress when `Close` ran in another goroutine. `n` may be non-zero for `Write` | No |
| `ErrLineInUse` | `ModemLines.SetRTS` while `Config.FlowControl` is `FlowRTSCTS` or `Config.RS485` is enabled, and `ModemLines.SetDTR` while it is `FlowDTRDSR`: the driver drives that line | No: open the port without that flow control |
| `*ConfigError` | Field-level detail of a validation failure: `Field`, `Value`, `Reason`. `Unwrap` gives the cause. With `Config.VerifySettings`, `Field` names the setting the port did not take and `Reason` gives what the port reports | No |

Errors of the operating system come wrapped with the operation, for example `serial: open …`, `serial: read: …`, `serial: write: …`, `serial: close: …`, `serial: enable RS485: …`, `serial: set DTR: …`, `serial: set RTS: …` or `serial: modem status: …`. `errors.Is` reaches the cause, for example a `syscall.Errno`; match that rather than the text.

Use `errors.Is(err, serial.ErrInvalidConfig)` to detect a validation failure, and `errors.As(err, &cfgErr)` for the details. Prefer those to matching message text.
