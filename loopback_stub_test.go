//go:build openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"os"
)

func ptySlavePath(_ *os.File) (string, error) {
	return "", errors.New("serial: programmatic PTY loopback is not supported on this platform")
}
