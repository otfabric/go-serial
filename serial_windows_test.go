//go:build windows

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"
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

// winResult is the result of a Read or Write made in a goroutine.
type winResult struct {
	n   int
	err error
}

// fakeCancelIO replaces ReadFile, WriteFile and CancelIoEx for one test. A
// ReadFile or WriteFile call blocks until CancelIoEx has been called need
// times, then fails with ERROR_OPERATION_ABORTED, as a call that CancelIoEx
// ended does. Run it inside a synctest bubble.
type fakeCancelIO struct {
	calls   atomic.Int32 // ReadFile and WriteFile calls
	inside  atomic.Int32 // calls that have not returned yet
	cancels atomic.Int32
	aborted chan struct{}
}

func installCancelIO(t *testing.T, need int32) *fakeCancelIO {
	t.Helper()
	f := &fakeCancelIO{aborted: make(chan struct{})}
	origRead, origWrite, origCancel := readFile, writeFile, cancelIoEx
	t.Cleanup(func() { readFile, writeFile, cancelIoEx = origRead, origWrite, origCancel })
	block := func(done *uint32) error {
		f.calls.Add(1)
		f.inside.Add(1)
		defer f.inside.Add(-1)
		<-f.aborted
		*done = 0
		return syscall.ERROR_OPERATION_ABORTED
	}
	readFile = func(_ syscall.Handle, _ []byte, done *uint32, _ *syscall.Overlapped) error { return block(done) }
	writeFile = func(_ syscall.Handle, _ []byte, done *uint32, _ *syscall.Overlapped) error { return block(done) }
	cancelIoEx = func(syscall.Handle, *syscall.Overlapped) error {
		if f.cancels.Add(1) == need {
			close(f.aborted)
		}
		return nil
	}
	return f
}

// TestCloseCancelsIO is row L3 and L4 of the lifecycle matrix on Windows:
// Close during a Read or a Write that waits in ReadFile or WriteFile
// cancels it with CancelIoEx, the call returns ErrClosed, and Close returns
// only after the call has. Later calls return ErrClosed without reaching
// the driver, and a second Close returns nil.
func TestCloseCancelsIO(t *testing.T) {
	for _, op := range []string{"Read", "Write"} {
		t.Run(op, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := installCancelIO(t, 1)
				p := &port{handle: syscall.InvalidHandle, waitForData: true}
				ch := make(chan winResult, 1)
				go func() {
					var r winResult
					if op == "Read" {
						r.n, r.err = p.Read(make([]byte, 8))
					} else {
						r.n, r.err = p.Write(make([]byte, 8))
					}
					ch <- r
				}()
				synctest.Wait() // the call waits in the driver
				if err := p.Close(); err != nil {
					t.Fatalf("Close: %v", err)
				}
				if n := f.inside.Load(); n != 0 {
					t.Fatalf("Close returned while %s was in the driver", op)
				}
				if r := <-ch; r.n != 0 || !errors.Is(r.err, ErrClosed) {
					t.Fatalf("%s = %d, %v; want 0, ErrClosed", op, r.n, r.err)
				}
				if n := f.cancels.Load(); n != 1 {
					t.Fatalf("CancelIoEx called %d times, want 1", n)
				}
				if _, err := p.Read(make([]byte, 1)); !errors.Is(err, ErrClosed) {
					t.Fatalf("Read after Close = %v, want ErrClosed", err)
				}
				if _, err := p.Read(nil); !errors.Is(err, ErrClosed) {
					t.Fatalf("Read of an empty buffer after Close = %v, want ErrClosed", err)
				}
				if _, err := p.Write([]byte("x")); !errors.Is(err, ErrClosed) {
					t.Fatalf("Write after Close = %v, want ErrClosed", err)
				}
				if n := f.calls.Load(); n != 1 {
					t.Fatalf("%d ReadFile and WriteFile calls, want 1", n)
				}
				if err := p.Close(); err != nil {
					t.Fatalf("second Close: %v", err)
				}
			})
		})
	}
}

// TestCloseCancelsAgain: a Read that the first CancelIoEx missed, because
// it entered ReadFile after it, is cancelled by a later one: Close cancels
// every cancelEvery until the call has returned.
func TestCloseCancelsAgain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := installCancelIO(t, 2)
		p := &port{handle: syscall.InvalidHandle, waitForData: true}
		ch := make(chan winResult, 1)
		go func() {
			n, err := p.Read(make([]byte, 8))
			ch <- winResult{n, err}
		}()
		synctest.Wait()
		start := time.Now()
		if err := p.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		if d := time.Since(start); d != cancelEvery {
			t.Fatalf("Close took %v, want %v", d, cancelEvery)
		}
		if n := f.inside.Load(); n != 0 {
			t.Fatal("Close returned while Read was in the driver")
		}
		if r := <-ch; !errors.Is(r.err, ErrClosed) {
			t.Fatalf("Read = %d, %v; want ErrClosed", r.n, r.err)
		}
		if n := f.cancels.Load(); n != 2 {
			t.Fatalf("CancelIoEx called %d times, want 2", n)
		}
	})
}

// TestCloseBetweenWritePieces: a Write that Close reaches between two
// WriteFile calls returns the count so far with ErrClosed and makes no
// further call.
func TestCloseBetweenWritePieces(t *testing.T) {
	orig := maxIO
	t.Cleanup(func() { maxIO = orig })
	maxIO = 4
	origWrite := writeFile
	t.Cleanup(func() { writeFile = origWrite })
	p := &port{handle: syscall.InvalidHandle}
	calls := 0
	writeFile = func(_ syscall.Handle, buf []byte, done *uint32, _ *syscall.Overlapped) error {
		calls++
		*done = uint32(len(buf))
		if calls == 1 {
			p.gate.mu.Lock()
			p.gate.closed = true // as Close does, while the call is registered
			p.gate.mu.Unlock()
		}
		return nil
	}
	n, err := p.Write(make([]byte, 10))
	if n != 4 || !errors.Is(err, ErrClosed) || calls != 1 {
		t.Fatalf("Write = %d, %v after %d WriteFile calls; want 4, ErrClosed after 1", n, err, calls)
	}
}

// TestReadErrorWhileOpen: a ReadFile error on an open port keeps the
// "serial: read: " prefix and its cause.
func TestReadErrorWhileOpen(t *testing.T) {
	orig := readFile
	t.Cleanup(func() { readFile = orig })
	readFile = func(syscall.Handle, []byte, *uint32, *syscall.Overlapped) error { return syscall.ERROR_ACCESS_DENIED }
	p := &port{handle: syscall.InvalidHandle}
	_, err := p.Read(make([]byte, 1))
	if !errors.Is(err, syscall.ERROR_ACCESS_DENIED) || !strings.HasPrefix(err.Error(), "serial: read: ") {
		t.Fatalf("Read = %v; want the prefix \"serial: read: \" and ERROR_ACCESS_DENIED", err)
	}
}

// utf16At returns the NUL-terminated UTF-16 string at p.
func utf16At(p *uint16) string {
	n := 0
	for *(*uint16)(unsafe.Add(unsafe.Pointer(p), 2*n)) != 0 {
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n))
}

// TestOpenAddress: Open takes a device name only and opens it in the device
// namespace; any other address is refused before CreateFile is called.
// With UncheckedAddress the address goes to CreateFile as given.
func TestOpenAddress(t *testing.T) {
	orig := createFile
	t.Cleanup(func() { createFile = orig })
	var names []string
	createFile = func(name *uint16) (syscall.Handle, error) {
		names = append(names, utf16At(name))
		return syscall.InvalidHandle, syscall.ERROR_FILE_NOT_FOUND
	}
	for _, address := range []string{`\\server\share\port`, `\\.\pipe\port`, `C:\port`, `port\x`, `\\?\GLOBALROOT\Device\Mup\server\share`} {
		_, err := Open(&Config{Address: address})
		var ce *ConfigError
		if !errors.Is(err, ErrInvalidConfig) || !errors.As(err, &ce) || ce.Field != "Address" {
			t.Errorf("Open(%q) = %v; want a *ConfigError for Address wrapping ErrInvalidConfig", address, err)
		}
	}
	if len(names) != 0 {
		t.Fatalf("CreateFile called for %q", names)
	}
	for _, tc := range []struct {
		cfg  Config
		name string
	}{
		{Config{Address: "COM10"}, `\\.\COM10`},
		{Config{Address: "COM3:"}, `\\.\COM3`},
		{Config{Address: `\\.\COM12`}, `\\.\COM12`},
		{Config{Address: `\\server\share\port`, UncheckedAddress: true}, `\\server\share\port`},
		{Config{Address: "COM3:", UncheckedAddress: true}, "COM3:"},
	} {
		names = nil
		_, err := Open(&tc.cfg)
		if !errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) || len(names) != 1 || names[0] != tc.name {
			t.Errorf("Open(%+v) = %v, CreateFile(%q); want ERROR_FILE_NOT_FOUND from CreateFile(%q)", tc.cfg, err, names, tc.name)
		}
	}
	names = nil
	if _, err := Open(&Config{Address: "COM1\x00", UncheckedAddress: true}); err == nil || len(names) != 0 {
		t.Fatalf("Open of an address with NUL = %v, CreateFile(%q); want an error before CreateFile", err, names)
	}
}

// TestNewDCBFlowControl: the DCB of each flow control, against the values
// of the Win32 headers. XON/XOFF takes the driver's thresholds, or defaults
// where it reports none.
func TestNewDCBFlowControl(t *testing.T) {
	flags := func(d c_DCB) uint32 {
		return uint32(d.Pad_cgo_0[0]) | uint32(d.Pad_cgo_0[1])<<8 | uint32(d.Pad_cgo_0[2])<<16 | uint32(d.Pad_cgo_0[3])<<24
	}
	old := c_DCB{XonLim: 100, XoffLim: 40}
	for _, tc := range []struct {
		cfg   Config
		flags uint32
		xon   bool
	}{
		{Config{BaudRate: 19200, Parity: ParityEven}, 0x0003, false},
		{Config{FlowControl: FlowRTSCTS}, 0x2005, false},
		{Config{FlowControl: FlowDTRDSR}, 0x0029, false},
		{Config{FlowControl: FlowXONXOFF}, 0x0301, true},
	} {
		d, err := newDCB(&tc.cfg, &old)
		if err != nil {
			t.Fatalf("%+v: %v", tc.cfg, err)
		}
		if got := flags(d); got != tc.flags {
			t.Errorf("%+v: flags %#x; want %#x", tc.cfg, got, tc.flags)
		}
		if tc.xon != (d.XonChar == 0x11 && d.XoffChar == 0x13 && d.XonLim == 100 && d.XoffLim == 40) {
			t.Errorf("%+v: XON/XOFF fields %+v", tc.cfg, d)
		}
	}
	d, err := newDCB(&Config{FlowControl: FlowXONXOFF}, &c_DCB{})
	if err != nil || d.XonLim != dcbDefaultXonLimit || d.XoffLim != dcbDefaultXoffLim {
		t.Errorf("XON/XOFF without driver thresholds: %+v, %v", d, err)
	}
	if _, err := newDCB(&Config{FlowControl: "bogus"}, &old); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("unknown flow control: %v", err)
	}
}

// TestModemLinesClosedWindows: the modem-line methods of a closed port
// return ErrClosed without a call to the driver, and a line that flow
// control drives is refused.
func TestModemLinesClosedWindows(t *testing.T) {
	p := newPort()
	p.lines = Config{FlowControl: FlowDTRDSR}
	if err := p.SetDTR(true); !errors.Is(err, ErrLineInUse) {
		t.Errorf("SetDTR under DTR/DSR: %v", err)
	}
	p.gate.shut(func() {}, time.Millisecond)
	// ErrClosed comes first, also for a line that flow control drives.
	if err := p.SetDTR(true); !errors.Is(err, ErrClosed) {
		t.Errorf("SetDTR under DTR/DSR after Close: %v", err)
	}
	if err := p.SetRTS(true); !errors.Is(err, ErrClosed) {
		t.Errorf("SetRTS after Close: %v", err)
	}
	if _, err := p.ModemStatus(); !errors.Is(err, ErrClosed) {
		t.Errorf("ModemStatus after Close: %v", err)
	}
}
