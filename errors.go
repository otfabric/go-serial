package serial

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrTimeout is returned when a read or write does not complete before the deadline.
	// Config.Timeout influences read/write behavior; semantics are platform-dependent. On most
	// platforms, ErrTimeout is returned when a read times out with no data; write timeout behavior
	// may differ. Zero timeout means no deadline (block until I/O completes). Use errors.Is(err, serial.ErrTimeout) to detect.
	ErrTimeout = errors.New("serial: timeout")

	// ErrInvalidConfig is returned when config validation fails (e.g. empty address,
	// unsupported parity). Use errors.Is(err, serial.ErrInvalidConfig) to detect.
	// Use errors.As(err, &cfgErr) with *ConfigError to get field, value, and reason.
	ErrInvalidConfig = errors.New("serial: invalid config")

	// ErrUnsupportedPlatform is returned by Open when the current OS is not supported
	// (no platform init ran). Use errors.Is(err, serial.ErrUnsupportedPlatform) to detect.
	ErrUnsupportedPlatform = errors.New("serial: unsupported platform")
)

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
