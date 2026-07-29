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
Do not run multiple concurrent Readers or Writers, and do not call Close
concurrently with Read or Write — that is undefined.

# Timeouts and Close

Config.Timeout applies to Read on all platforms; Write timeout is honored on
Windows when Timeout > 0, and is not applied on POSIX (Write is a blocking
syscall.Write). Timeout == 0 means Read may block indefinitely until data
arrives (or the fd/handle is closed by the OS).

Close restores prior termios/DCB settings when possible, then closes the
fd/handle. Closing may cause a blocked Read/Write to return an error on some
platforms, but that is not a synchronized cancel contract — prefer a finite
Timeout for Read, and avoid Close while I/O is in flight.
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
