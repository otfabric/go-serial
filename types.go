// SPDX-License-Identifier: MIT

package serial

import (
	"fmt"
	"strconv"
)

// Parity is the parity mode for serial communication. Use the constants or ParseParity when setting Config.Parity.
type Parity string

const (
	ParityNone Parity = "N" // No parity
	ParityEven Parity = "E" // Even parity
	ParityOdd  Parity = "O" // Odd parity
)

// ParseParity converts a string (e.g. from a CLI flag) to Parity. Accepts "N", "E", "O" (case-insensitive).
// Empty string returns ParityNone. Returns an error wrapping ErrInvalidConfig for other invalid input.
func ParseParity(s string) (Parity, error) {
	switch s {
	case "", "N", "n":
		return ParityNone, nil
	case "E", "e":
		return ParityEven, nil
	case "O", "o":
		return ParityOdd, nil
	default:
		return "", fmt.Errorf("serial: invalid parity %q: %w", s, ErrInvalidConfig)
	}
}

// String returns the parity character ("N", "E", "O").
func (p Parity) String() string {
	switch p {
	case ParityNone:
		return "N"
	case ParityEven:
		return "E"
	case ParityOdd:
		return "O"
	default:
		return string(p)
	}
}

// MarshalText implements encoding.TextMarshaler.
func (p Parity) MarshalText() ([]byte, error) {
	return []byte(p.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler. Accepts "N", "E", "O" (case-insensitive).
func (p *Parity) UnmarshalText(text []byte) error {
	if p == nil {
		return fmt.Errorf("serial: nil Parity receiver")
	}
	parsed, err := ParseParity(string(text))
	if err != nil {
		return err
	}
	*p = parsed
	return nil
}

// DataBits is the number of data bits per character (5, 6, 7, or 8).
type DataBits uint8

const (
	DataBits5 DataBits = 5
	DataBits6 DataBits = 6
	DataBits7 DataBits = 7
	DataBits8 DataBits = 8
)

// String returns the data bits as a decimal string (actual value; zero is "0").
func (d DataBits) String() string {
	return fmt.Sprint(uint8(d))
}

// MarshalText implements encoding.TextMarshaler.
func (d DataBits) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler. Accepts "5", "6", "7", "8".
func (d *DataBits) UnmarshalText(text []byte) error {
	if d == nil {
		return fmt.Errorf("serial: nil DataBits receiver")
	}
	n, err := strconv.ParseUint(string(text), 10, 8)
	if err != nil {
		return fmt.Errorf("serial: invalid data bits %q: %w", text, ErrInvalidConfig)
	}
	switch DataBits(n) {
	case DataBits5, DataBits6, DataBits7, DataBits8:
		*d = DataBits(n)
		return nil
	default:
		return fmt.Errorf("serial: data bits must be 5, 6, 7, or 8 (got %d): %w", n, ErrInvalidConfig)
	}
}

// StopBits is the number of stop bits (1 or 2).
type StopBits uint8

const (
	StopBits1 StopBits = 1
	StopBits2 StopBits = 2
)

// String returns the stop bits as a decimal string (actual value; zero is "0").
func (s StopBits) String() string {
	return fmt.Sprint(uint8(s))
}

// MarshalText implements encoding.TextMarshaler.
func (s StopBits) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler. Accepts "1" or "2".
func (s *StopBits) UnmarshalText(text []byte) error {
	if s == nil {
		return fmt.Errorf("serial: nil StopBits receiver")
	}
	n, err := strconv.ParseUint(string(text), 10, 8)
	if err != nil {
		return fmt.Errorf("serial: invalid stop bits %q: %w", text, ErrInvalidConfig)
	}
	switch StopBits(n) {
	case StopBits1, StopBits2:
		*s = StopBits(n)
		return nil
	default:
		return fmt.Errorf("serial: stop bits must be 1 or 2 (got %d): %w", n, ErrInvalidConfig)
	}
}

// ParseDataBits converts an int (e.g. from a CLI flag) to DataBits. Accepts 5, 6, 7, 8.
func ParseDataBits(n int) (DataBits, error) {
	switch DataBits(n) {
	case DataBits5, DataBits6, DataBits7, DataBits8:
		return DataBits(n), nil
	default:
		return 0, fmt.Errorf("serial: data bits must be 5, 6, 7, or 8 (got %d): %w", n, ErrInvalidConfig)
	}
}

// ParseStopBits converts an int (e.g. from a CLI flag) to StopBits. Accepts 1 or 2.
func ParseStopBits(n int) (StopBits, error) {
	switch StopBits(n) {
	case StopBits1, StopBits2:
		return StopBits(n), nil
	default:
		return 0, fmt.Errorf("serial: stop bits must be 1 or 2 (got %d): %w", n, ErrInvalidConfig)
	}
}

// BaudRate is the serial baud rate. Use constants for common rates; custom rates use BaudRate(n).
type BaudRate int

const (
	Baud1200   BaudRate = 1200
	Baud2400   BaudRate = 2400
	Baud4800   BaudRate = 4800
	Baud9600   BaudRate = 9600
	Baud19200  BaudRate = 19200
	Baud38400  BaudRate = 38400
	Baud57600  BaudRate = 57600
	Baud115200 BaudRate = 115200
)

// String returns the baud rate as a decimal string (actual value; zero is "0").
func (b BaudRate) String() string {
	return fmt.Sprint(int(b))
}

// MarshalText implements encoding.TextMarshaler.
func (b BaudRate) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler. Accepts a decimal baud rate (e.g. "9600").
// Platform-specific support is checked at Open.
func (b *BaudRate) UnmarshalText(text []byte) error {
	if b == nil {
		return fmt.Errorf("serial: nil BaudRate receiver")
	}
	n, err := strconv.ParseInt(string(text), 10, 0)
	if err != nil || n <= 0 {
		return fmt.Errorf("serial: invalid baud rate %q: %w", text, ErrInvalidConfig)
	}
	*b = BaudRate(n)
	return nil
}

// ParseBaudRate converts an int (e.g. from a CLI flag) to BaudRate. Accepts any positive value.
// Platform-specific support is checked at Open.
func ParseBaudRate(n int) (BaudRate, error) {
	if n <= 0 {
		return 0, fmt.Errorf("serial: baud rate must be positive (got %d): %w", n, ErrInvalidConfig)
	}
	return BaudRate(n), nil
}
