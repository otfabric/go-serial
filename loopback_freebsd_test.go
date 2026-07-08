//go:build freebsd

// SPDX-License-Identifier: MIT

package serial

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func ptySlavePath(master *os.File) (string, error) {
	// FreeBSD exposes TIOCGPTN but not TIOCSPTLCK in Go's syscall package.
	var n int32
	if err := ioctlPtr(master.Fd(), syscall.TIOCGPTN, unsafe.Pointer(&n)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/dev/pts/%d", n), nil
}

func ioctlPtr(fd uintptr, req uint, arg unsafe.Pointer) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(req), uintptr(arg)); errno != 0 {
		return errno
	}
	return nil
}
