//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"math"
	"os"
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

func TestCloseUnblocksRead(t *testing.T) {
	p, _ := openTestPTY(t, NoTimeout)
	ch := make(chan ioResult, 1)
	go func() {
		n, err := p.Read(make([]byte, 16))
		ch <- ioResult{n, err}
	}()
	time.Sleep(50 * time.Millisecond) // let Read block in its wait
	closed := make(chan error, 1)
	go func() { closed <- p.Close() }()
	r := waitResult(t, ch, "Read")
	if r.n != 0 || !errors.Is(r.err, ErrClosed) {
		t.Fatalf("Read = %d, %v; want 0, ErrClosed", r.n, r.err)
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return")
	}
}

func TestCloseUnblocksWrite(t *testing.T) {
	p, _ := openTestPTY(t, NoTimeout)
	// The master never reads, so the pty fills and Write waits for room.
	ch := make(chan ioResult, 1)
	go func() {
		n, err := p.Write(make([]byte, 1<<20))
		ch <- ioResult{n, err}
	}()
	time.Sleep(100 * time.Millisecond)
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	r := waitResult(t, ch, "Write")
	if !errors.Is(r.err, ErrClosed) {
		t.Fatalf("Write = %d, %v; want ErrClosed", r.n, r.err)
	}
	if r.n < 0 || r.n >= 1<<20 {
		t.Fatalf("Write n = %d, want a partial count", r.n)
	}
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
	err := enableRS485(-1, &RS485Config{Enabled: true})
	if rs485Tiocs == 0 {
		if !errors.Is(err, ErrUnsupportedPlatform) {
			t.Fatalf("enableRS485 = %v, want ErrUnsupportedPlatform", err)
		}
		return
	}
	// On Linux the ioctl reaches the kernel: a closed descriptor fails
	// with EBADF, not with an unsupported-platform error.
	if !errors.Is(err, syscall.EBADF) {
		t.Fatalf("enableRS485 = %v, want EBADF", err)
	}
}
