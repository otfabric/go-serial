package modbus

import (
	"fmt"
	"testing"
)

// ExampleDefaultRTUConfig demonstrates the Modbus RTU serial preset (19200 8E1).
func ExampleDefaultRTUConfig() {
	cfg := DefaultRTUConfig("/dev/ttyUSB0")
	fmt.Printf("BaudRate: %d, Parity: %s\n", cfg.BaudRate, cfg.Parity)
	// Output:
	// BaudRate: 19200, Parity: E
}

func TestDefaultModbusRTUConfigAlias(t *testing.T) {
	// DefaultModbusRTUConfig is an alias for DefaultRTUConfig.
	a := DefaultRTUConfig("/dev/tty0")
	b := DefaultModbusRTUConfig("/dev/tty0")
	if a != b {
		t.Error("DefaultModbusRTUConfig should equal DefaultRTUConfig")
	}
}

func TestDefaultRTUConfig(t *testing.T) {
	cfg := DefaultRTUConfig("/dev/ttyUSB0")
	if cfg.Address != "/dev/ttyUSB0" {
		t.Errorf("Address: got %q", cfg.Address)
	}
	if cfg.BaudRate != 19200 {
		t.Errorf("BaudRate: got %d, want 19200", cfg.BaudRate)
	}
	if cfg.DataBits != 8 {
		t.Errorf("DataBits: got %d, want 8", cfg.DataBits)
	}
	if cfg.StopBits != 1 {
		t.Errorf("StopBits: got %d, want 1", cfg.StopBits)
	}
	if cfg.Parity != "E" {
		t.Errorf("Parity: got %q, want E", cfg.Parity)
	}
}
