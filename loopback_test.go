//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"
)

// setupLoopbackPair returns two serial device paths connected in a loopback pair.
// It prefers SERIAL_LOOPBACK_PTY1 and SERIAL_LOOPBACK_PTY2 when set (e.g. from socat).
func setupLoopbackPair(t *testing.T) (path1, path2 string, cleanup func()) {
	t.Helper()

	if p1, p2 := os.Getenv("SERIAL_LOOPBACK_PTY1"), os.Getenv("SERIAL_LOOPBACK_PTY2"); p1 != "" && p2 != "" {
		for _, p := range []string{p1, p2} {
			if _, err := os.Stat(p); err != nil {
				skipNoPTY(t, fmt.Errorf("loopback path %q unavailable: %w", p, err))
			}
		}
		return p1, p2, func() {}
	}

	m1, s1, err := openPTY()
	if err != nil {
		skipNoPTY(t, err)
	}
	m2, s2, err := openPTY()
	if err != nil {
		_ = m1.Close()
		skipNoPTY(t, err)
	}

	done1 := make(chan struct{})
	done2 := make(chan struct{})
	go relayPTY(m1, m2, done1)
	go relayPTY(m2, m1, done2)

	return s1, s2, func() {
		_ = m1.Close()
		_ = m2.Close()
		<-done1
		<-done2
	}
}

func relayPTY(dst, src *os.File, done chan struct{}) {
	_, _ = io.Copy(dst, src)
	close(done)
}

func openPTY() (master *os.File, slavePath string, err error) {
	master, err = os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, "", err
	}

	slavePath, err = ptySlavePath(master)
	if err != nil {
		_ = master.Close()
		return nil, "", err
	}
	return master, slavePath, nil
}

// TestReadAfterHangup: once the other end of the line goes away, Read
// reports an error (io.EOF or the OS error) with n == 0 instead of
// returning n == -1 or (0, nil) forever.
func TestReadAfterHangup(t *testing.T) {
	master, slave, err := openPTY()
	if err != nil {
		skipNoPTY(t, err)
	}
	p, err := Open(&Config{Address: slave, BaudRate: 115200, Timeout: 100 * time.Millisecond})
	if err != nil {
		_ = master.Close()
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	if _, err := master.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	n, err := p.Read(buf)
	if err != nil || n != 1 || buf[0] != 'x' {
		t.Fatalf("Read = %d, %v; want 1 byte x", n, err)
	}
	_ = master.Close()
	for i := 0; i < 50; i++ {
		n, err = p.Read(buf)
		if n < 0 {
			t.Fatalf("Read returned n = %d", n)
		}
		if err != nil && !errors.Is(err, ErrTimeout) {
			if n != 0 {
				t.Fatalf("Read = %d, %v after hangup", n, err)
			}
			return
		}
	}
	t.Fatalf("Read kept returning (%d, %v) after the other end hung up", n, err)
}

func TestReadWrite(t *testing.T) {
	pty1, pty2, cleanup := setupLoopbackPair(t)
	defer cleanup()

	config1 := Config{Address: pty1}
	port1, err := Open(&config1)
	if err != nil {
		skipNoPTY(t, err)
	}
	defer func() { _ = port1.Close() }()

	config2 := Config{
		Address:  pty2,
		BaudRate: Baud57600,
		DataBits: DataBits7,
		Parity:   ParityNone,
		StopBits: StopBits2,
	}
	port2, err := Open(&config2)
	if err != nil {
		skipNoPTY(t, err)
	}
	defer func() { _ = port2.Close() }()

	message := "test serial"
	n, err := port1.Write([]byte(message))
	if err != nil {
		t.Fatal(err)
	}
	if n != len(message) {
		t.Fatalf("unexpected write length %v", n)
	}
	var buf [16]byte
	n, err = port2.Read(buf[:])
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != message {
		t.Fatalf("unexpected response %q (len: %d)", buf[:n], n)
	}
}
