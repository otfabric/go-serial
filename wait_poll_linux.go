//go:build linux

// SPDX-License-Identifier: MIT

package serial

import (
	"syscall"
	"time"
	"unsafe"
)

// maxPpollTimeout keeps the timespec seconds within int32, the width of
// tv_sec on 32-bit Linux; waitFD waits again until the real deadline.
const maxPpollTimeout = (1<<31 - 1) * time.Second

// pollOnce wraps ppoll(2); plain poll(2) is not available on every Linux
// architecture (e.g. arm64, riscv64, loong64).
func pollOnce(pfds []pollFd, d time.Duration) (int, error) {
	var ts *syscall.Timespec
	if d >= 0 {
		t := syscall.NsecToTimespec(min(d, maxPpollTimeout).Nanoseconds())
		ts = &t
	}
	n, _, errno := syscall.Syscall6(syscall.SYS_PPOLL,
		uintptr(unsafe.Pointer(&pfds[0])), uintptr(len(pfds)), uintptr(unsafe.Pointer(ts)), 0, 0, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(n), nil
}
