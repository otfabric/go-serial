// Package modbus provides Modbus RTU-oriented serial presets for use with the parent serial package.
// It does not implement Modbus framing, CRC, or protocol logic—use a dedicated Modbus library for that.
package modbus

import "github.com/gofabric/go-serial"

// DefaultRTUConfig returns a serial.Config with common Modbus RTU defaults:
// 19200 baud, 8 data bits, 1 stop bit, even parity.
// Use with serial.Open for Modbus RTU over serial.
func DefaultRTUConfig(address string) serial.Config {
	return serial.Config{
		Address:  address,
		BaudRate: 19200,
		DataBits: 8,
		StopBits: 1,
		Parity:   "E",
	}
}
