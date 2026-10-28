//go:build freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"syscall"
	"time"
	"unsafe"
)

// pollOnce wraps poll(2). d < 0 waits indefinitely; positive sub-millisecond
// durations round up so a short timeout never turns into a busy loop.
func pollOnce(pfd *pollFd, d time.Duration) (int, error) {
	ms := -1
	if d >= 0 {
		ms = int((d + time.Millisecond - 1) / time.Millisecond)
	}
	n, _, errno := syscall.Syscall(syscall.SYS_POLL,
		uintptr(unsafe.Pointer(pfd)), 1, uintptr(ms))
	if errno != 0 {
		return 0, errno
	}
	return int(n), nil
}
