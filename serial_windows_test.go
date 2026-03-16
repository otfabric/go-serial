//go:build windows

package serial

import "testing"

func TestWindowsBaudRateValidationIsDriverDependent(t *testing.T) {
	// On Windows, any positive baud is accepted by this library; the driver validates.
	if isBaudRateSupported == nil {
		t.Fatal("isBaudRateSupported should be set")
	}
	if !isBaudRateSupported(BaudRate(12345)) {
		t.Fatal("windows should accept positive baud rates for driver-level validation")
	}
}
