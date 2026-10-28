//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"testing"
)

// FuzzTermiosAgreesWithValidate: whenever Validate and the platform baud
// table accept a config, building termios for it must succeed — the two
// layers of validation must never disagree.
func FuzzTermiosAgreesWithValidate(f *testing.F) {
	f.Add(9600, uint8(8), uint8(1), "N")
	f.Add(0, uint8(0), uint8(0), "")
	f.Add(115200, uint8(5), uint8(2), "O")
	f.Add(12345, uint8(7), uint8(1), "E")
	f.Fuzz(func(t *testing.T, baud int, data, stop uint8, parity string) {
		c := Config{Address: "/dev/null", BaudRate: BaudRate(baud), DataBits: DataBits(data), StopBits: StopBits(stop), Parity: Parity(parity)}
		n := c.Normalized()
		_, err := newTermios(&n)
		accepted := c.Validate() == nil && isBaudRateSupported(n.BaudRate)
		if accepted && err != nil {
			t.Fatalf("config %+v passes validation but newTermios fails: %v", c, err)
		}
		if err != nil && !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("newTermios error %v does not wrap ErrInvalidConfig", err)
		}
	})
}
