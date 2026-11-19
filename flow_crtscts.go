//go:build netbsd || openbsd

// SPDX-License-Identifier: MIT

package serial

// crtscts is the termios CRTSCTS flag: flow control on RTS and CTS.
const crtscts = 0x00010000
