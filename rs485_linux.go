//go:build linux && !(mips || mipsle || mips64 || mips64le)

// SPDX-License-Identifier: MIT

package serial

// rs485Tiocs and rs485Tiocg are Linux's TIOCSRS485 and TIOCGRS485 request
// numbers.
const (
	rs485Tiocs = 0x542f
	rs485Tiocg = 0x542e
)
