//go:build linux && !(mips || mipsle || mips64 || mips64le)

// SPDX-License-Identifier: MIT

package serial

// rs485Tiocs is Linux's TIOCSRS485 request number.
const rs485Tiocs = 0x542f
