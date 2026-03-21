//go:build darwin

package serial

// termiosFlag is the width of Termios.Cflag and baud/char-size constants on Darwin.
type termiosFlag = uint64
