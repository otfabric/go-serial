//go:build darwin

// SPDX-License-Identifier: MIT

package serial

import (
	"fmt"
	"syscall"
	"time"
)

// maxSelectTimeout is the largest timeout select(2) accepts on macOS
// (tv_sec <= 100000000); longer waits are clamped and waitFD waits again
// until the real deadline.
const maxSelectTimeout = 100000000 * time.Second

// waitFDOnce uses select(2) on macOS, whose poll(2) does not support
// tty devices. select cannot represent descriptors >= FD_SETSIZE in a
// syscall.FdSet, so those are rejected instead of letting the kernel read
// and write past the end of the set. When wake >= 0 it is watched for
// reading as well.
func waitFDOnce(fd, wake int, write bool, d time.Duration) (bool, error) {
	if fd >= syscall.FD_SETSIZE || wake >= syscall.FD_SETSIZE {
		return false, fmt.Errorf("fd %d exceeds FD_SETSIZE (%d) for select: %w", max(fd, wake), syscall.FD_SETSIZE, syscall.EINVAL)
	}
	var rset, wset syscall.FdSet
	var rfds, wfds *syscall.FdSet
	if write {
		fdset(fd, &wset)
		wfds = &wset
	} else {
		fdset(fd, &rset)
		rfds = &rset
	}
	nfd := fd
	if wake >= 0 {
		fdset(wake, &rset)
		rfds = &rset
		nfd = max(fd, wake)
	}
	var tv *syscall.Timeval
	if d >= 0 {
		t := syscall.NsecToTimeval(min(d, maxSelectTimeout).Nanoseconds())
		tv = &t
	}
	if err := syscall.Select(nfd+1, rfds, wfds, nil, tv); err != nil {
		return false, err
	}
	if wake >= 0 && fdisset(wake, &rset) {
		return false, errWoken
	}
	if write {
		return fdisset(fd, &wset), nil
	}
	return fdisset(fd, &rset), nil
}

// fdget returns index and offset of fd in fds.
func fdget(fd int, fds *syscall.FdSet) (index, offset int) {
	bits := syscall.FD_SETSIZE / len(fds.Bits)
	return fd / bits, fd % bits
}

// fdset implements FD_SET macro.
func fdset(fd int, fds *syscall.FdSet) {
	idx, pos := fdget(fd, fds)
	fds.Bits[idx] |= 1 << uint(pos)
}

// fdisset implements FD_ISSET macro.
func fdisset(fd int, fds *syscall.FdSet) bool {
	idx, pos := fdget(fd, fds)
	return fds.Bits[idx]&(1<<uint(pos)) != 0
}
