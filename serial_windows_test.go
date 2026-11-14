//go:build windows

// SPDX-License-Identifier: MIT

package serial

import (
	"math"
	"strconv"
	"testing"
	"time"
	"unsafe"
)

func TestWindowsBaudRateValidationIsDriverDependent(t *testing.T) {
	// On Windows, any positive baud that fits a DWORD is accepted by this
	// library; the driver validates.
	if isBaudRateSupported == nil {
		t.Fatal("isBaudRateSupported should be set")
	}
	if !isBaudRateSupported(BaudRate(12345)) {
		t.Fatal("windows should accept positive baud rates for driver-level validation")
	}
	if strconv.IntSize == 64 {
		big := uint64(math.MaxUint32)
		if !isBaudRateSupported(BaudRate(big)) {
			t.Fatal("MaxUint32 fits the DCB")
		}
		if isBaudRateSupported(BaudRate(big + 1)) {
			t.Fatal("a baud rate above MaxUint32 does not fit the DCB")
		}
	}
}

func TestTimeoutMs(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want uint32
	}{
		{time.Nanosecond, 1},
		{500 * time.Microsecond, 1},
		{time.Millisecond, 1},
		{time.Millisecond + 1, 2},
		{1500 * time.Millisecond, 1500},
		{(1<<32 - 2) * time.Millisecond, 1<<32 - 2},
		{(1<<32 - 1) * time.Millisecond, 1<<32 - 2},
		{1 << 32 * time.Millisecond, 1<<32 - 2},
		{math.MaxInt64, 1<<32 - 2},
	} {
		if got := timeoutMs(tc.d); got != tc.want {
			t.Errorf("timeoutMs(%v) = %d, want %d", tc.d, got, tc.want)
		}
	}
}

func TestDCBLayout(t *testing.T) {
	if got := unsafe.Sizeof(c_DCB{}); got != 28 {
		t.Fatalf("sizeof(DCB) = %d, want 28", got)
	}
	if got := unsafe.Sizeof(c_COMMTIMEOUTS{}); got != 20 {
		t.Fatalf("sizeof(COMMTIMEOUTS) = %d, want 20", got)
	}
}
