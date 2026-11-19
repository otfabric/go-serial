//go:build darwin || freebsd

// SPDX-License-Identifier: MIT

package serial

// crtscts is the termios CRTSCTS flag, CCTS_OFLOW | CRTS_IFLOW: flow
// control on RTS and CTS.
const crtscts = 0x00030000
