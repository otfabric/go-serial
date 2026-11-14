//go:build freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"math"
	"syscall"
	"time"
	"unsafe"
)

// pollTimeoutMs converts d to poll(2)'s millisecond timeout, a C int.
// d < 0 waits indefinitely (-1); positive sub-millisecond durations round
// up so a short timeout never turns into a busy loop; durations beyond
// math.MaxInt32 ms are clamped, and waitFD waits again until the deadline.
func pollTimeoutMs(d time.Duration) int {
	if d < 0 {
		return -1
	}
	ms := d / time.Millisecond
	if d%time.Millisecond != 0 {
		ms++
	}
	if ms > math.MaxInt32 {
		ms = math.MaxInt32
	}
	return int(ms)
}

// pollOnce wraps poll(2).
func pollOnce(pfds []pollFd, d time.Duration) (int, error) {
	n, _, errno := syscall.Syscall(syscall.SYS_POLL,
		uintptr(unsafe.Pointer(&pfds[0])), uintptr(len(pfds)), uintptr(pollTimeoutMs(d)))
	if errno != 0 {
		return 0, errno
	}
	return int(n), nil
}
