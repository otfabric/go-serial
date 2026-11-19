//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"fmt"
	"syscall"
	"unsafe"
)

// modemIoctl sends the modem-line ioctl req (TIOCMGET, TIOCMBIS or TIOCMBIC)
// with the line bits; tests replace it.
var modemIoctl = func(fd int, req uintptr, bits *int32) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(bits)))
	if errno != 0 {
		return errno
	}
	return nil
}

// SetDTR asserts or clears Data Terminal Ready (TIOCMBIS or TIOCMBIC).
func (p *port) SetDTR(on bool) error {
	return p.setLine("DTR", syscall.TIOCM_DTR, on)
}

// SetRTS asserts or clears Request To Send (TIOCMBIS or TIOCMBIC).
func (p *port) SetRTS(on bool) error {
	return p.setLine("RTS", syscall.TIOCM_RTS, on)
}

func (p *port) setLine(name string, bit int32, on bool) error {
	fd, _, err := p.begin()
	if err != nil {
		return err
	}
	defer p.end()
	if err := lineInUse(&p.lines, name == "RTS"); err != nil {
		return err
	}
	req := uintptr(syscall.TIOCMBIC)
	if on {
		req = syscall.TIOCMBIS
	}
	if err := modemIoctl(fd, req, &bit); err != nil {
		return fmt.Errorf("serial: set %s: %w", name, err)
	}
	return nil
}

// ModemStatus reads CTS, DSR, RI and DCD (TIOCMGET).
func (p *port) ModemStatus() (ModemStatus, error) {
	fd, _, err := p.begin()
	if err != nil {
		return ModemStatus{}, err
	}
	defer p.end()
	var bits int32
	if err := modemIoctl(fd, syscall.TIOCMGET, &bits); err != nil {
		return ModemStatus{}, fmt.Errorf("serial: modem status: %w", err)
	}
	return modemStatusOf(bits), nil
}

// modemStatusOf returns the status inputs set in the TIOCMGET bits.
func modemStatusOf(bits int32) ModemStatus {
	return ModemStatus{
		CTS: bits&syscall.TIOCM_CTS != 0,
		DSR: bits&syscall.TIOCM_DSR != 0,
		RI:  bits&syscall.TIOCM_RI != 0,
		DCD: bits&syscall.TIOCM_CD != 0,
	}
}
