// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"fmt"
)

// ExampleDefaultConfig demonstrates the generic serial preset (9600 8N1).
func ExampleDefaultConfig() {
	cfg := DefaultConfig("/dev/ttyUSB0")
	fmt.Printf("BaudRate: %d, Parity: %s\n", cfg.BaudRate, cfg.Parity)
	// Output:
	// BaudRate: 9600, Parity: N
}

// ExampleParseParity shows parsing a parity string (e.g. from a CLI flag).
func ExampleParseParity() {
	p, err := ParseParity("e")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(p)
	// Output: E
}

// ExampleParseDataBits shows parsing data bits (e.g. from a CLI flag).
func ExampleParseDataBits() {
	d, err := ParseDataBits(7)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(d)
	// Output: 7
}

// ExampleParseStopBits shows parsing stop bits (e.g. from a CLI flag).
func ExampleParseStopBits() {
	s, err := ParseStopBits(2)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(s)
	// Output: 2
}

// ExampleConfig_Validate shows validating config and using errors.As with ConfigError.
func ExampleConfig_Validate() {
	cfg := Config{Address: "/dev/tty0", DataBits: 9}
	err := cfg.Validate()
	if err == nil {
		fmt.Println("valid")
		return
	}
	var cfgErr *ConfigError
	if errors.As(err, &cfgErr) {
		fmt.Printf("invalid field=%s value=%v reason=%s\n", cfgErr.Field, cfgErr.Value, cfgErr.Reason)
	}
	// Output:
	// invalid field=DataBits value=9 reason=must be 5, 6, 7, or 8
}

// ExampleConfig_Normalized shows using Normalized() to inspect effective defaults.
func ExampleConfig_Normalized() {
	cfg := Config{Address: "/dev/tty0"}
	effective := cfg.Normalized()
	fmt.Printf("BaudRate=%d DataBits=%d\n", effective.BaudRate, effective.DataBits)
	// Output:
	// BaudRate=9600 DataBits=8
}

// ExampleErrTimeout demonstrates how to detect read/write timeout using errors.Is.
func ExampleErrTimeout() {
	// In real use, err would come from port.Read() or port.Write() when Config.Timeout is set.
	err := ErrTimeout
	if errors.Is(err, ErrTimeout) {
		fmt.Println("timeout")
	}
	// Output:
	// timeout
}
