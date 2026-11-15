//go:build windows

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"syscall"
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

// fakeWriteFile replaces WriteFile for one test. Each call reports the next
// count of counts, or the whole buffer once counts is used up.
func fakeWriteFile(t *testing.T, counts ...uint32) *[]int {
	t.Helper()
	orig := writeFile
	t.Cleanup(func() { writeFile = orig })
	sizes := new([]int)
	writeFile = func(_ syscall.Handle, buf []byte, done *uint32, _ *syscall.Overlapped) error {
		*done = uint32(len(buf))
		if i := len(*sizes); i < len(counts) {
			*done = counts[i]
		}
		*sizes = append(*sizes, len(buf))
		return nil
	}
	return sizes
}

// TestWriteShortCount: a WriteFile that reports fewer bytes than it was
// given ends Write with that count and an error.
func TestWriteShortCount(t *testing.T) {
	fakeWriteFile(t, 4, 4)
	p := &port{handle: syscall.InvalidHandle, timedWrite: true}
	if n, err := p.Write(make([]byte, 8)); n != 4 || !errors.Is(err, ErrTimeout) {
		t.Fatalf("with a time-out: Write = %d, %v; want 4, ErrTimeout", n, err)
	}
	p.timedWrite = false
	if n, err := p.Write(make([]byte, 8)); n != 4 || !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("without a time-out: Write = %d, %v; want 4, io.ErrShortWrite", n, err)
	}
}

// TestWriteCountOutOfRange: a count that WriteFile cannot have written is
// an error, not a count of Write. MaxUint32 is above the buffer where int
// has 64 bits and negative where it has 32.
func TestWriteCountOutOfRange(t *testing.T) {
	fakeWriteFile(t, math.MaxUint32)
	p := &port{handle: syscall.InvalidHandle, timedWrite: true}
	n, err := p.Write(make([]byte, 8))
	if n != 0 || err == nil || errors.Is(err, ErrTimeout) {
		t.Fatalf("Write = %d, %v; want 0 and an error other than ErrTimeout", n, err)
	}
}

// TestWriteLongBuffer: a buffer longer than one WriteFile call takes is
// written in several calls.
func TestWriteLongBuffer(t *testing.T) {
	orig := maxIO
	t.Cleanup(func() { maxIO = orig })
	maxIO = 4
	sizes := fakeWriteFile(t)
	p := &port{handle: syscall.InvalidHandle}
	if n, err := p.Write(make([]byte, 10)); n != 10 || err != nil {
		t.Fatalf("Write = %d, %v; want 10, nil", n, err)
	}
	if got := *sizes; len(got) != 3 || got[0] != 4 || got[1] != 4 || got[2] != 2 {
		t.Fatalf("WriteFile calls of %v bytes, want [4 4 2]", got)
	}
}

// TestWriteError: an error of WriteFile is returned with the "serial: write"
// prefix and a zero count, whatever count the call left behind.
func TestWriteError(t *testing.T) {
	orig := writeFile
	t.Cleanup(func() { writeFile = orig })
	writeFile = func(_ syscall.Handle, _ []byte, done *uint32, _ *syscall.Overlapped) error {
		*done = 1
		return syscall.ERROR_ACCESS_DENIED
	}
	p := &port{handle: syscall.InvalidHandle, timedWrite: true}
	n, err := p.Write([]byte("xy"))
	if n != 0 || !errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
		t.Fatalf("Write = %d, %v; want 0 and ERROR_ACCESS_DENIED", n, err)
	}
	if !strings.HasPrefix(err.Error(), "serial: write: ") {
		t.Fatalf("error text %q, want the prefix \"serial: write: \"", err)
	}
}
