//go:build darwin

// SPDX-License-Identifier: MIT

package serial

import (
	"fmt"
	"syscall"
	"time"
)

// waitFDOnce uses select(2) on macOS, whose poll(2) does not support
// tty devices. select cannot represent descriptors >= FD_SETSIZE in a
// syscall.FdSet, so those are rejected instead of letting the kernel read
// and write past the end of the set.
func waitFDOnce(fd int, write bool, d time.Duration) (bool, error) {
	if fd >= syscall.FD_SETSIZE {
		return false, fmt.Errorf("fd %d exceeds FD_SETSIZE (%d) for select: %w", fd, syscall.FD_SETSIZE, syscall.EINVAL)
	}
	var set syscall.FdSet
	fdset(fd, &set)
	rfds, wfds := &set, (*syscall.FdSet)(nil)
	if write {
		rfds, wfds = nil, &set
	}
	var tv *syscall.Timeval
	if d >= 0 {
		t := syscall.NsecToTimeval(d.Nanoseconds())
		tv = &t
	}
	if err := syscall.Select(fd+1, rfds, wfds, nil, tv); err != nil {
		return false, err
	}
	return fdisset(fd, &set), nil
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
