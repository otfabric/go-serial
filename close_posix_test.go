//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"math"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// openTestPTY opens the slave of a fresh pseudo-terminal with go-serial.
func openTestPTY(t *testing.T, timeout time.Duration) (Port, *os.File) {
	t.Helper()
	master, slave, err := openPTY()
	if err != nil {
		skipNoPTY(t, err)
	}
	t.Cleanup(func() { _ = master.Close() })
	p, err := Open(&Config{Address: slave, BaudRate: 9600, Timeout: timeout})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p, master
}

// result of an I/O call made in a goroutine.
type ioResult struct {
	n   int
	err error
}

func waitResult(t *testing.T, ch <-chan ioResult, what string) ioResult {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not return", what)
		return ioResult{}
	}
}

// ioHooks lets a test follow one Read or Write on a port through port.hook.
type ioHooks struct {
	waiting chan struct{} // closed when the call is about to wait for the descriptor
	ending  chan struct{} // closed when the call is about to unregister
	resume  chan struct{} // the call unregisters once this is closed

	waitOnce, endOnce, resumeOnce sync.Once
}

// release lets the call that stopped in the hook unregister.
func (h *ioHooks) release() { h.resumeOnce.Do(func() { close(h.resume) }) }

// hookPort installs hooks on p that stop a Read or Write just before it
// unregisters, until release is called. The test's cleanup releases it too,
// so a failed test does not leave Close waiting.
func hookPort(t *testing.T, p Port) (*port, *ioHooks) {
	t.Helper()
	pp, ok := p.(*port)
	if !ok {
		t.Fatalf("Open returned %T, want *port", p)
	}
	h := &ioHooks{waiting: make(chan struct{}), ending: make(chan struct{}), resume: make(chan struct{})}
	pp.hook = func(e hookEvent) {
		switch e {
		case hookWaiting:
			h.waitOnce.Do(func() { close(h.waiting) })
		case hookEnding:
			h.endOnce.Do(func() { close(h.ending) })
			<-h.resume
		}
	}
	t.Cleanup(h.release)
	return pp, h
}

func waitEvent(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// assertCloseWaits checks the state of a port whose Close runs in another
// goroutine while a Read or Write (op) has been woken and has stopped in the
// hook, still registered: Close has not returned, the port still holds
// descriptor fd, and fd is open. A second Close returns nil without closing it.
func assertCloseWaits(t *testing.T, pp *port, fd int, closed <-chan error, op string) {
	t.Helper()
	// Close holds mu from its first statement and gives it up only while it
	// waits for the call, or when it returns: taking mu here observes one of
	// those two states and nothing in between.
	pp.mu.Lock()
	isClosed, active, cur := pp.closed, pp.active, pp.fd
	pp.mu.Unlock()
	if !isClosed || active != 1 || cur != fd {
		t.Fatalf("with %s still registered: closed=%v active=%d fd=%d; want true, 1, %d", op, isClosed, active, cur, fd)
	}
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		t.Fatalf("descriptor %d while %s still uses it: %v", fd, op, err)
	}
	select {
	case err := <-closed:
		t.Fatalf("Close returned (%v) while %s was still registered", err, op)
	default:
	}
	if err := pp.Close(); err != nil {
		t.Fatalf("second Close during %s: %v", op, err)
	}
	if err := syscall.Fstat(fd, &st); err != nil {
		t.Fatalf("descriptor %d after a second Close, with %s still registered: %v", fd, op, err)
	}
}

// assertClosed checks that the first Close returned nil and that the port
// gave up its descriptor.
func assertClosed(t *testing.T, pp *port, closed <-chan error) {
	t.Helper()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return")
	}
	pp.mu.Lock()
	fd, active := pp.fd, pp.active
	pp.mu.Unlock()
	if fd != -1 || active != 0 {
		t.Fatalf("after Close: fd=%d active=%d; want -1, 0", fd, active)
	}
}

// TestCloseUnblocksRead is row L3 of the lifecycle matrix: Close during a
// Read that waits for data. The Read returns ErrClosed, and Close keeps the
// descriptor open and does not return until the Read has unregistered.
func TestCloseUnblocksRead(t *testing.T) {
	p, _ := openTestPTY(t, NoTimeout)
	pp, h := hookPort(t, p)
	fd := pp.fd
	ch := make(chan ioResult, 1)
	go func() {
		n, err := p.Read(make([]byte, 16))
		ch <- ioResult{n, err}
	}()
	waitEvent(t, h.waiting, "Read to reach its wait")
	closed := make(chan error, 1)
	go func() { closed <- p.Close() }()
	// Close wakes the Read, which stops in the hook before it unregisters.
	waitEvent(t, h.ending, "Close to wake Read")
	assertCloseWaits(t, pp, fd, closed, "Read")
	h.release()
	r := waitResult(t, ch, "Read")
	if r.n != 0 || !errors.Is(r.err, ErrClosed) {
		t.Fatalf("Read = %d, %v; want 0, ErrClosed", r.n, r.err)
	}
	assertClosed(t, pp, closed)
}

// TestCloseUnblocksWrite is row L4: Close during a Write that waits for
// room in the output buffer. The Write returns the count it wrote with
// ErrClosed, and Close waits for it as for a Read.
func TestCloseUnblocksWrite(t *testing.T) {
	p, _ := openTestPTY(t, NoTimeout)
	pp, h := hookPort(t, p)
	fd := pp.fd
	// The master never reads, so the pty fills and Write waits for room.
	const size = 1 << 20
	ch := make(chan ioResult, 1)
	go func() {
		n, err := p.Write(make([]byte, size))
		ch <- ioResult{n, err}
	}()
	waitEvent(t, h.waiting, "Write to wait for room")
	closed := make(chan error, 1)
	go func() { closed <- p.Close() }()
	waitEvent(t, h.ending, "Close to wake Write")
	assertCloseWaits(t, pp, fd, closed, "Write")
	h.release()
	r := waitResult(t, ch, "Write")
	if !errors.Is(r.err, ErrClosed) {
		t.Fatalf("Write = %d, %v; want ErrClosed", r.n, r.err)
	}
	if r.n <= 0 || r.n >= size {
		t.Fatalf("Write n = %d, want a partial count", r.n)
	}
	assertClosed(t, pp, closed)
}

func TestIOAfterCloseReturnsErrClosed(t *testing.T) {
	p, _ := openTestPTY(t, 10*time.Millisecond)
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := p.Read(make([]byte, 1)); !errors.Is(err, ErrClosed) {
		t.Fatalf("Read after Close: %v, want ErrClosed", err)
	}
	if _, err := p.Write([]byte("x")); !errors.Is(err, ErrClosed) {
		t.Fatalf("Write after Close: %v, want ErrClosed", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestConcurrentClose(t *testing.T) {
	p, _ := openTestPTY(t, NoTimeout)
	errs := make(chan error, 4)
	for range 4 {
		go func() { errs <- p.Close() }()
	}
	for range 4 {
		if err := <-errs; err != nil {
			t.Fatalf("Close: %v", err)
		}
	}
}

// TestCloseDoesNotLeakDescriptors: the wake pipe is released with the port.
func TestCloseDoesNotLeakDescriptors(t *testing.T) {
	master, slave, err := openPTY()
	if err != nil {
		skipNoPTY(t, err)
	}
	defer func() { _ = master.Close() }()
	before := openFDs()
	for range 20 {
		p, err := Open(&Config{Address: slave, BaudRate: 9600})
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if err := p.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}
	if after := openFDs(); after > before {
		t.Fatalf("open descriptors grew from %d to %d", before, after)
	}
}

// TestReadVeryLongTimeoutWaits: a Timeout longer than the wait primitive
// can represent still waits for data instead of failing at once.
func TestReadVeryLongTimeoutWaits(t *testing.T) {
	for _, timeout := range []time.Duration{100000001 * time.Second, math.MaxInt64} {
		p, master := openTestPTY(t, timeout)
		ch := make(chan ioResult, 1)
		go func() {
			n, err := p.Read(make([]byte, 4))
			ch <- ioResult{n, err}
		}()
		select {
		case r := <-ch:
			t.Fatalf("Timeout %v: Read returned at once: %d, %v", timeout, r.n, r.err)
		case <-time.After(100 * time.Millisecond):
		}
		if _, err := master.Write([]byte("A")); err != nil {
			t.Fatal(err)
		}
		r := waitResult(t, ch, "Read")
		if r.n != 1 || r.err != nil {
			t.Fatalf("Timeout %v: Read = %d, %v; want 1, nil", timeout, r.n, r.err)
		}
		_ = p.Close()
	}
}

func TestRS485Ioctl(t *testing.T) {
	err := enableRS485(-1, &RS485Config{Enabled: true}, false)
	if rs485Tiocs == 0 {
		if !errors.Is(err, ErrUnsupportedPlatform) {
			t.Fatalf("enableRS485 = %v, want ErrUnsupportedPlatform", err)
		}
		return
	}
	// On Linux the ioctl reaches the kernel: a closed descriptor fails
	// with EBADF, not with an unsupported-platform error, and the error
	// names the operation.
	if !errors.Is(err, syscall.EBADF) {
		t.Fatalf("enableRS485 = %v, want EBADF", err)
	}
	if !strings.HasPrefix(err.Error(), "serial: enable RS485: ") {
		t.Fatalf("enableRS485 = %q, want the prefix \"serial: enable RS485: \"", err)
	}
}
