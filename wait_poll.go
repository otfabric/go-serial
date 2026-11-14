//go:build linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"math"
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

// waitFDOnce performs a single poll on fd and, when wake >= 0, on the wake
// descriptor. Unlike select(2), poll has no FD_SETSIZE limit, so ports
// opened at high descriptor numbers work. d < 0 waits indefinitely.
func waitFDOnce(fd, wake int, write bool, d time.Duration) (bool, error) {
	if fd < 0 || fd > math.MaxInt32 || wake > math.MaxInt32 {
		return false, syscall.EBADF
	}
	pfds := [2]pollFd{{fd: int32(fd), events: pollIn}}
	if write {
		pfds[0].events = pollOut
	}
	nfds := 1
	if wake >= 0 {
		pfds[1] = pollFd{fd: int32(wake), events: pollIn}
		nfds = 2
	}
	n, err := pollOnce(pfds[:nfds], d)
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	if nfds == 2 && pfds[1].revents != 0 {
		return false, errWoken
	}
	if pfds[0].revents&pollNval != 0 {
		return false, syscall.EBADF
	}
	// Ready, POLLHUP or POLLERR: let read(2)/write(2) report data, EOF or the error.
	return pfds[0].revents != 0, nil
}
