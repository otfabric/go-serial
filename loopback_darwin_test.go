//go:build darwin

// SPDX-License-Identifier: MIT

package serial

import (
	"os"
	"syscall"
	"unsafe"
)

func ptySlavePath(master *os.File) (string, error) {
	if err := ioctl(master.Fd(), syscall.TIOCPTYUNLK, 0); err != nil {
		return "", err
	}
	if err := ioctl(master.Fd(), syscall.TIOCPTYGRANT, 0); err != nil {
		return "", err
	}
	var name [256]byte
	if err := ioctlPtr(master.Fd(), syscall.TIOCPTYGNAME, unsafe.Pointer(&name[0])); err != nil {
		return "", err
	}
	return cString(name[:]), nil
}

func ioctl(fd uintptr, req uint, arg uintptr) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(req), arg); errno != 0 {
		return errno
	}
	return nil
}

func ioctlPtr(fd uintptr, req uint, arg unsafe.Pointer) error {
	return ioctl(fd, req, uintptr(arg))
}

func cString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
