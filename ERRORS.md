# Errors

| Error | Returned when | Retryable |
|---|---|---|
| `ErrTimeout` | A read or write did not complete before the deadline. `Config.Timeout` influences this and semantics are platform-dependent: on most platforms a read that times out with no data returns it; write behaviour may differ. A zero timeout means no deadline | Yes |
| `ErrInvalidConfig` | Config validation failed, e.g. an empty address or unsupported parity | No |
| `ErrUnsupportedPlatform` | `Open` on an operating system that is not supported | No |
| `*ConfigError` | Field-level detail of a validation failure: `Field`, `Value`, `Reason`. `Unwrap` gives the cause | No |

Use `errors.Is(err, serial.ErrInvalidConfig)` to detect a validation failure, and `errors.As(err, &cfgErr)` for the details. Prefer those to matching message text.
