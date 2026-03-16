package serial

import "time"

// Config describes how a serial port should be opened and configured.
// Zero-value policy: Open normalizes before use. BaudRate(0)→9600, DataBits(0)→8, StopBits(0)→1, Parity("")→none.
type Config struct {
	// Device path (e.g. /dev/ttyUSB0 on Unix, COM1 on Windows).
	Address string
	// Baud rate. Zero = 9600. Use constants or BaudRate(n) for custom rates.
	BaudRate BaudRate
	// Data bits (5–8). Zero = 8.
	DataBits DataBits
	// Stop bits (1 or 2). Zero = 1.
	StopBits StopBits
	// Parity. Empty = none. Use constants or ParseParity for CLI input.
	Parity Parity
	// Timeout for read/write. Zero (NoTimeout) = no deadline. Behavior is platform-dependent.
	Timeout time.Duration
	// RS-485 options. Ignored unless Enabled is true.
	RS485 RS485Config
}

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
// address, data bits, stop bits, parity, timeout, and RS-485 delays. Unsupported baud
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
// BaudRate 0 → 9600, DataBits 0 → 8, StopBits 0 → 1, Parity "" → none.
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
	return out
}

// Normalized returns a copy of c with zero-value fields replaced by generic defaults
// (BaudRate 0→9600, DataBits 0→8, StopBits 0→1, Parity ""→none). Useful for logging
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
