# go-serial

[![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Go Reference](https://pkg.go.dev/badge/github.com/otfabric/go-serial.svg)](https://pkg.go.dev/github.com/otfabric/go-serial)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/otfabric/go-serial/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/go-serial/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/gh/otfabric/go-serial/graph/badge.svg)](https://codecov.io/gh/otfabric/go-serial)
[![Release](https://img.shields.io/github/v/release/otfabric/go-serial?label=release)](https://github.com/otfabric/go-serial/releases)

A generic cross-platform Go library for serial (UART) communication over RS-232 and RS-485.

## Table of Contents

- [Installation](#installation)
- [What this library is](#what-this-library-is)
- [What this library is not](#what-this-library-is-not)
- [Usage](#usage)
  - [Generic serial (default 9600 8N1)](#generic-serial-default-9600-8n1)
  - [Modbus RTU preset (19200 8E1)](#modbus-rtu-preset-19200-8e1)
  - [Typed configuration](#typed-configuration)
  - [Timeout and error handling](#timeout-and-error-handling)
  - [Config validation](#config-validation)
  - [Zero-value defaults](#zero-value-defaults)
  - [RS-485](#rs-485)
  - [Supported platforms](#supported-platforms)
- [Automated tests](#automated-tests)
- [Manual loopback testing](#manual-loopback-testing)
  - [Linux and macOS](#linux-and-macos)
  - [Windows](#windows)
- [Migration from goburrow/serial](#migration-from-goburrowserial)
- [License](#license)

## Installation

```bash
go get github.com/otfabric/go-serial
```

## What this library is

- **Generic serial transport**: open, configure, read, write, and close serial ports.
- **Cross-platform**: Linux, macOS, Windows, and BSD variants.
- **Protocol-neutral**: no built-in protocol logic. Use it for any serial-based protocol (custom, Modbus RTU, etc.).
- **RS-485**: optional RS-485 driver configuration where the OS supports it.

## What this library is not

It is **not** a Modbus (or any other protocol) implementation. It does not handle framing, CRC, ADU/PDU, or master/slave logic. For Modbus, use a dedicated Modbus library and pass it a port opened with this library.

## Usage

### Generic serial (default 9600 8N1)

```go
package main

import (
	"log"

	"github.com/otfabric/go-serial"
)

func main() {
	cfg := serial.DefaultConfig("/dev/ttyUSB0")
	port, err := serial.Open(&cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer port.Close()

	_, err = port.Write([]byte("hello"))
	if err != nil {
		log.Fatal(err)
	}
}
```

### Modbus RTU preset (19200 8E1)

For Modbus RTU over serial, use the `serial/modbus` package to get standard RTU defaults, then open with the root package. `modbus.DefaultModbusRTUConfig` is an alias for `DefaultRTUConfig`.

```go
package main

import (
	"log"

	"github.com/otfabric/go-serial"
	"github.com/otfabric/go-serial/modbus"
)

func main() {
	cfg := modbus.DefaultRTUConfig("/dev/ttyUSB0")
	port, err := serial.Open(&cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer port.Close()

	// Use port with your Modbus library (framing, CRC, etc. are not in this repo).
}
```

### Typed configuration

Config uses typed fields and constants:

```go
cfg := serial.DefaultConfig("/dev/ttyUSB0")
cfg.BaudRate = serial.Baud115200
cfg.DataBits = serial.DataBits8
cfg.StopBits = serial.StopBits1
cfg.Parity = serial.ParityNone
```

### Timeout and error handling

Set `Config.Timeout` to influence blocking I/O timeout behavior. Read timeout behavior is supported across platforms; write timeout behavior may depend on the platform and driver. When a read times out, the port returns `serial.ErrTimeout`. Use `errors.Is` to detect it:

```go
package main

import (
	"errors"
	"log"
	"time"

	"github.com/otfabric/go-serial"
)

func main() {
	cfg := serial.DefaultConfig("/dev/ttyUSB0")
	cfg.Timeout = 5 * time.Second
	port, err := serial.Open(&cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer port.Close()

	buf := make([]byte, 256)
	n, err := port.Read(buf)
	if err != nil {
		if errors.Is(err, serial.ErrTimeout) {
			// read deadline exceeded, no data
			return
		}
		log.Fatal(err)
	}
	_ = n // use buf[:n]
}
```

### Config validation

Config is validated when you call `Open`. Invalid settings (empty address, unsupported parity, negative timeout, etc.) return an error wrapping `serial.ErrInvalidConfig`. Use `errors.As` with `*serial.ConfigError` to get the invalid field, value, and reason:

```go
package main

import (
	"errors"
	"log"

	"github.com/otfabric/go-serial"
)

func main() {
	_, err := serial.Open(&serial.Config{Address: ""})
	if err != nil {
		if errors.Is(err, serial.ErrInvalidConfig) {
			var cfgErr *serial.ConfigError
			if errors.As(err, &cfgErr) {
				log.Printf("invalid config: field=%s value=%v reason=%s",
					cfgErr.Field, cfgErr.Value, cfgErr.Reason)
			}
		}
		return
	}
}
```

You can also call `cfg.Validate()` or `serial.ValidateConfig(&cfg)` before opening.

Use `serial.IsUnsupportedBaudRate(err)` to detect when the baud rate is not supported on this platform.

### Zero-value defaults

Before opening, `Open` normalizes config: `BaudRate(0)`→9600, `DataBits(0)`→8, `StopBits(0)`→1, `Parity("")`→none. So you can set only `Address` and get 9600 8N1.

### RS-485

RS-485 options live on `Config.RS485`. Set `RS485.Enabled` to true and configure delays and RTS behavior as needed. Support is platform-dependent (e.g. Linux with RS485-capable drivers). The transport API is unchanged; RS-485 is not Modbus-specific.

### Supported platforms

- Linux
- macOS (Darwin)
- Windows
- FreeBSD, NetBSD, OpenBSD

On Windows, any positive baud rate is passed to the driver; acceptance is driver-dependent. On POSIX, supported baud rates are platform-specific (see your OS termios).

## Automated tests

Run the test suite with:

```bash
go test ./...
```

Or use the Makefile targets:

```bash
make test        # unit tests (shuffle enabled)
make test-race   # race detector
make coverage    # library coverage report (example/ excluded)
make check       # fmt, tidy, vet, lint, tests, race, coverage
```

Tests do not require physical serial hardware. On Linux, macOS, and BSD, **`TestReadWrite`** creates a programmatic PTY loopback via `/dev/ptmx` and exercises open, read, write, and close. The test skips when PTY creation is unavailable (e.g. restricted environments).

For a manual socat loopback, set device paths before testing:

```bash
# Terminal 1: socat -d -d pty,raw,echo=0 pty,raw,echo=0
export SERIAL_LOOPBACK_PTY1=/dev/pts/3   # paths from socat output
export SERIAL_LOOPBACK_PTY2=/dev/pts/4
go test -run TestReadWrite ./...
```

The **`example/`** package is a standalone demo (`go run ./example/...`); it is excluded from **`make coverage`** and from CI coverage via the shared workflow package filter.

## Manual loopback testing

For interactive debugging outside the test suite:

### Linux and macOS

```bash
socat -d -d pty,raw,echo=0 pty,raw,echo=0
```

On macOS: `brew install socat`. Use the two PTY paths from socat output with your serial tool or set **`SERIAL_LOOPBACK_PTY1`** / **`SERIAL_LOOPBACK_PTY2`** for **`TestReadWrite`**.

### Windows

- [Null-modem emulator](http://com0com.sourceforge.net/)
- [Terminal](https://sites.google.com/site/terminalbpp/)

---

## Migration from goburrow/serial

| Old | New |
|-----|-----|
| `github.com/goburrow/serial` | `github.com/otfabric/go-serial` |
| Defaults (19200, even parity) | Generic defaults are now 9600 8N1. Use `serial.DefaultConfig(address)` for generic, or `modbus.DefaultRTUConfig(address)` for Modbus RTU (19200 8E1). |

**Behavior change**: If you relied on zero-value `Config` defaults (19200, even parity), that behavior was Modbus-oriented. In go-serial, zero-value defaults are now generic (9600, no parity). For Modbus RTU, switch to:

```go
cfg := modbus.DefaultRTUConfig("/dev/ttyUSB0")
port, err := serial.Open(&cfg)
```

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).
