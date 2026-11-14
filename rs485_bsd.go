//go:build darwin || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

// rs485Tiocs is zero where the kernel has no RS-485 ioctl: Open refuses
// Config.RS485.Enabled instead of sending Linux's request number.
const rs485Tiocs = 0
