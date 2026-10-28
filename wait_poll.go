//go:build linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"syscall"
	"time"
)

// pollFd mirrors struct pollfd, which has the same layout on Linux and the BSDs.
type pollFd struct {
	fd      int32
	events  int16
	revents int16
}

// poll(2) event bits; identical on Linux and the BSDs.
const (
	pollIn   = 0x1
	pollOut  = 0x4
	pollNval = 0x20
)

// waitFDOnce performs a single poll on fd. Unlike select(2), poll has
// no FD_SETSIZE limit, so ports opened at high descriptor numbers work.
// d < 0 waits indefinitely.
func waitFDOnce(fd int, write bool, d time.Duration) (bool, error) {
	pfd := pollFd{fd: int32(fd), events: pollIn}
	if write {
		pfd.events = pollOut
	}
	n, err := pollOnce(&pfd, d)
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	if pfd.revents&pollNval != 0 {
		return false, syscall.EBADF
	}
	// Ready, POLLHUP or POLLERR: let read(2)/write(2) report data, EOF or the error.
	return true, nil
}
