//go:build freebsd

// SPDX-License-Identifier: MIT

package serial

import "syscall"

// fdget returns index and offset of fd in fds.
func fdget(fd int, fds *syscall.FdSet) (index, offset int) {
	index = fd / (syscall.FD_SETSIZE / len(fds.X__fds_bits)) % len(fds.X__fds_bits)
	offset = fd % (syscall.FD_SETSIZE / len(fds.X__fds_bits))
	return
}

// fdset implements FD_SET macro.
func fdset(fd int, fds *syscall.FdSet) {
	idx, pos := fdget(fd, fds)
	fds.X__fds_bits[idx] = 1 << uint(pos)
}

// fdisset implements FD_ISSET macro.
func fdisset(fd int, fds *syscall.FdSet) bool {
	idx, pos := fdget(fd, fds)
	return fds.X__fds_bits[idx]&(1<<uint(pos)) != 0
}
