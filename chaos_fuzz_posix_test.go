//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"testing"
)

// FuzzTermiosAgreesWithValidate: whenever Validate and the platform baud
// table accept a config, building termios for it must succeed — the two
// layers of validation must never disagree. The one exception is DTR/DSR
// flow control, which Validate accepts and termios refuses with
// ErrUnsupportedPlatform. The termios built must carry the flow control
// asked for, as the read-back of VerifySettings sees it.
func FuzzTermiosAgreesWithValidate(f *testing.F) {
	f.Add(9600, uint8(8), uint8(1), "N", "")
	f.Add(0, uint8(0), uint8(0), "", "none")
	f.Add(115200, uint8(5), uint8(2), "O", "rts-cts")
	f.Add(12345, uint8(7), uint8(1), "E", "xon-xoff")
	f.Add(9600, uint8(8), uint8(1), "N", "dtr-dsr")
	f.Add(9600, uint8(8), uint8(1), "N", "hardware")
	f.Fuzz(func(t *testing.T, baud int, data, stop uint8, parity, flow string) {
		c := Config{Address: "/dev/null", BaudRate: BaudRate(baud), DataBits: DataBits(data), StopBits: StopBits(stop), Parity: Parity(parity), FlowControl: FlowControl(flow)}
		n := c.Normalized()
		tio, err := newTermios(&n)
		accepted := c.Validate() == nil && isBaudRateSupported(n.BaudRate)
		if accepted && n.FlowControl == FlowDTRDSR {
			if !errors.Is(err, ErrUnsupportedPlatform) {
				t.Fatalf("DTR/DSR config %+v: newTermios = %v; want ErrUnsupportedPlatform", c, err)
			}
			return
		}
		if accepted && err != nil {
			t.Fatalf("config %+v passes validation but newTermios fails: %v", c, err)
		}
		if err != nil {
			if !errors.Is(err, ErrInvalidConfig) && !errors.Is(err, ErrUnsupportedPlatform) {
				t.Fatalf("newTermios error %v wraps neither ErrInvalidConfig nor ErrUnsupportedPlatform", err)
			}
			return
		}
		if got := termiosFlow(tio); got != n.FlowControl {
			t.Fatalf("config %+v: termios carries flow control %s", c, got)
		}
	})
}
