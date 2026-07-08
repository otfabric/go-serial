//go:build linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

// termiosFlag is the width of Termios.Cflag and baud/char-size constants on this platform.
type termiosFlag = uint32
