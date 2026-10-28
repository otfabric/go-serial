//go:build linux

// SPDX-License-Identifier: MIT

package serial

import (
	"syscall"
	"time"
	"unsafe"
)

// pollOnce wraps ppoll(2); plain poll(2) is not available on every Linux
// architecture (e.g. arm64, riscv64, loong64).
func pollOnce(pfd *pollFd, d time.Duration) (int, error) {
	var ts *syscall.Timespec
	if d >= 0 {
		t := syscall.NsecToTimespec(d.Nanoseconds())
		ts = &t
	}
	n, _, errno := syscall.Syscall6(syscall.SYS_PPOLL,
		uintptr(unsafe.Pointer(pfd)), 1, uintptr(unsafe.Pointer(ts)), 0, 0, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(n), nil
}
