//go:build darwin

// SPDX-License-Identifier: MIT

package serial

import (
	"syscall"
)

func cfSetIspeed(termios *syscall.Termios, speed uint64) {
	termios.Ispeed = speed
}

func cfSetOspeed(termios *syscall.Termios, speed uint64) {
	termios.Ospeed = speed
}

// termiosSpeed returns the output speed of termios.
func termiosSpeed(termios *syscall.Termios) uint64 {
	return termios.Ospeed
}
