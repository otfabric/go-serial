//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"syscall"
	"time"
)

// waitFD blocks until fd is ready for reading (write == false) or writing
// (write == true) — including hangup or error conditions, which the
// following read/write then reports — or until deadline passes. A zero
// deadline waits indefinitely. It returns ready == false with a nil error
// when the deadline passes.
//
// Interrupted waits (EINTR, e.g. from Go runtime preemption signals) are
// retried against the same deadline, so signals neither shorten nor extend
// the wait.
func waitFD(fd int, write bool, deadline time.Time) (ready bool, err error) {
	if fd < 0 {
		return false, syscall.EBADF
	}
	for {
		remaining := time.Duration(-1) // wait forever
		if !deadline.IsZero() {
			if remaining = time.Until(deadline); remaining < 0 {
				remaining = 0
			}
		}
		ready, err = waitFDOnce(fd, write, remaining)
		if err == syscall.EINTR {
			continue
		}
		return ready, err
	}
}
