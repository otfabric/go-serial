// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"testing"
)

func TestParseFlowControl(t *testing.T) {
	for in, want := range map[string]FlowControl{
		"": FlowNone, "none": FlowNone, "NONE": FlowNone, "rts-cts": FlowRTSCTS, "RTS-CTS": FlowRTSCTS,
		"dtr-dsr": FlowDTRDSR, "xon-xoff": FlowXONXOFF, "Xon-Xoff": FlowXONXOFF,
	} {
		got, err := ParseFlowControl(in)
		if err != nil || got != want {
			t.Errorf("ParseFlowControl(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"rtscts", "hardware", "x"} {
		if _, err := ParseFlowControl(in); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("ParseFlowControl(%q) = %v; want ErrInvalidConfig", in, err)
		}
	}
}

func TestFlowControlText(t *testing.T) {
	if s := FlowControl("").String(); s != "none" {
		t.Errorf(`FlowControl("").String() = %q`, s)
	}
	for _, f := range []FlowControl{FlowNone, FlowRTSCTS, FlowDTRDSR, FlowXONXOFF} {
		b, err := f.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var back FlowControl
		if err := back.UnmarshalText(b); err != nil || back != f {
			t.Errorf("%q: round trip gave %q, %v", f, back, err)
		}
	}
	var f FlowControl
	if err := f.UnmarshalText([]byte("bogus")); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("UnmarshalText(bogus) = %v", err)
	}
	var nilFlow *FlowControl
	if err := nilFlow.UnmarshalText([]byte("none")); err == nil {
		t.Error("UnmarshalText on a nil receiver returned nil")
	}
}

func TestValidateFlowControl(t *testing.T) {
	for _, f := range []FlowControl{"", FlowNone, FlowRTSCTS, FlowDTRDSR, FlowXONXOFF} {
		if err := (Config{Address: "x", FlowControl: f}).Validate(); err != nil {
			t.Errorf("%q: %v", f, err)
		}
	}
	var ce *ConfigError
	if err := (Config{Address: "x", FlowControl: "rtscts"}).Validate(); !errors.As(err, &ce) || ce.Field != "FlowControl" || !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("unknown flow control: %v", err)
	}
	err := Config{Address: "x", FlowControl: FlowRTSCTS, RS485: RS485Config{Enabled: true}}.Validate()
	if !errors.As(err, &ce) || ce.Field != "FlowControl" || ce.Reason != "RS-485 drives RTS" {
		t.Errorf("RTS/CTS with RS-485: %v", err)
	}
	if err := (Config{Address: "x", FlowControl: FlowXONXOFF, RS485: RS485Config{Enabled: true}}).Validate(); err != nil {
		t.Errorf("XON/XOFF with RS-485: %v", err)
	}
	if f := (Config{Address: "x"}).Normalized().FlowControl; f != FlowNone {
		t.Errorf("normalized flow control = %q", f)
	}
}

func TestLineInUse(t *testing.T) {
	for _, tc := range []struct {
		cfg      Config
		rts, dtr bool // in use
	}{
		{Config{}, false, false},
		{Config{FlowControl: FlowRTSCTS}, true, false},
		{Config{FlowControl: FlowDTRDSR}, false, true},
		{Config{FlowControl: FlowXONXOFF}, false, false},
		{Config{RS485: RS485Config{Enabled: true}}, true, false},
	} {
		if err := lineInUse(&tc.cfg, true); errors.Is(err, ErrLineInUse) != tc.rts {
			t.Errorf("%+v RTS: %v", tc.cfg, err)
		}
		if err := lineInUse(&tc.cfg, false); errors.Is(err, ErrLineInUse) != tc.dtr {
			t.Errorf("%+v DTR: %v", tc.cfg, err)
		}
	}
}

// TestWindowsCommMapping: the DCB bit field, the EscapeCommFunction
// functions and the GetCommModemStatus bits, against the values of the
// Win32 headers.
func TestWindowsCommMapping(t *testing.T) {
	for _, tc := range []struct {
		cfg  Config
		want uint32
	}{
		{Config{}, 0x0001},
		{Config{Parity: ParityEven}, 0x0003},
		{Config{Parity: ParityOdd, FlowControl: FlowNone}, 0x0003},
		{Config{FlowControl: FlowRTSCTS}, 0x0001 | 0x0004 | 0x2000},
		{Config{FlowControl: FlowDTRDSR}, 0x0001 | 0x0008 | 0x0020},
		{Config{FlowControl: FlowXONXOFF}, 0x0001 | 0x0100 | 0x0200},
	} {
		if got := dcbFlags(&tc.cfg); got != tc.want {
			t.Errorf("dcbFlags(%+v) = %#x; want %#x", tc.cfg, got, tc.want)
		}
	}
	for _, tc := range []struct {
		rts, on bool
		want    uint32
	}{{true, true, 3}, {true, false, 4}, {false, true, 5}, {false, false, 6}} {
		if got := commEscape(tc.rts, tc.on); got != tc.want {
			t.Errorf("commEscape(rts=%v, on=%v) = %d; want %d", tc.rts, tc.on, got, tc.want)
		}
	}
	for bits, want := range map[uint32]ModemStatus{
		0: {}, 0x10: {CTS: true}, 0x20: {DSR: true}, 0x40: {RI: true}, 0x80: {DCD: true},
		0xF0: {CTS: true, DSR: true, RI: true, DCD: true}, 0x0F: {},
	} {
		if got := commModemStatus(bits); got != want {
			t.Errorf("commModemStatus(%#x) = %+v; want %+v", bits, got, want)
		}
	}
}
