//go:build linux

// SPDX-License-Identifier: MIT

package serial

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func ptySlavePath(master *os.File) (string, error) {
	var lock uint32
	if err := ioctlPtr(master.Fd(), syscall.TIOCSPTLCK, unsafe.Pointer(&lock)); err != nil {
		return "", err
	}
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
