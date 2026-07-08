//go:build darwin

// SPDX-License-Identifier: MIT

package serial

// termiosFlag is the width of Termios.Cflag and baud/char-size constants on Darwin.
type termiosFlag = uint64
