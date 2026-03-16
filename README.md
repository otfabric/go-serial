# go-serial

[![GoDoc](https://pkg.go.dev/badge/github.com/gofabric/go-serial)](https://pkg.go.dev/github.com/gofabric/go-serial)

A generic cross-platform Go library for serial (UART) communication over RS-232 and RS-485.

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

	"github.com/gofabric/go-serial"
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

For Modbus RTU over serial, use the `serial/modbus` package to get standard RTU defaults, then open with the root package:

```go
package main

import (
	"log"

	"github.com/gofabric/go-serial"
	"github.com/gofabric/go-serial/modbus"
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

## Testing

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
| `github.com/goburrow/serial` | `github.com/gofabric/go-serial` |
| Defaults (19200, even parity) | Generic defaults are now 9600 8N1. Use `serial.DefaultConfig(address)` for generic, or `modbus.DefaultRTUConfig(address)` for Modbus RTU (19200 8E1). |

**Behavior change**: If you relied on zero-value `Config` defaults (19200, even parity), that behavior was Modbus-oriented. In go-serial, zero-value defaults are now generic (9600, no parity). For Modbus RTU, switch to:

```go
cfg := modbus.DefaultRTUConfig("/dev/ttyUSB0")
port, err := serial.Open(&cfg)
```
