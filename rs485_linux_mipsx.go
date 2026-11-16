//go:build linux && (mips || mipsle || mips64 || mips64le)

// SPDX-License-Identifier: MIT

package serial

// rs485Tiocs is TIOCSRS485 on MIPS, which encodes the direction and size
// of struct serial_rs485 in the request number: _IOWR('T', 0x2F, 32 bytes).
// rs485Tiocg is TIOCGRS485: _IOR('T', 0x2E, 32 bytes).
const (
	rs485Tiocs = 0xc020542f
	rs485Tiocg = 0x4020542e
)
