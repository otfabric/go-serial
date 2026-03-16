# go-serial

[![Go](https://img.shields.io/badge/Go-1.20%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/otfabric/go-serial)](https://goreportcard.com/report/github.com/otfabric/go-serial)
[![CI](https://github.com/otfabric/go-serial/actions/workflows/test.yml/badge.svg)](https://github.com/otfabric/go-serial/actions/workflows/test.yml)
[![Codecov](https://codecov.io/github/otfabric/go-serial/graph/badge.svg)](https://app.codecov.io/github/otfabric/go-serial)
[![Release](https://img.shields.io/github/v/release/otfabric/go-serial?label=release)](https://github.com/otfabric/go-serial/releases)

A generic cross-platform Go library for serial (UART) communication over RS-232 and RS-485.

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

Tests do not require a serial device; hardware-dependent tests are skipped when no suitable port is available.

## Manual loopback testing

### Linux and macOS

- `socat -d -d pty,raw,echo=0 pty,raw,echo=0`
- On macOS: `brew install socat`

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
