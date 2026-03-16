# go-serial Releases

## v0.1.0 (first release)

Initial public release of `github.com/otfabric/go-serial`, a generic cross-platform Go library for serial (UART) communication over RS-232 and RS-485.

### Highlights

- **Transport API**
  - `Open(*Config) (Port, error)` — validate, normalize, and open a serial port.
  - `Port` implements `io.ReadWriteCloser`; use with any protocol (custom, Modbus RTU, etc.).
  - `DefaultConfig(address)` for generic 9600 8N1.

- **Typed configuration**
  - `Config` with typed fields: `BaudRate`, `DataBits`, `StopBits`, `Parity`, `Timeout`, `RS485`.
  - Constants and `ParseBaudRate`, `ParseParity`, `ParseDataBits`, `ParseStopBits` for CLI/config integration.
  - Zero-value normalization at open (e.g. BaudRate 0 → 9600); `Config.Normalized()` to inspect effective config.
  - Text marshal/unmarshal on config types for config files.

- **Validation and errors**
  - `Validate()` / `ValidateConfig()`; `*ConfigError` with `Field`, `Value`, `Reason` for invalid config.
  - Sentinel errors: `ErrInvalidConfig`, `ErrTimeout`, `ErrUnsupportedPlatform`.
  - `IsUnsupportedBaudRate(err)` for baud-specific failures (POSIX); on Windows, baud is driver-dependent.

- **Modbus RTU preset**
  - `serial/modbus`: `DefaultRTUConfig(address)` (19200 8E1); `DefaultModbusRTUConfig` alias.

- **Platform support**
  - Linux, macOS, Windows, FreeBSD, NetBSD, OpenBSD.
  - Optional RS-485 options where the OS/driver supports them.

- **Tooling and docs**
  - `README.md` with usage, examples, migration from goburrow/serial.
  - `Makefile`: `test`, `test-race`, `coverage`, `verify`, `check`.
  - GitHub Actions: static checks (tidy, gofmt, vet), test matrix (Linux/macOS/Windows + Go 1.20 compat), coverage with example excluded, Codecov.

### Go version

- Requires **Go 1.20+** (see `go.mod`).

### License

- MIT. See [LICENSE](LICENSE).
