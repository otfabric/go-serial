# API

Module `github.com/otfabric/go-serial`. Symbol-level reference: [pkg.go.dev](https://pkg.go.dev/github.com/otfabric/go-serial) or `go doc -all .`. Examples are in the [README](README.md#usage) and [example_test.go](example_test.go). Errors are in [ERRORS.md](ERRORS.md).

## Packages

| Package | Import path | Contents |
|---|---|---|
| `serial` | `github.com/otfabric/go-serial` | `Open`, `Port`, `Config`, typed settings, validation, errors |
| `modbus` | `github.com/otfabric/go-serial/modbus` | Modbus RTU presets for `serial.Config`. No framing, CRC or protocol logic |

`example/` is a demo program, not an API.

## Lifecycle

1. **Construct a `Config`**: `serial.DefaultConfig(address)` (9600 8N1), `modbus.DefaultRTUConfig(address)` (19200 8E1), or a literal. `Address` is a device path such as `/dev/ttyUSB0`, or a device name such as `COM3` or `\\.\COM10` on Windows.
2. **Open**: `serial.Open(&cfg)` validates the config, normalizes a copy of it, checks the baud rate for the platform, and opens the device. On POSIX systems the path must be a character device: for any other file `Open` fails with an error wrapping `ENOTTY`, before it reads or changes a terminal setting. On Windows the address must be a device name (see `Config.Address`), unless `Config.UncheckedAddress` is set. With `Config.VerifySettings`, POSIX `Open` reads the settings back and fails with a `*ConfigError` when the port did not take one. It returns a `Port`.
3. **Use**: `Port` is `io.ReadWriteCloser`. The port `Open` returns also implements `ModemLines` (see below). `Write` returns `n < len(b)` only with an error; on Windows that error is `ErrTimeout` when `Config.Timeout` passed first.
4. **Close**: the caller owns the `Port` and must call `Close`, which restores the saved termios or DCB and timeouts, then closes the descriptor or handle. A second `Close` returns nil. `Close` cancels a `Read` or `Write` in progress, which returns `ErrClosed`, and waits until it has returned; `Read` and `Write` after `Close` return `ErrClosed`.

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
| `ParseFlowControl(s string)` | `none`, `rts-cts`, `dtr-dsr`, `xon-xoff`, case-insensitive; the empty string is `FlowNone` |

### `Config`

| Field | Type | Zero value after normalization | Validation |
|---|---|---|---|
| `Address` | `string` | none | Must not be empty. POSIX: `Open` requires a character device. Windows: `Open` requires a device name, such as `COM3`, `COM3:` or `\\.\COM10` |
| `BaudRate` | `BaudRate` | 9600 | Checked by `Open`, not by `Validate` |
| `DataBits` | `DataBits` | 8 | 5, 6, 7, 8 |
| `StopBits` | `StopBits` | 1 | 1, 2 |
| `Parity` | `Parity` | `ParityNone` | `N`, `E`, `O` |
| `Timeout` | `time.Duration` | `NoTimeout` | Not negative. Applies to `Read`, and on Windows to `Write` |
| `FlowControl` | `FlowControl` | `FlowNone` | `none`, `rts-cts`, `dtr-dsr`, `xon-xoff`; not `rts-cts` with RS-485 enabled. `dtr-dsr` is Windows only: elsewhere `Open` fails with `ErrUnsupportedPlatform` |
| `RS485` | `RS485Config` | disabled | Delays from 0 to `MaxRS485Delay`, when enabled |
| `VerifySettings` | `bool` | false | POSIX: `Open` reads the settings back and fails when the port did not take the speed, data bits, stop bits, parity, flow control or, with RS-485, a delay. No effect on Windows |
| `UncheckedAddress` | `bool` | false | Windows: `Open` passes `Address` to `CreateFile` as given, as before v0.5.0. No effect on POSIX |

Methods: `Normalized()` returns the config with the defaults filled in, and `Validate()` returns a `*ConfigError` wrapping `ErrInvalidConfig`.

`RS485Config` has `Enabled`, `DelayRtsBeforeSend`, `DelayRtsAfterSend`, `RtsHighDuringSend`, `RtsHighAfterSend` and `RxDuringTx`. `MaxRS485Delay` (2^32-1 ms) is the largest delay `Validate` accepts; the kernel holds the delays as 32-bit millisecond counts.

### Typed settings

`BaudRate`, `DataBits`, `StopBits` and `Parity` have constants (`Baud1200` to `Baud115200`, `DataBits5` to `DataBits8`, `StopBits1`, `StopBits2`, `ParityNone`, `ParityEven`, `ParityOdd`), and implement `String`, `encoding.TextMarshaler` and `encoding.TextUnmarshaler`. A custom rate is `serial.BaudRate(n)`.

### Flow control and modem lines

`FlowControl` has the constants `FlowNone`, `FlowRTSCTS`, `FlowDTRDSR` and `FlowXONXOFF`, and implements `String`, `encoding.TextMarshaler` and `encoding.TextUnmarshaler`. `XON` (0x11) and `XOFF` (0x13) are the characters of software flow control.

| Flow control | POSIX | Windows |
|---|---|---|
| `FlowRTSCTS` | `CRTSCTS` | `fOutxCtsFlow`, `RTS_CONTROL_HANDSHAKE` |
| `FlowDTRDSR` | `ErrUnsupportedPlatform` | `fOutxDsrFlow`, `DTR_CONTROL_HANDSHAKE` |
| `FlowXONXOFF` | `IXON`, `IXOFF`, `VSTART` = XON, `VSTOP` = XOFF | `fOutX`, `fInX`, `XonChar`, `XoffChar`; the driver's `XonLim` and `XoffLim`, or 2048 and 512 |

`ModemLines` is implemented by the port `Open` returns on Linux, macOS, the BSDs and Windows (`p.(serial.ModemLines)`):

| Method | POSIX | Windows |
|---|---|---|
| `SetDTR(on bool) error` | `TIOCMBIS` / `TIOCMBIC` with `TIOCM_DTR` | `EscapeCommFunction` `SETDTR` / `CLRDTR` |
| `SetRTS(on bool) error` | `TIOCMBIS` / `TIOCMBIC` with `TIOCM_RTS` | `EscapeCommFunction` `SETRTS` / `CLRRTS` |
| `ModemStatus() (ModemStatus, error)` | `TIOCMGET` | `GetCommModemStatus` |

`ModemStatus` has `CTS`, `DSR`, `RI` and `DCD`, true when asserted. `Close` does not restore the levels `SetDTR` and `SetRTS` set ([ARCHITECTURE.md](ARCHITECTURE.md#state-saved-and-restored)). `SetRTS` returns `ErrLineInUse` under `FlowRTSCTS` or with RS-485 enabled, `SetDTR` under `FlowDTRDSR`; a closed port returns `ErrClosed`; a port without modem lines (a pseudo-terminal) returns the operating system's error, wrapped as `serial: set DTR: …`, `serial: set RTS: …` or `serial: modem status: …`.

## `modbus`

| Symbol | Purpose |
|---|---|
| `DefaultRTUConfig(address string) serial.Config` | 19200 baud, 8 data bits, 1 stop bit, even parity |
| `DefaultModbusRTUConfig` | Variable holding the same function |
