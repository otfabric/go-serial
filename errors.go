// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"fmt"
	"os"
	"time"
)

var (
	// ErrTimeout is returned by Read when no data arrived within
	// Config.Timeout, and on Windows by Write when that time passed before
	// all bytes were written: the count returned with it says how many
	// were. The POSIX Write has no deadline and does not return it. Zero
	// timeout means no deadline (block until I/O completes). Use
	// errors.Is(err, serial.ErrTimeout) to detect.
	//
	// Config.Timeout is an I/O deadline, so ErrTimeout also matches
	// os.ErrDeadlineExceeded, the error of an expired deadline on an
	// os.File or a net.Conn, and has a method Timeout that reports true. It
	// does not match context.DeadlineExceeded: no context is involved.
	ErrTimeout error = timeoutError{}

	// ErrInvalidConfig is returned when config validation fails (e.g. empty address,
	// unsupported parity). Use errors.Is(err, serial.ErrInvalidConfig) to detect.
	// Use errors.As(err, &cfgErr) with *ConfigError to get field, value, and reason.
	ErrInvalidConfig = errors.New("serial: invalid config")

	// ErrUnsupportedPlatform is returned by Open when the current OS is not supported
	// (no platform init ran). Use errors.Is(err, serial.ErrUnsupportedPlatform) to detect.
	ErrUnsupportedPlatform = errors.New("serial: unsupported platform")

	// ErrClosed is returned by Read and Write when the port was closed
	// before or during the call. Use errors.Is(err, serial.ErrClosed) to
	// detect.
	ErrClosed = errors.New("serial: port closed")

	// ErrLineInUse is returned by ModemLines.SetRTS while flow control on
	// RTS and CTS or RS-485 drives RTS, and by ModemLines.SetDTR while flow
	// control on DTR and DSR drives DTR. Use errors.Is(err,
	// serial.ErrLineInUse) to detect.
	ErrLineInUse = errors.New("serial: line driven by flow control or RS-485")
)

// timeoutError is the type of ErrTimeout.
type timeoutError struct{}

func (timeoutError) Error() string { return "serial: timeout" }

// Timeout reports true: the error is a time-out.
func (timeoutError) Timeout() bool { return true }

// Is reports whether target is os.ErrDeadlineExceeded, so that
// errors.Is(err, os.ErrDeadlineExceeded) holds for ErrTimeout.
func (timeoutError) Is(target error) bool { return target == os.ErrDeadlineExceeded }

// NoTimeout means no read/write deadline (block until I/O completes).
const NoTimeout time.Duration = 0

// ConfigError gives field-level details for config validation failures.
// Use errors.As(err, &cfgErr) to inspect; errors.Is(err, ErrInvalidConfig) to detect.
// Reason is stable for human-readable messages; prefer errors.Is and errors.As over string matching.
type ConfigError struct {
	Field  string
	Value  any
	Reason string
	Err    error
}

func (e *ConfigError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Reason != "" {
		return fmt.Sprintf("serial: invalid %s=%v: %s", e.Field, e.Value, e.Reason)
	}
	if e.Err != nil {
		return fmt.Sprintf("serial: invalid %s=%v: %v", e.Field, e.Value, e.Err)
	}
	return fmt.Sprintf("serial: invalid %s=%v", e.Field, e.Value)
}

func (e *ConfigError) Unwrap() error { return e.Err }
