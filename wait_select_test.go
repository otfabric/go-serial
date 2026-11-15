//go:build darwin || openbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"syscall"
	"testing"
)

// TestSelectDescriptorRange: a descriptor or wake descriptor that does not
// fit a syscall.FdSet is refused with EINVAL before select is called; the
// largest one that fits is passed to select.
func TestSelectDescriptorRange(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fd, wake int
	}{
		{"fd at FD_SETSIZE", syscall.FD_SETSIZE, -1},
		{"fd above FD_SETSIZE", syscall.FD_SETSIZE + 1, -1},
		{"wake at FD_SETSIZE", closedFD(t), syscall.FD_SETSIZE},
	} {
		for _, write := range []bool{false, true} {
			ready, err := waitFDOnce(tc.fd, tc.wake, write, 0)
			if ready || !errors.Is(err, syscall.EINVAL) {
				t.Errorf("%s (write=%v): waitFDOnce = %v, %v; want false, EINVAL", tc.name, write, ready, err)
			}
		}
	}
	// Whether this descriptor is open depends on the process: select
	// answers ready, not ready or EBADF, never the range error.
	if _, err := waitFDOnce(syscall.FD_SETSIZE-1, -1, false, 0); errors.Is(err, syscall.EINVAL) {
		t.Errorf("fd FD_SETSIZE-1: %v, want it passed to select", err)
	}
}

// TestFdSetBits: fdset and fdisset address one bit per descriptor over the
// whole set.
func TestFdSetBits(t *testing.T) {
	fds := []int{0, 1, 31, 32, 33, 63, 64, syscall.FD_SETSIZE - 2, syscall.FD_SETSIZE - 1}
	for _, fd := range fds {
		var set syscall.FdSet
		fdset(fd, &set)
		ones := 0
		for i := 0; i < syscall.FD_SETSIZE; i++ {
			if fdisset(i, &set) {
				ones++
				if i != fd {
					t.Errorf("fdset(%d) set the bit of descriptor %d", fd, i)
				}
			}
		}
		if ones != 1 {
			t.Errorf("fdset(%d) set %d bits, want 1", fd, ones)
		}
	}
}
