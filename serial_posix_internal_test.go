//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestNewTermiosBaudLeavesModeBits checks that no baud rate changes the
// character size, stop bits or parity: on Darwin and the BSDs the speed
// constant is the numeric rate, and 9600 has the bits of CSTOPB and PARODD.
func TestNewTermiosBaudLeavesModeBits(t *testing.T) {
	const mode = uint32(syscall.CSIZE | syscall.CSTOPB | syscall.PARENB | syscall.PARODD)
	for rate := range baudRates {
		tio, err := newTermios(&Config{BaudRate: BaudRate(rate)})
		if err != nil {
			t.Fatalf("newTermios(%d): %v", rate, err)
		}
		if got := uint32(tio.Cflag) & mode; got != uint32(syscall.CS8) {
			t.Errorf("%d baud: mode bits of Cflag = %#x, want %#x", rate, got, uint32(syscall.CS8))
		}
	}
}

func TestNewTermiosOptions(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		wantCflag uint32
		wantIflag uint32
	}{
		{"defaults", Config{}, syscall.CS8 | syscall.CREAD | syscall.CLOCAL, 0},
		{"5 data bits", Config{DataBits: 5}, syscall.CS5, 0},
		{"6 data bits", Config{DataBits: 6}, syscall.CS6, 0},
		{"7 data bits", Config{DataBits: 7}, syscall.CS7, 0},
		{"2 stop bits", Config{StopBits: StopBits2}, syscall.CSTOPB, 0},
		{"1 stop bit", Config{StopBits: StopBits1}, syscall.CS8, 0},
		{"parity none", Config{Parity: ParityNone}, syscall.CS8, 0},
		{"parity even", Config{Parity: ParityEven}, syscall.PARENB, syscall.INPCK},
		{"parity odd", Config{Parity: ParityOdd}, syscall.PARENB | syscall.PARODD, syscall.INPCK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tio, err := newTermios(&tt.cfg)
			if err != nil {
				t.Fatalf("newTermios: %v", err)
			}
			if uint32(tio.Cflag)&tt.wantCflag != tt.wantCflag {
				t.Errorf("Cflag = %#x, want bits %#x", tio.Cflag, tt.wantCflag)
			}
			if uint32(tio.Iflag)&tt.wantIflag != tt.wantIflag {
				t.Errorf("Iflag = %#x, want bits %#x", tio.Iflag, tt.wantIflag)
			}
		})
	}

	odd, _ := newTermios(&Config{Parity: ParityOdd})
	even, _ := newTermios(&Config{Parity: ParityEven})
	if uint32(even.Cflag)&syscall.PARODD != 0 {
		t.Error("even parity must not set PARODD")
	}
	if uint32(odd.Cflag)&syscall.PARODD == 0 {
		t.Error("odd parity must set PARODD")
	}
}

func TestNewTermiosInvalid(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"baud", Config{BaudRate: 12345}},
		{"data bits", Config{DataBits: 9}},
		{"stop bits", Config{StopBits: StopBits(3)}},
		{"parity", Config{Parity: Parity("mark")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newTermios(&tt.cfg)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("want ErrInvalidConfig, got %v", err)
			}
		})
	}
}

func TestOpenNonexistentDevice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-tty")
	p, err := Open(&Config{Address: path, BaudRate: 9600})
	if err == nil {
		_ = p.Close()
		t.Fatal("expected error")
	}
	if !errors.Is(err, syscall.ENOENT) {
		t.Fatalf("want ENOENT, got %v", err)
	}
}

func TestOpenNotATerminal(t *testing.T) {
	// /dev/null opens fine but is not a tty: termios backup is skipped and
	// setting termios fails, so open must clean up and report the error.
	if _, err := os.Stat("/dev/null"); err != nil {
		t.Skip("/dev/null unavailable")
	}
	p := newPort()
	err := p.open(&Config{Address: "/dev/null", BaudRate: 9600})
	if err == nil {
		_ = p.Close()
		t.Fatal("expected error opening non-tty")
	}
	if p.fd != -1 || p.oldTermios != nil {
		t.Fatalf("port not reset after failed open: fd=%d oldTermios=%v", p.fd, p.oldTermios)
	}
}

// TestOpenNotACharacterDevice: a path that opens but is not a character
// device is refused before a terminal setting is read or changed. The error
// wraps ENOTTY and names the check, and the descriptor is released.
func TestOpenNotACharacterDevice(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{"regular file": file}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Logf("no FIFO case: mkfifo: %v", err)
	} else {
		paths["FIFO"] = fifo
	}
	for name, path := range paths {
		t.Run(name, func(t *testing.T) {
			before := openFDs()
			p := newPort()
			err := p.open(&Config{Address: path, BaudRate: 9600})
			if err == nil {
				_ = p.Close()
				t.Fatal("open succeeded")
			}
			if !errors.Is(err, syscall.ENOTTY) || !strings.Contains(err.Error(), "not a character device") {
				t.Fatalf("open = %v; want the character device check, wrapping ENOTTY", err)
			}
			if p.fd != -1 || p.oldTermios != nil {
				t.Fatalf("port not reset after failed open: fd=%d oldTermios=%v", p.fd, p.oldTermios)
			}
			if after := openFDs(); after > before {
				t.Fatalf("open descriptors grew from %d to %d", before, after)
			}
			if _, err := Open(&Config{Address: path}); !errors.Is(err, syscall.ENOTTY) || errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("Open = %v; want an error wrapping ENOTTY that is not ErrInvalidConfig", err)
			}
		})
	}
}

func TestCheckCharDevice(t *testing.T) {
	if err := checkCharDevice(closedFD(t)); !errors.Is(err, syscall.EBADF) {
		t.Fatalf("closed descriptor: %v, want EBADF", err)
	}
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip(err)
	}
	defer func() { _ = null.Close() }()
	if err := checkCharDevice(int(null.Fd())); err != nil {
		t.Fatalf("%s: %v, want nil", os.DevNull, err)
	}
}

func TestOpenRS485Unsupported(t *testing.T) {
	master, slave, err := openPTY()
	if err != nil {
		skipNoPTY(t, err)
	}
	defer func() { _ = master.Close() }()

	p := newPort()
	err = p.open(&Config{
		Address:  slave,
		BaudRate: 9600,
		RS485: RS485Config{
			Enabled:            true,
			DelayRtsBeforeSend: 2 * time.Millisecond,
			DelayRtsAfterSend:  3 * time.Millisecond,
			RtsHighDuringSend:  true,
			RtsHighAfterSend:   true,
			RxDuringTx:         true,
		},
	})
	if err == nil {
		// Some kernels may accept the ioctl on a pty; that is fine too.
		_ = p.Close()
		t.Skip("RS485 ioctl accepted on pty")
	}
	if p.fd != -1 || p.oldTermios != nil {
		t.Fatalf("port not reset after failed RS485 setup: fd=%d", p.fd)
	}
}

func TestEnableRS485Disabled(t *testing.T) {
	if err := enableRS485(-1, &RS485Config{}, true); err != nil {
		t.Fatalf("disabled RS485 should be a no-op, got %v", err)
	}
}

func TestCloseIdempotent(t *testing.T) {
	p := newPort()
	if err := p.Close(); err != nil {
		t.Fatalf("Close on unopened port: %v", err)
	}
	p.close() // must not panic

	master, slave, err := openPTY()
	if err != nil {
		skipNoPTY(t, err)
	}
	defer func() { _ = master.Close() }()
	port, err := Open(&Config{Address: slave, BaudRate: 9600})
	if err != nil {
		t.Fatal(err)
	}
	if err := port.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := port.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestCloseWithoutTermiosBackup(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip(err)
	}
	fd, err := syscall.Dup(int(f.Fd()))
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	p := &port{fd: fd}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestCloseReportsRestoreError(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip(err)
	}
	fd, err := syscall.Dup(int(f.Fd()))
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	// Restoring termios on a non-tty fails; Close must still close the fd
	// and surface the error, which names the operation and keeps the cause
	// (ENOTTY on Linux, ENODEV on macOS).
	cause := tcsetattr(fd, &syscall.Termios{})
	if cause == nil {
		t.Skipf("%s takes terminal settings", os.DevNull)
	}
	p := &port{fd: fd, oldTermios: &syscall.Termios{}}
	err = p.Close()
	if err == nil {
		t.Fatal("expected restore error")
	}
	if !errors.Is(err, cause) || !strings.HasPrefix(err.Error(), "serial: close: restore termios: ") {
		t.Fatalf("Close = %q; want the prefix \"serial: close: restore termios: \" and %v", err, cause)
	}
	if p.fd != -1 || p.oldTermios != nil {
		t.Fatal("port not reset after Close")
	}
}

// TestCloseReportsCloseError: an error of close(2) names the operation and
// keeps the cause.
func TestCloseReportsCloseError(t *testing.T) {
	p := &port{fd: closedFD(t)}
	err := p.Close()
	if !errors.Is(err, syscall.EBADF) || err.Error() != "serial: close: "+syscall.EBADF.Error() {
		t.Fatalf("Close = %v; want \"serial: close: %v\" wrapping EBADF", err, syscall.EBADF)
	}
	if p.fd != -1 {
		t.Fatalf("fd = %d after Close, want -1", p.fd)
	}
}

// closedFD returns a file descriptor number that is not open.
func closedFD(t *testing.T) int {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fd")
	if err != nil {
		t.Fatal(err)
	}
	fd, err := syscall.Dup(int(f.Fd()))
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	_ = syscall.Close(fd)
	return fd
}

func TestReadWaitError(t *testing.T) {
	p := &port{fd: closedFD(t), timeout: 10 * time.Millisecond}
	n, err := p.Read(make([]byte, 4))
	if err == nil || n != 0 {
		t.Fatalf("Read = %d, %v; want wait error", n, err)
	}
	if !errors.Is(err, syscall.EBADF) {
		t.Fatalf("want EBADF, got %v", err)
	}
}

func TestReadError(t *testing.T) {
	// A directory fd is reported readable by select, but read(2) fails
	// with EISDIR, exercising the read error path (n must be 0, not -1).
	fd, err := syscall.Open(t.TempDir(), syscall.O_RDONLY, 0)
	if err != nil {
		t.Skip(err)
	}
	defer func() { _ = syscall.Close(fd) }()
	p := &port{fd: fd} // no timeout: blocking select
	n, err := p.Read(make([]byte, 4))
	if err == nil {
		t.Fatal("expected read error")
	}
	if n != 0 {
		t.Fatalf("n = %d, want 0", n)
	}
}

func TestWriteError(t *testing.T) {
	p := &port{fd: closedFD(t)}
	if _, err := p.Write([]byte("x")); !errors.Is(err, syscall.EBADF) {
		t.Fatalf("want EBADF, got %v", err)
	}
}

func TestTermiosHelpersOnNonTTY(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip(err)
	}
	defer func() { _ = f.Close() }()
	p := &port{fd: int(f.Fd())}
	p.backupTermios()
	if p.oldTermios != nil {
		t.Fatal("backupTermios should skip on non-tty")
	}
	if err := p.restoreTermios(); err != nil {
		t.Fatalf("restoreTermios with no backup: %v", err)
	}
	if err := p.setTermios(&syscall.Termios{}); err == nil {
		t.Fatal("setTermios on non-tty should fail")
	}
}

// TestIOAfterCloseFailsFast: using a port after Close is a contract
// violation, but it must fail with EBADF rather than block forever.
func TestIOAfterCloseFailsFast(t *testing.T) {
	p := newPort() // fd == -1, as after Close
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := p.Read(make([]byte, 1)); !errors.Is(err, syscall.EBADF) {
			t.Errorf("Read after close: want EBADF, got %v", err)
		}
		if _, err := p.Write([]byte("x")); !errors.Is(err, syscall.EBADF) {
			t.Errorf("Write after close: want EBADF, got %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("I/O on a closed port blocked")
	}
}
