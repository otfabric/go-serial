// SPDX-License-Identifier: MIT

/*
Package serial provides a cross-platform library for opening, configuring, and
using serial ports (UART, RS-232, RS-485). It is transport-focused and
protocol-neutral. For Modbus RTU serial presets, see the serial/modbus subpackage.

# Ownership

Open copies and normalizes Config; the caller owns the returned Port and must
Close it. Do not use the Port after Close.

# Concurrency

One Read and one Write may run concurrently on the same Port (full-duplex).
Do not run multiple concurrent Readers or Writers. On Windows the port is a
synchronous handle, so a Write issued while a Read waits may wait for that
Read to return; use a finite Timeout there when reading and writing from
separate goroutines. The ModemLines methods may be called from any goroutine,
also while a Read or Write runs; on Windows such a call may likewise wait for
a pending Read. Close waits for them as it does for Read and Write, and they
return ErrClosed after Close.

# Timeouts and Close

Config.Timeout applies to Read on all platforms; Write timeout is honored on
Windows when Timeout > 0, where a Write that runs out of time returns the
number of bytes written and ErrTimeout. It is not applied on POSIX, where
Write blocks while the driver's output buffer is full and returns once all
bytes are written or an error occurs. On every platform Write returns
n < len(b) only with an error. Timeout == 0 means Read may block
indefinitely until data arrives (or the fd/handle is closed by the OS).

Close restores prior termios/DCB settings when possible, then closes the
fd/handle. Close may be called more than once and from several goroutines.
Close also cancels I/O: a Read or Write in progress in another goroutine
returns ErrClosed, Close waits until it has returned before it closes the
descriptor or handle, and Read and Write after Close return ErrClosed. On
Windows the cancel is CancelIoEx on the port's handle.

On Windows Config.Address must be a device name such as COM3 or \\.\COM10;
see Config.Address and Config.UncheckedAddress. Config.VerifySettings makes
Open on POSIX systems check that the port took the settings.

Config.FlowControl sets RTS/CTS or XON/XOFF flow control, and DTR/DSR on
Windows. The port Open returns also implements ModemLines: SetDTR and SetRTS
drive the modem control outputs and ModemStatus reads CTS, DSR, RI and DCD.

On POSIX systems Read follows the io.Reader contract when the line goes
away: it returns n == 0 with io.EOF once the other end hung up (an
unplugged USB adapter, a closed pseudo-terminal) or with the OS error
(for example EIO), never a negative count.
*/
package serial

import (
	"errors"
	"fmt"
	"io"
)

// Port represents an opened serial port returned by Open.
// The caller owns the Port and must Close it. See package docs for
// concurrency and Close semantics.
type Port interface {
	io.ReadWriteCloser
}

// openPort is set by platform-specific files (serial_posix.go, serial_windows.go).
var openPort func(*Config) (Port, error)

// isBaudRateSupported is set by each platform to validate baud rate before open. If nil, no check.
var isBaudRateSupported func(BaudRate) bool

// Open opens a serial port with the given config. It validates and normalizes the config,
// then opens the device. Invalid config returns an error wrapping ErrInvalidConfig.
// Unsupported platform (e.g. unsupported OS) returns ErrUnsupportedPlatform.
func Open(c *Config) (Port, error) {
	if openPort == nil {
		return nil, fmt.Errorf("serial: open: %w", ErrUnsupportedPlatform)
	}
	if err := ValidateConfig(c); err != nil {
		return nil, err
	}
	cfg := normalizeConfig(c)
	if isBaudRateSupported != nil && !isBaudRateSupported(cfg.BaudRate) {
		return nil, &ConfigError{Field: "BaudRate", Value: cfg.BaudRate, Reason: "unsupported baud rate for this platform", Err: ErrInvalidConfig}
	}
	return openPort(&cfg)
}

// IsUnsupportedBaudRate reports whether err indicates the baud rate is not supported on this platform.
// It is true when Open returns a ConfigError with Field "BaudRate" and reason "unsupported baud rate for this platform".
func IsUnsupportedBaudRate(err error) bool {
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		return false
	}
	return cfgErr.Field == "BaudRate" && cfgErr.Reason == "unsupported baud rate for this platform"
}
