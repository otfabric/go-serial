//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"io"
	"os"
	"testing"
)

// setupLoopbackPair returns two serial device paths connected in a loopback pair.
// It prefers SERIAL_LOOPBACK_PTY1 and SERIAL_LOOPBACK_PTY2 when set (e.g. from socat).
func setupLoopbackPair(t *testing.T) (path1, path2 string, cleanup func()) {
	t.Helper()

	if p1, p2 := os.Getenv("SERIAL_LOOPBACK_PTY1"), os.Getenv("SERIAL_LOOPBACK_PTY2"); p1 != "" && p2 != "" {
		for _, p := range []string{p1, p2} {
			if _, err := os.Stat(p); err != nil {
				t.Skipf("loopback path %q unavailable: %v", p, err)
			}
		}
		return p1, p2, func() {}
	}

	m1, s1, err := openPTY()
	if err != nil {
		t.Skipf("skip when PTY cannot be created: %v", err)
	}
	m2, s2, err := openPTY()
	if err != nil {
		_ = m1.Close()
		t.Skipf("skip when PTY cannot be created: %v", err)
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
