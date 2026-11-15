# Errors

| Error | Returned when | Retryable |
|---|---|---|
| `ErrTimeout` | `Read`: no data arrived within `Config.Timeout`. `Write` on Windows: `Config.Timeout` passed before all bytes were written; `n` is the number that were, and the rest of the buffer was not sent. The POSIX `Write` has no deadline and does not return it. A zero timeout means no deadline | `Read`: yes. `Write`: send the bytes after the first `n` again, if the protocol allows a message to arrive in parts |
| `ErrInvalidConfig` | Config validation failed, e.g. an empty address or unsupported parity | No |
| `ErrUnsupportedPlatform` | `Open` on an operating system that is not supported, or with `Config.RS485.Enabled` on a system without an RS-485 ioctl (macOS, the BSDs) | No |
| `ErrClosed` | POSIX: `Read` or `Write` on a closed port, or a blocked `Read` or `Write` when `Close` ran in another goroutine. `n` may be non-zero for `Write` | No |
| `*ConfigError` | Field-level detail of a validation failure: `Field`, `Value`, `Reason`. `Unwrap` gives the cause | No |

Use `errors.Is(err, serial.ErrInvalidConfig)` to detect a validation failure, and `errors.As(err, &cfgErr)` for the details. Prefer those to matching message text.
