// SPDX-License-Identifier: MIT

// Package modbus provides Modbus RTU-oriented serial presets for use with the parent serial package.
// It does not implement Modbus framing, CRC, or protocol logic—use a dedicated Modbus library for that.
package modbus

import "github.com/otfabric/go-serial"

// DefaultRTUConfig returns a serial.Config with common Modbus RTU defaults:
// 19200 baud, 8 data bits, 1 stop bit, even parity.
// Use with serial.Open for Modbus RTU over serial.
func DefaultRTUConfig(address string) serial.Config {
	return serial.Config{
		Address:  address,
		BaudRate: serial.Baud19200,
		DataBits: serial.DataBits8,
		StopBits: serial.StopBits1,
		Parity:   serial.ParityEven,
	}
}

// DefaultModbusRTUConfig is an alias for DefaultRTUConfig for discoverability.
var DefaultModbusRTUConfig = DefaultRTUConfig
