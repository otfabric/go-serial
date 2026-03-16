/*
Package serial provides a cross-platform serial reader and writer.
*/
package serial

import (
	"errors"
	"io"
	"time"
)

var (
	// ErrTimeout is occurred when timing out.
	ErrTimeout = errors.New("serial: timeout")
)

// Config is common configuration for serial port.
type Config struct {
	// Device path (e.g. /dev/ttyUSB0 on Unix, COM1 on Windows).
	Address string
	// Baud rate (e.g. 9600, 115200). Zero uses the default from DefaultConfig.
	BaudRate int
	// Data bits: 5, 6, 7 or 8. Zero means 8.
	DataBits int
	// Stop bits: 1 or 2. Zero means 1.
	StopBits int
	// Parity: N - None, E - Even, O - Odd. Empty uses the default from DefaultConfig.
	Parity string
	// Read/Write timeout. Zero means no timeout.
	Timeout time.Duration
	// Configuration related to RS485 (optional).
	RS485 RS485Config
}

// DefaultConfig returns a generic serial config for the given address.
// Use this for protocol-neutral UART/RS-232/RS-485: 9600 8N1.
// For Modbus RTU defaults, use the serial/modbus package instead.
func DefaultConfig(address string) Config {
	return Config{
		Address:  address,
		BaudRate: 9600,
		DataBits: 8,
		StopBits: 1,
		Parity:   "N",
	}
}

// RS485Config is platform-independent RS485 options. Ignored unless Enabled is true.
type RS485Config struct {
	// Enable RS485 support
	Enabled bool
	// Delay RTS prior to send
	DelayRtsBeforeSend time.Duration
	// Delay RTS after send
	DelayRtsAfterSend time.Duration
	// Set RTS high during send
	RtsHighDuringSend bool
	// Set RTS high after send
	RtsHighAfterSend bool
	// Rx during Tx
	RxDuringTx bool
}

// Port is the interface for controlling serial port.
type Port interface {
	io.ReadWriteCloser
	// Connect connects to the serial port.
	Open(*Config) error
}

// Open opens a serial port.
func Open(c *Config) (p Port, err error) {
	p = New()
	err = p.Open(c)
	return
}
