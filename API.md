# API

Module `github.com/otfabric/go-serial`. Symbol-level reference: [pkg.go.dev](https://pkg.go.dev/github.com/otfabric/go-serial) or `go doc -all .`. Examples are in the [README](README.md#usage) and [example_test.go](example_test.go). Errors are in [ERRORS.md](ERRORS.md).

## Packages

| Package | Import path | Contents |
|---|---|---|
| `serial` | `github.com/otfabric/go-serial` | `Open`, `Port`, `Config`, typed settings, validation, errors |
| `modbus` | `github.com/otfabric/go-serial/modbus` | Modbus RTU presets for `serial.Config`. No framing, CRC or protocol logic |

`example/` is a demo program, not an API.

## Lifecycle

1. **Construct a `Config`**: `serial.DefaultConfig(address)` (9600 8N1), `modbus.DefaultRTUConfig(address)` (19200 8E1), or a literal. `Address` is a device path such as `/dev/ttyUSB0`, or a name such as `COM3` on Windows.
2. **Open**: `serial.Open(&cfg)` validates the config, normalizes a copy of it, checks the baud rate for the platform, and opens the device. It returns a `Port`.
3. **Use**: `Port` is `io.ReadWriteCloser`; it has no other methods.
4. **Close**: the caller owns the `Port` and must call `Close`, which restores the saved termios or DCB and timeouts, then closes the descriptor or handle. A second `Close` returns nil. Do not use the `Port` after `Close`.

Concurrency, timeout and `Close` rules are stated once, in the package documentation in [serial.go](serial.go) and in the [README](README.md#ownership-concurrency-and-close). Platform differences are in [LIMITATIONS.md](LIMITATIONS.md).

## `serial`

### Functions

| Function | Purpose |
|---|---|
| `Open(c *Config) (Port, error)` | Validate, normalize and open |
| `ValidateConfig(c *Config) error` | `c.Validate()`, and a `*ConfigError` for a nil `c` |
| `IsUnsupportedBaudRate(err error) bool` | True for the `BaudRate` `ConfigError` that `Open` returns for a rate the platform table lacks |
| `ParseBaudRate(n int)` | Any positive value. Platform support is checked by `Open` |
| `ParseDataBits(n int)` | 5, 6, 7 or 8 |
| `ParseStopBits(n int)` | 1 or 2 |
| `ParseParity(s string)` | `N`, `E`, `O`, case-insensitive; the empty string is `ParityNone` |

### `Config`

| Field | Type | Zero value after normalization | Validation |
|---|---|---|---|
| `Address` | `string` | none | Must not be empty |
| `BaudRate` | `BaudRate` | 9600 | Checked by `Open`, not by `Validate` |
| `DataBits` | `DataBits` | 8 | 5, 6, 7, 8 |
| `StopBits` | `StopBits` | 1 | 1, 2 |
| `Parity` | `Parity` | `ParityNone` | `N`, `E`, `O` |
| `Timeout` | `time.Duration` | `NoTimeout` | Not negative |
| `RS485` | `RS485Config` | disabled | Delays not negative, when enabled |

Methods: `Normalized()` returns the config with the defaults filled in, and `Validate()` returns a `*ConfigError` wrapping `ErrInvalidConfig`.

`RS485Config` has `Enabled`, `DelayRtsBeforeSend`, `DelayRtsAfterSend`, `RtsHighDuringSend`, `RtsHighAfterSend` and `RxDuringTx`.

### Typed settings

`BaudRate`, `DataBits`, `StopBits` and `Parity` have constants (`Baud1200` to `Baud115200`, `DataBits5` to `DataBits8`, `StopBits1`, `StopBits2`, `ParityNone`, `ParityEven`, `ParityOdd`), and implement `String`, `encoding.TextMarshaler` and `encoding.TextUnmarshaler`. A custom rate is `serial.BaudRate(n)`.

## `modbus`

| Symbol | Purpose |
|---|---|
| `DefaultRTUConfig(address string) serial.Config` | 19200 baud, 8 data bits, 1 stop bit, even parity |
| `DefaultModbusRTUConfig` | Variable holding the same function |
