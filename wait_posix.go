//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"syscall"
	"time"
)

// errWoken is returned by waitFD when the wake descriptor became readable:
// Close is in progress and the caller must give up the descriptor.
var errWoken = syscall.ECANCELED

// waitFD blocks until fd is ready for reading (write == false) or writing
// (write == true) — including hangup or error conditions, which the
// following read/write then reports — or until deadline passes. A zero
// deadline waits indefinitely. It returns ready == false with a nil error
// when the deadline passes, and errWoken when wake (if >= 0) becomes
// readable.
//
// Interrupted waits (EINTR, e.g. from Go runtime preemption signals) are
// retried against the same deadline, so signals neither shorten nor extend
// the wait. A backend may wait for less than the remaining time when the
// primitive cannot represent it; waitFD then waits again until the real
// deadline.
func waitFD(fd, wake int, write bool, deadline time.Time) (ready bool, err error) {
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
		ready, err = waitFDOnce(fd, wake, write, remaining)
		if err == syscall.EINTR {
			continue
		}
		if err == nil && !ready && remaining != 0 && (deadline.IsZero() || time.Now().Before(deadline)) {
			continue // the backend clamped the wait; the deadline is still ahead
		}
		return ready, err
	}
}
