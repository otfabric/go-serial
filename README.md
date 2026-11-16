# go-serial — Cross-Platform Serial Port (UART, RS-232 & RS-485) Library for Go (Golang)

[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Go Reference](https://pkg.go.dev/badge/github.com/otfabric/go-serial.svg)](https://pkg.go.dev/github.com/otfabric/go-serial)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI Status](https://github.com/otfabric/go-serial/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/go-serial/actions/workflows/ci.yml)
[![Code Coverage](https://codecov.io/gh/otfabric/go-serial/graph/badge.svg)](https://codecov.io/gh/otfabric/go-serial)
[![Latest Release](https://img.shields.io/github/v/release/otfabric/go-serial?label=release)](https://github.com/otfabric/go-serial/releases)

**go-serial is a cross-platform serial port library for Go.** Open, configure, read, write and close serial (UART) ports over RS-232 and RS-485 on Linux, macOS, Windows and the BSDs, with typed configuration, validated settings and a documented concurrency contract.

It is protocol-neutral, so you can use it for custom serial protocols, GPS and sensor modules, microcontroller consoles and **Modbus RTU** (a ready-made 19200 8E1 preset is included). It is also the serial layer used by [go-modbus](https://github.com/otfabric/go-modbus).

If you are looking for a maintained alternative to [goburrow/serial](https://github.com/goburrow/serial), see the [migration guide](#migrating-from-goburrowserial).

**Quick links:** [Quick start](#quick-start) · [Comparison](#go-serial-vs-other-go-serial-port-libraries) · [Modbus RTU preset](#modbus-rtu-preset-19200-8e1) · [Migration guide](#migrating-from-goburrowserial) · [FAQ](#faq)

---

## Why go-serial?

- **Simple API** — `serial.Open(&cfg)` returns a `Port` with standard `Read`, `Write` and `Close`.
- **RS-232 and RS-485** — Optional RS-485 driver configuration (RTS control and delays) where the OS supports it.
- **Typed configuration** — Constants for baud rate, data bits, stop bits and parity instead of bare integers and strings.
- **Validated settings** — Invalid configuration fails at `Open` with `serial.ErrInvalidConfig` and a `*serial.ConfigError` that names the field, value and reason.
- **Sensible defaults** — Zero values give you 9600 8N1; use the `modbus` package for Modbus RTU defaults (19200 8E1).
- **Clear timeouts and errors** — Read timeouts return `serial.ErrTimeout`, detectable with `errors.Is`.
- **Documented concurrency and close semantics** — One reader and one writer may run concurrently; ownership and `Close` behavior are spelled out.
- **Cross-platform** — Backends for Linux, macOS, Windows, FreeBSD, NetBSD and OpenBSD. Linux and macOS are tested in CI; the others are tested in part or only built: see [Supported platforms](#supported-platforms).
- **Testable without hardware** — The tests run over pseudo-terminals on Linux and macOS.

---

## Quick start

```bash
go get github.com/otfabric/go-serial
```

Requires **Go 1.26** or later.

Open `/dev/ttyUSB0` at the default 9600 8N1 and write some bytes:

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

On Windows, use a device name such as `COM3` as the address. See [Usage](#usage) for timeouts, typed configuration, RS-485 and error handling.

---

## go-serial vs other Go serial port libraries

Several Go serial libraries exist. This table compares go-serial with [bugst/go-serial](https://github.com/bugst/go-serial) (import path `go.bug.st/serial`) and [goburrow/serial](https://github.com/goburrow/serial) so you can choose the right one.

| Capability | otfabric/go-serial | bugst/go-serial | goburrow/serial |
|---|:---:|:---:|:---:|
| Open, read, write, close (RS-232/UART) | ✅ | ✅ | ✅ |
| Linux, macOS and Windows | ✅ | ✅ | ✅ |
| Read timeout with a sentinel error (`ErrTimeout`) | ✅ | ✅ | ✅ |
| Typed constants for baud, data bits, stop bits, parity | ✅ | partial | — |
| Config validation errors with field, value and reason | ✅ | — | — |
| RS-485 driver configuration | ✅ | — | ✅ |
| Modbus RTU preset (19200 8E1) | ✅ | — | implicit default |
| Documented concurrency and `Close` contract | ✅ | — | — |
| Port enumeration (list available ports) | — | ✅ | — |
| USB metadata (VID/PID, serial number) | — | ✅ | — |
| Modem control lines (DTR, RTS, status bits) | — | ✅ | — |
| License | MIT | BSD-3-Clause | MIT |

✅ = documented feature · partial = some of it · — = not documented / not available. Based on each project's public documentation as of October 2026. If you spot an inaccuracy, please [open an issue](https://github.com/otfabric/go-serial/issues) and we will correct it.

**Choose go-serial if** you want a small, strictly validated serial transport with typed configuration, RS-485 support and clear concurrency rules, especially as the serial layer under a Modbus RTU library, or as a replacement for goburrow/serial.

**Choose bugst/go-serial if** you need to list ports, read USB device metadata or control modem lines (DTR/RTS) directly. go-serial does not provide these.

---

## Use cases

- **Modbus RTU over RS-485/RS-232** — Pair with [go-modbus](https://github.com/otfabric/go-modbus) or any Modbus library.
- **Industrial and IoT gateways** — Talk to meters, sensors and controllers over serial lines.
- **Embedded and lab tooling** — Drive microcontroller consoles, GPS modules and test equipment.
- **Custom serial protocols** — Build framing and parsing on top of a reliable, validated transport.

---

## Table of Contents

- [Why go-serial?](#why-go-serial)
- [Quick start](#quick-start)
- [go-serial vs other Go serial port libraries](#go-serial-vs-other-go-serial-port-libraries)
- [Use cases](#use-cases)
- [What this library is](#what-this-library-is)
- [What this library is not](#what-this-library-is-not)
- [Migrating from goburrow/serial](#migrating-from-goburrowserial)
- [Usage](#usage)
- [Automated tests](#automated-tests)
- [Manual loopback testing](#manual-loopback-testing)
- [FAQ](#faq)
- [Project structure](#project-structure)
- [Contributing](#contributing)
- [License](#license)

---

## What this library is

- **Generic serial transport**: open, configure, read, write, and close serial ports.
- **Cross-platform**: Linux, macOS, Windows, and BSD variants, at the levels listed under [Supported platforms](#supported-platforms).
- **Protocol-neutral**: no built-in protocol logic. Use it for any serial-based protocol (custom, Modbus RTU, etc.).
- **RS-485**: optional RS-485 driver configuration where the OS supports it.

## What this library is not

It is **not** a Modbus (or any other protocol) implementation. It does not handle framing, CRC, ADU/PDU, or master/slave logic. For Modbus, use a dedicated Modbus library and pass it a port opened with this library.


## Migrating from goburrow/serial

| Old | New |
|-----|-----|
| `github.com/goburrow/serial` | `github.com/otfabric/go-serial` |
| Defaults (19200, even parity) | Generic defaults are now 9600 8N1. Use `serial.DefaultConfig(address)` for generic, or `modbus.DefaultRTUConfig(address)` for Modbus RTU (19200 8E1). |

**Behavior change**: If you relied on zero-value `Config` defaults (19200, even parity), that behavior was Modbus-oriented. In go-serial, zero-value defaults are now generic (9600, no parity). For Modbus RTU, switch to:

```go
cfg := modbus.DefaultRTUConfig("/dev/ttyUSB0")
port, err := serial.Open(&cfg)
```

---

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

### Ownership, concurrency, and Close

- **Ownership** — `Open` copies and normalizes `Config`. The caller owns the
  returned `Port` and must `Close` it. Do not use the port after `Close`.
- **Concurrency** — One `Read` and one `Write` may run concurrently on the same
  `Port` (full-duplex). Do **not** run multiple concurrent readers or writers.
  On **Windows** the port is a synchronous handle: a `Write` issued while a
  `Read` waits may wait for that `Read` to return, so use a finite `Timeout`
  when reading and writing from separate goroutines.
- **Timeouts** — `Config.Timeout` applies to **Read** on all platforms. On
  **Windows**, the same timeout is also applied to Write when `Timeout > 0`:
  a Write that runs out of time returns the number of bytes written and
  `ErrTimeout`.
  On **POSIX** (Linux/macOS/BSD), Write has no library-applied timeout: it
  blocks while the driver's output buffer is full and returns only once all
  bytes are written (or with an error). `Timeout == 0` means Read may block indefinitely until data
  arrives (or the underlying fd/handle is closed by the OS).
- **Close** — Restores prior termios (POSIX) or DCB/timeouts (Windows) when
  possible, then closes the fd/handle. `Close` may be called more than once
  and from several goroutines. It also cancels I/O: a `Read` or `Write` in
  progress in another goroutine returns `ErrClosed`, `Close` waits until it
  has returned, and later calls return `ErrClosed`. On **Windows** the
  cancel is `CancelIoEx` on the port's handle.
- **Hangup** — On POSIX systems, `Read` returns `io.EOF` (or the OS error, such
  as `EIO`) with `n == 0` once the device goes away, e.g. an unplugged USB
  adapter; it never returns a negative count.

Package godoc (`serial.go`) restates the same contract.

### Timeout and error handling

Set `Config.Timeout` to bound blocking I/O. It applies to `Read` on every platform and to `Write` on Windows only. When a read times out, the port returns `serial.ErrTimeout`; a Windows write that times out returns it with the number of bytes written. `ErrTimeout` also matches `os.ErrDeadlineExceeded` and has a `Timeout()` method, like the time-outs of `net` and `os`. Use `errors.Is` to detect it:

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

### Checking that the port took the settings

A driver can accept a setting without applying it. With `Config.VerifySettings`, `Open` on POSIX systems reads the settings back and fails with a `*serial.ConfigError` wrapping `serial.ErrInvalidConfig` that names the field the port did not take: the speed, data bits, stop bits, parity and, with RS-485 on Linux, the two delays. A Linux pseudo-terminal sets 8 data bits and no parity whatever it is given, so do not set the option there for other settings. It has no effect on Windows.

### RS-485

RS-485 options live on `Config.RS485`. Set `RS485.Enabled` to true and configure delays and RTS behavior as needed. Support is platform-dependent (e.g. Linux with RS485-capable drivers). The transport API is unchanged; RS-485 is not Modbus-specific.

### Supported platforms

The library has a backend for each platform below. They are not at the same level: the table says what CI runs.

| Platform | Level | What CI does |
|---|---|---|
| Linux | Tested over pseudo-terminals | Every pull request that changes code runs all tests against the kernel's tty layer, with the race detector, on Go 1.26 and 1.27. Nightly: the chaos tests with more rounds, the fuzz targets, and all tests as a 32-bit (386) binary. Built and vetted for amd64, arm (v7) and arm64 |
| macOS | Tested over pseudo-terminals | The same pull requests run all tests with the race detector; nightly, the chaos tests. Built and vetted for amd64 and arm64 |
| Windows | Built; tested without a port | Built and vetted for amd64 and arm64. A `windows-latest` job runs the tests that need no port: the time-out and DCB conversions, the address check, `Write` on a replaced `WriteFile`, and `Close` cancelling `Read` and `Write` on replaced system calls. No test opens a port, so `Open`, `Read` and `Close` on a device are not exercised |
| FreeBSD | Built, not run | Built and vetted for amd64 and arm64. No job runs its tests |
| NetBSD | Built, not run | Built and vetted for amd64. No job runs its tests |
| OpenBSD | Built, not run | Built and vetted for amd64. Nothing has been run on OpenBSD. It waits for I/O with the `select` code that the macOS tests run |

No test on any platform uses a physical UART, a USB adapter or an RS-485 transceiver: the baud rate, parity and stop bits are never observed on a line.

Known not to work:

- **RS-485 outside Linux.** With `Config.RS485.Enabled`, `Open` fails with `ErrUnsupportedPlatform` on macOS and the BSDs, and the Windows backend ignores the setting.
- **Descriptors at or above 1024 on macOS and OpenBSD.** `Read`, and a `Write` that has to wait, fail with `EINVAL`, because `select` cannot watch them.
- **OpenBSD with a release before v0.4.1 built with Go 1.26 or later.** `Read` fails there with "function not implemented". Use v0.4.1 or later.

[LIMITATIONS.md](LIMITATIONS.md) has the differences between the platforms and the list of what is not verified.

On Windows, any positive baud rate is passed to the driver; acceptance is driver-dependent. On POSIX, supported baud rates are platform-specific (see your OS termios). The address is a device path on POSIX systems and must be a character device. On Windows it is a device name, such as `COM3`, `COM3:` or `\\.\COM10`, which `Open` opens in the device namespace, so `COM10` and higher work as they are. Any other form fails with `serial.ErrInvalidConfig` before anything is opened; `Config.UncheckedAddress` passes the address to `CreateFile` as given, as before v0.5.0.

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

Tests do not require physical serial hardware. On Linux, macOS and FreeBSD, **`TestReadWrite`** creates a programmatic PTY loopback via `/dev/ptmx` and exercises open, read, write, and close; CI runs it on Linux and macOS. The test skips when PTY creation is unavailable (e.g. restricted environments, NetBSD and OpenBSD).

**Chaos and fuzz tests** (`TestChaos*`, `Fuzz*`) inject faults over PTYs: random line fragmentation, hangups mid-stream, signal storms during blocking reads, ports above `FD_SETSIZE`, write backpressure, random configs and open/close churn. Each logs its seed; reproduce or scale a run with:

```bash
SERIAL_CHAOS_SEED=<seed> go test -run TestChaos -v .
SERIAL_CHAOS_ITERATIONS=10 go test -race -run TestChaos .
go test -run '^$' -fuzz FuzzConfigValidate -fuzztime 1m .
```

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

## FAQ

### How do I open a serial port in Go?

Create a config with `serial.DefaultConfig("/dev/ttyUSB0")` (or `COM3` on Windows), call `serial.Open(&cfg)`, then use `Read`, `Write` and `Close` on the returned port. See the [Quick start](#quick-start).

### Does go-serial support RS-485?

Yes. Set `Config.RS485.Enabled` and configure RTS behavior and delays. Support depends on the platform and driver. See [RS-485](#rs-485).

### What serial settings does Modbus RTU use?

The `modbus` package provides `DefaultRTUConfig`, which gives the usual Modbus RTU defaults of 19200 baud, 8 data bits, even parity and 1 stop bit. go-serial only provides the transport. For framing, CRC and master/slave logic use a Modbus library such as [go-modbus](https://github.com/otfabric/go-modbus).

### How is it different from goburrow/serial?

go-serial keeps a similar `Config` and `Open` shape but adds typed configuration, validation errors, a documented concurrency contract and a Modbus RTU preset. The zero-value defaults changed from Modbus-style 19200 even parity to a generic 9600 8N1. See the [migration guide](#migrating-from-goburrowserial).

### Can I list the available serial ports?

Not with go-serial. Pass the device path or COM port name directly. If you need enumeration or USB metadata, `go.bug.st/serial` and its `enumerator` package provide it.

### Which platforms are supported?

There are backends for Linux, macOS, Windows, FreeBSD, NetBSD and OpenBSD. Linux and macOS are tested in CI over pseudo-terminals; Windows is built and tested without a port; the BSDs are built and not run. See [Supported platforms](#supported-platforms).

### How can I test without a physical serial device?

Use a PTY pair created with `socat`. The test suite also creates a PTY loopback automatically on Linux, macOS and FreeBSD. See [Manual loopback testing](#manual-loopback-testing).

---

## Project structure

<details>
<summary>Repository layout</summary>

```
go-serial/
├── .                  Public package `serial` — Open, Config, Port
├── modbus/            Modbus RTU serial presets (defaults only)
├── example/           Demo program
├── RELEASE.md         Release history
└── README.md
```

Platform backends are build-tagged (`serial_posix.go`, `serial_windows.go`, …).

Documents: [API.md](API.md), [ARCHITECTURE.md](ARCHITECTURE.md), [ERRORS.md](ERRORS.md), [TESTING.md](TESTING.md), [LIMITATIONS.md](LIMITATIONS.md), [SECURITY.md](SECURITY.md), [CONTRIBUTING.md](CONTRIBUTING.md), [RELEASE.md](RELEASE.md).

</details>

---

## Contributing

Bug reports, platform and driver compatibility notes, and pull requests are welcome. Please [open an issue](https://github.com/otfabric/go-serial/issues) to discuss larger changes first.

If go-serial is useful to you, a star on GitHub helps other Go developers find it.

---

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).