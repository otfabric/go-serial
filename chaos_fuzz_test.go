// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"testing"
	"time"
)

// FuzzParseParity: never panics; accepted input maps to a valid constant
// whose text form parses back to itself.
func FuzzParseParity(f *testing.F) {
	for _, s := range []string{"", "N", "n", "E", "e", "O", "o", "M", "NONE", "\x00", "é"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		p, err := ParseParity(s)
		if err != nil {
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("ParseParity(%q) error %v does not wrap ErrInvalidConfig", s, err)
			}
			return
		}
		if p != ParityNone && p != ParityEven && p != ParityOdd {
			t.Fatalf("ParseParity(%q) = %q, not a known constant", s, p)
		}
		var back Parity
		text, _ := p.MarshalText()
		if err := back.UnmarshalText(text); err != nil || back != p {
			t.Fatalf("round trip of %q: got %q, %v", p, back, err)
		}
	})
}

// FuzzParseFlowControl: never panics; accepted input maps to a valid
// constant whose text form parses back to itself, and anything else is
// refused with ErrInvalidConfig.
func FuzzParseFlowControl(f *testing.F) {
	for _, s := range []string{"", "none", "NONE", "rts-cts", "RTS-CTS", "dtr-dsr", "xon-xoff", "Xon-Xoff", "rtscts", "hardware", "\x00", "rts-cts\x00", "ı"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		fc, err := ParseFlowControl(s)
		if err != nil {
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("ParseFlowControl(%q) error %v does not wrap ErrInvalidConfig", s, err)
			}
			return
		}
		switch fc {
		case FlowNone, FlowRTSCTS, FlowDTRDSR, FlowXONXOFF:
		default:
			t.Fatalf("ParseFlowControl(%q) = %q, not a known constant", s, fc)
		}
		if err := (Config{Address: "x", FlowControl: fc}).Validate(); err != nil {
			t.Fatalf("ParseFlowControl(%q) = %q, which Validate refuses: %v", s, fc, err)
		}
		var back FlowControl
		text, _ := fc.MarshalText()
		if err := back.UnmarshalText(text); err != nil || back != fc {
			t.Fatalf("round trip of %q: got %q, %v", fc, back, err)
		}
	})
}

// FuzzTextRoundTrip: for every config type, UnmarshalText either rejects
// input with ErrInvalidConfig or yields a value that marshals back to text
// which unmarshals to the same value.
func FuzzTextRoundTrip(f *testing.F) {
	for _, s := range []string{"", "0", "1", "2", "5", "8", "9", "9600", "-1", "+8", "08", " 8", "1e3", "99999999999999999999", "N", "E", "none", "rts-cts", "XON-XOFF"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		check := func(name string, unmarshal func([]byte) error, roundTrip func() error) {
			if err := unmarshal([]byte(s)); err != nil {
				if !errors.Is(err, ErrInvalidConfig) {
					t.Fatalf("%s.UnmarshalText(%q) error %v does not wrap ErrInvalidConfig", name, s, err)
				}
				return
			}
			if err := roundTrip(); err != nil {
				t.Fatalf("%s round trip of %q: %v", name, s, err)
			}
		}
		var b BaudRate
		check("BaudRate", b.UnmarshalText, func() error {
			if b <= 0 {
				return errors.New("accepted non-positive baud")
			}
			txt, _ := b.MarshalText()
			var c BaudRate
			if err := c.UnmarshalText(txt); err != nil || c != b {
				return errors.New("mismatch")
			}
			return nil
		})
		var d DataBits
		check("DataBits", d.UnmarshalText, func() error {
			if _, err := ParseDataBits(int(d)); err != nil {
				return err
			}
			txt, _ := d.MarshalText()
			var e DataBits
			if err := e.UnmarshalText(txt); err != nil || e != d {
				return errors.New("mismatch")
			}
			return nil
		})
		var sb StopBits
		check("StopBits", sb.UnmarshalText, func() error {
			if _, err := ParseStopBits(int(sb)); err != nil {
				return err
			}
			txt, _ := sb.MarshalText()
			var e StopBits
			if err := e.UnmarshalText(txt); err != nil || e != sb {
				return errors.New("mismatch")
			}
			return nil
		})
		var p Parity
		check("Parity", p.UnmarshalText, func() error {
			txt, _ := p.MarshalText()
			var e Parity
			if err := e.UnmarshalText(txt); err != nil || e != p {
				return errors.New("mismatch")
			}
			return nil
		})
		var fc FlowControl
		check("FlowControl", fc.UnmarshalText, func() error {
			txt, _ := fc.MarshalText()
			var e FlowControl
			if err := e.UnmarshalText(txt); err != nil || e != fc {
				return errors.New("mismatch")
			}
			return nil
		})
	})
}

// FuzzConfigValidate: Validate never panics, every failure is a
// *ConfigError wrapping ErrInvalidConfig, and normalizing a valid config
// keeps it valid and fills every defaulted field.
func FuzzConfigValidate(f *testing.F) {
	f.Add("/dev/ttyS0", 9600, uint8(8), uint8(1), "N", "", int64(0), false, int64(0), int64(0))
	f.Add("", 0, uint8(0), uint8(0), "", "none", int64(-1), true, int64(-5), int64(3))
	f.Add("COM3", -1, uint8(9), uint8(3), "M", "RTS-CTS", int64(time.Second), true, int64(1), int64(-1))
	f.Add("/dev/ttyS0", 19200, uint8(8), uint8(1), "E", "rts-cts", int64(0), true, int64(0), int64(0))
	f.Add("/dev/ttyS0", 19200, uint8(8), uint8(1), "E", "xon-xoff", int64(0), true, int64(0), int64(0))
	f.Fuzz(func(t *testing.T, addr string, baud int, data, stop uint8, parity, flow string, timeout int64, rs485 bool, before, after int64) {
		c := Config{
			Address: addr, BaudRate: BaudRate(baud), DataBits: DataBits(data), StopBits: StopBits(stop),
			Parity: Parity(parity), FlowControl: FlowControl(flow), Timeout: time.Duration(timeout),
			RS485: RS485Config{Enabled: rs485, DelayRtsBeforeSend: time.Duration(before), DelayRtsAfterSend: time.Duration(after)},
		}
		err := c.Validate()
		if err != nil {
			var ce *ConfigError
			if !errors.As(err, &ce) || !errors.Is(err, ErrInvalidConfig) || ce.Error() == "" {
				t.Fatalf("Validate(%+v) = %v; want *ConfigError wrapping ErrInvalidConfig", c, err)
			}
			return
		}
		n := c.Normalized()
		if err := n.Validate(); err != nil {
			t.Fatalf("normalized valid config became invalid: %v", err)
		}
		if c.FlowControl == FlowRTSCTS && c.RS485.Enabled {
			t.Fatalf("Validate accepted RTS/CTS flow control with RS-485: %+v", c)
		}
		if n.DataBits == 0 || n.StopBits == 0 || n.Parity == "" || n.FlowControl == "" || (c.BaudRate == 0 && n.BaudRate == 0) {
			t.Fatalf("Normalized left defaults unset: %+v", n)
		}
	})
}
