// SPDX-License-Identifier: MIT

package serial

import (
	"math"
	"time"
)

// Config describes how a serial port should be opened and configured.
// Zero-value policy: Open normalizes before use. BaudRate(0)→9600, DataBits(0)→8, StopBits(0)→1, Parity("")→none,
// FlowControl("")→none.
type Config struct {
	// Device path (e.g. /dev/ttyUSB0 on Unix, COM1 on Windows). On POSIX
	// systems Open fails unless the path is a character device.
	//
	// On Windows Address is a device name: NAME, NAME: (COM1, COM1:,
	// COM10), \\.\NAME or \\?\NAME (\\.\COM10, a device interface name),
	// \\.\Global\NAME or \\.\GLOBALROOT\Device\NAME, where NAME holds no
	// backslash, slash or colon. Open opens it in the device namespace, as
	// \\.\NAME, so port numbers above 9 need no prefix. Any other form fails
	// with a *ConfigError wrapping ErrInvalidConfig before anything is
	// opened, unless UncheckedAddress is set.
	Address string
	// Baud rate. Zero = 9600. Use constants or BaudRate(n) for custom rates.
	BaudRate BaudRate
	// Data bits (5–8). Zero = 8.
	DataBits DataBits
	// Stop bits (1 or 2). Zero = 1.
	StopBits StopBits
	// Parity. Empty = none. Use constants or ParseParity for CLI input.
	Parity Parity
	// Timeout for Read, and on Windows for Write as well. Zero (NoTimeout) =
	// no deadline. See the package documentation.
	Timeout time.Duration
	// Flow control. Empty = none. FlowRTSCTS and FlowXONXOFF work on every
	// platform; FlowDTRDSR on Windows only, and Open fails with
	// ErrUnsupportedPlatform for it elsewhere. FlowRTSCTS with RS485.Enabled
	// is invalid: RS-485 drives RTS.
	FlowControl FlowControl
	// RS-485 options. Ignored unless Enabled is true.
	RS485 RS485Config
	// VerifySettings makes Open read the settings back from the driver
	// after setting them, and fail when the port did not take them: a
	// *ConfigError wrapping ErrInvalidConfig names the field and what the
	// port reports, and the port is closed. Compared are the speed, the
	// data bits, the stop bits, the parity and the flow control and, with
	// RS485.Enabled, the two RS-485 delays. POSIX systems only; on Windows SetCommState
	// reports a setting the driver refuses as an error, and the field has
	// no effect. A pseudo-terminal on Linux takes 8 data bits without
	// parity whatever is asked: with VerifySettings, other data bits or a
	// parity fail there.
	VerifySettings bool
	// UncheckedAddress makes the Windows backend pass Address to
	// CreateFile as given, as releases before v0.5.0 did, instead of
	// accepting a device name only. CreateFile then opens whatever object
	// the name refers to before the backend can tell whether it is a serial
	// port: set it only for an address from a trusted source. It has no
	// effect on POSIX systems.
	UncheckedAddress bool
}

// MaxRS485Delay is the largest RS485Config.DelayRtsBeforeSend or
// DelayRtsAfterSend that Validate accepts. The kernel takes the delays as
// 32-bit millisecond counts.
const MaxRS485Delay = time.Duration(math.MaxUint32) * time.Millisecond

// RS485Config holds platform-independent RS-485 options. Ignored unless Enabled is true.
// Support depends on the OS and driver (e.g. Linux with RS485-capable serial drivers).
type RS485Config struct {
	Enabled            bool          // Enable RS-485 mode
	DelayRtsBeforeSend time.Duration // Delay RTS before send
	DelayRtsAfterSend  time.Duration // Delay RTS after send
	RtsHighDuringSend  bool          // RTS high during send
	RtsHighAfterSend   bool          // RTS high after send
	RxDuringTx         bool          // Receive during transmit
}

// DefaultConfig returns a generic serial config for the given address (9600 8N1).
// Use this for protocol-neutral UART/RS-232/RS-485. For Modbus RTU defaults,
// use the serial/modbus package instead.
func DefaultConfig(address string) Config {
	return Config{
		Address:  address,
		BaudRate: Baud9600,
		DataBits: DataBits8,
		StopBits: StopBits1,
		Parity:   ParityNone,
	}
}

// Validate checks the config for invalid or unsupported values. Root validation covers
// address, data bits, stop bits, parity, flow control, timeout, and RS-485 delays. Unsupported baud
// is reported at Open (platform-specific). Returns a *ConfigError wrapping ErrInvalidConfig;
// use errors.As(err, &cfgErr) to get Field, Value, and Reason.
func (c Config) Validate() error {
	if c.Address == "" {
		return &ConfigError{Field: "Address", Value: c.Address, Reason: "empty address", Err: ErrInvalidConfig}
	}
	if c.DataBits != 0 && c.DataBits != DataBits5 && c.DataBits != DataBits6 && c.DataBits != DataBits7 && c.DataBits != DataBits8 {
		return &ConfigError{Field: "DataBits", Value: c.DataBits, Reason: "must be 5, 6, 7, or 8", Err: ErrInvalidConfig}
	}
	if c.StopBits != 0 && c.StopBits != StopBits1 && c.StopBits != StopBits2 {
		return &ConfigError{Field: "StopBits", Value: c.StopBits, Reason: "must be 1 or 2", Err: ErrInvalidConfig}
	}
	switch c.Parity {
	case "", ParityNone, ParityEven, ParityOdd:
	default:
		return &ConfigError{Field: "Parity", Value: c.Parity, Reason: "must be N, E, or O", Err: ErrInvalidConfig}
	}
	switch c.FlowControl {
	case "", FlowNone, FlowRTSCTS, FlowDTRDSR, FlowXONXOFF:
	default:
		return &ConfigError{Field: "FlowControl", Value: c.FlowControl, Reason: "must be none, rts-cts, dtr-dsr or xon-xoff", Err: ErrInvalidConfig}
	}
	if c.FlowControl == FlowRTSCTS && c.RS485.Enabled {
		return &ConfigError{Field: "FlowControl", Value: c.FlowControl, Reason: "RS-485 drives RTS", Err: ErrInvalidConfig}
	}
	if c.Timeout < 0 {
		return &ConfigError{Field: "Timeout", Value: c.Timeout, Reason: "cannot be negative", Err: ErrInvalidConfig}
	}
	if c.RS485.Enabled {
		if c.RS485.DelayRtsBeforeSend < 0 {
			return &ConfigError{Field: "RS485.DelayRtsBeforeSend", Value: c.RS485.DelayRtsBeforeSend, Reason: "cannot be negative", Err: ErrInvalidConfig}
		}
		if c.RS485.DelayRtsAfterSend < 0 {
			return &ConfigError{Field: "RS485.DelayRtsAfterSend", Value: c.RS485.DelayRtsAfterSend, Reason: "cannot be negative", Err: ErrInvalidConfig}
		}
		if c.RS485.DelayRtsBeforeSend > MaxRS485Delay {
			return &ConfigError{Field: "RS485.DelayRtsBeforeSend", Value: c.RS485.DelayRtsBeforeSend, Reason: "exceeds MaxRS485Delay", Err: ErrInvalidConfig}
		}
		if c.RS485.DelayRtsAfterSend > MaxRS485Delay {
			return &ConfigError{Field: "RS485.DelayRtsAfterSend", Value: c.RS485.DelayRtsAfterSend, Reason: "exceeds MaxRS485Delay", Err: ErrInvalidConfig}
		}
	}
	return nil
}

// ValidateConfig validates config. Nil-safe convenience; when c is non-nil, c.Validate() is equivalent.
// Returns ErrInvalidConfig for nil or invalid config.
func ValidateConfig(c *Config) error {
	if c == nil {
		return &ConfigError{Field: "Config", Value: nil, Reason: "nil config", Err: ErrInvalidConfig}
	}
	return c.Validate()
}

// normalizeConfig returns a copy of c with zero values replaced by generic defaults:
// BaudRate 0 → 9600, DataBits 0 → 8, StopBits 0 → 1, Parity "" → none, FlowControl "" → none.
func normalizeConfig(c *Config) Config {
	out := *c
	if out.BaudRate == 0 {
		out.BaudRate = Baud9600
	}
	if out.DataBits == 0 {
		out.DataBits = DataBits8
	}
	if out.StopBits == 0 {
		out.StopBits = StopBits1
	}
	if out.Parity == "" {
		out.Parity = ParityNone
	}
	if out.FlowControl == "" {
		out.FlowControl = FlowNone
	}
	return out
}

// Normalized returns a copy of c with zero-value fields replaced by generic defaults
// (BaudRate 0→9600, DataBits 0→8, StopBits 0→1, Parity ""→none, FlowControl ""→none). Useful for logging
// effective config or validating after normalization. Open applies the same logic internally.
//
// Example:
//
//	cfg := serial.Config{Address: "/dev/ttyUSB0"}
//	effective := cfg.Normalized()
//	log.Printf("effective config: %d 8N1", effective.BaudRate)
func (c Config) Normalized() Config {
	return normalizeConfig(&c)
}
