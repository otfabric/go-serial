// SPDX-License-Identifier: MIT

package serial

import (
	"fmt"
	"strings"
)

// FlowControl is the flow control of a port. Use the constants or ParseFlowControl when setting Config.FlowControl.
type FlowControl string

const (
	FlowNone    FlowControl = "none"     // No flow control
	FlowRTSCTS  FlowControl = "rts-cts"  // Hardware flow control on RTS and CTS
	FlowDTRDSR  FlowControl = "dtr-dsr"  // Hardware flow control on DTR and DSR (Windows only)
	FlowXONXOFF FlowControl = "xon-xoff" // Software flow control with XON (0x11) and XOFF (0x13)
)

// XON and XOFF are the characters of software flow control (DC1 and DC3).
const (
	XON  byte = 0x11
	XOFF byte = 0x13
)

// ParseFlowControl converts a string (e.g. from a CLI flag) to FlowControl. Accepts "none", "rts-cts", "dtr-dsr"
// and "xon-xoff" (case-insensitive). Empty string returns FlowNone. Returns an error wrapping ErrInvalidConfig
// for other input.
func ParseFlowControl(s string) (FlowControl, error) {
	switch f := FlowControl(strings.ToLower(s)); f {
	case "", FlowNone:
		return FlowNone, nil
	case FlowRTSCTS, FlowDTRDSR, FlowXONXOFF:
		return f, nil
	default:
		return "", fmt.Errorf("serial: invalid flow control %q: %w", s, ErrInvalidConfig)
	}
}

// String returns the flow control's name ("none", "rts-cts", "dtr-dsr", "xon-xoff").
func (f FlowControl) String() string {
	if f == "" {
		return string(FlowNone)
	}
	return string(f)
}

// MarshalText implements encoding.TextMarshaler.
func (f FlowControl) MarshalText() ([]byte, error) {
	return []byte(f.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler. Accepts what ParseFlowControl accepts.
func (f *FlowControl) UnmarshalText(text []byte) error {
	if f == nil {
		return fmt.Errorf("serial: nil FlowControl receiver")
	}
	parsed, err := ParseFlowControl(string(text))
	if err != nil {
		return err
	}
	*f = parsed
	return nil
}

// ModemStatus is the state of a port's modem status inputs: true is asserted.
type ModemStatus struct {
	CTS bool // Clear To Send
	DSR bool // Data Set Ready
	RI  bool // Ring Indicator
	DCD bool // Data Carrier Detect
}

// ModemLines sets a port's modem control outputs and reads its modem status inputs. Every Port that Open returns
// on Linux, macOS, the BSDs and Windows implements it:
//
//	if m, ok := p.(serial.ModemLines); ok {
//		err = m.SetDTR(true)
//	}
//
// A method on a closed port returns ErrClosed. SetRTS returns ErrLineInUse while Config.FlowControl is
// FlowRTSCTS or Config.RS485 is enabled, and SetDTR while Config.FlowControl is FlowDTRDSR: the driver drives
// that line then. A port without modem lines, such as a pseudo-terminal, returns the operating system's error,
// wrapped with the operation.
type ModemLines interface {
	// SetDTR asserts (true) or clears (false) Data Terminal Ready.
	SetDTR(on bool) error
	// SetRTS asserts (true) or clears (false) Request To Send.
	SetRTS(on bool) error
	// ModemStatus reads CTS, DSR, RI and DCD.
	ModemStatus() (ModemStatus, error)
}

// lineInUse returns ErrLineInUse, wrapped with the line, when c gives the line named by rts or dtr to flow
// control or RS-485.
func lineInUse(c *Config, rts bool) error {
	switch {
	case rts && c.FlowControl == FlowRTSCTS:
		return fmt.Errorf("serial: set RTS: flow control %s: %w", c.FlowControl, ErrLineInUse)
	case rts && c.RS485.Enabled:
		return fmt.Errorf("serial: set RTS: RS-485: %w", ErrLineInUse)
	case !rts && c.FlowControl == FlowDTRDSR:
		return fmt.Errorf("serial: set DTR: flow control %s: %w", c.FlowControl, ErrLineInUse)
	}
	return nil
}
