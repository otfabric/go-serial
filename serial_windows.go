//go:build windows

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"fmt"
	"syscall"
)

type port struct {
	handle syscall.Handle

	oldDCB      c_DCB
	oldTimeouts c_COMMTIMEOUTS
}

// newPort allocates a new serial port (internal use).
func newPort() *port {
	return &port{handle: syscall.InvalidHandle}
}

func init() {
	openPort = func(c *Config) (Port, error) {
		p := newPort()
		if err := p.open(c); err != nil {
			return nil, err
		}
		return p, nil
	}
	// Windows: any positive baud is passed to the driver; actual support is driver-dependent
	// (unlike POSIX which uses fixed baud tables per platform).
	isBaudRateSupported = func(b BaudRate) bool { return b > 0 }
}

// open connects to the given serial port (internal use).
func (p *port) open(c *Config) (err error) {
	p.handle, err = newHandle(c)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			syscall.CloseHandle(p.handle)
			p.handle = syscall.InvalidHandle
		}
	}()
	err = p.setSerialConfig(c)
	if err != nil {
		return
	}
	err = p.setTimeouts(c)
	return
}

func (p *port) Close() (err error) {
	if p.handle == syscall.InvalidHandle {
		return nil
	}
	err1 := SetCommTimeouts(p.handle, &p.oldTimeouts)
	err2 := SetCommState(p.handle, &p.oldDCB)
	err3 := syscall.CloseHandle(p.handle)
	p.handle = syscall.InvalidHandle
	return errors.Join(err1, err2, err3)
}

// Read reads from the serial port. It blocks until data is available or the configured timeout expires.
func (p *port) Read(b []byte) (n int, err error) {
	var done uint32
	if err = syscall.ReadFile(p.handle, b, &done, nil); err != nil {
		return 0, fmt.Errorf("serial: read: %w", err)
	}
	if done == 0 {
		return 0, ErrTimeout
	}
	return int(done), nil
}

// Write writes data to the serial port.
func (p *port) Write(b []byte) (n int, err error) {
	var done uint32
	if err = syscall.WriteFile(p.handle, b, &done, nil); err != nil {
		return 0, fmt.Errorf("serial: write: %w", err)
	}
	return int(done), nil
}

func (p *port) setTimeouts(c *Config) error {
	var timeouts c_COMMTIMEOUTS
	// Read and write timeout
	if c.Timeout > 0 {
		timeout := toDWORD(int(c.Timeout.Nanoseconds() / 1e6))
		// wait until a byte arrived or time out
		timeouts.ReadIntervalTimeout = c_MAXDWORD
		timeouts.ReadTotalTimeoutMultiplier = c_MAXDWORD
		timeouts.ReadTotalTimeoutConstant = timeout
		timeouts.WriteTotalTimeoutConstant = timeout
	}
	if err := GetCommTimeouts(p.handle, &p.oldTimeouts); err != nil {
		return fmt.Errorf("serial: get comm timeouts: %w", err)
	}
	if err := SetCommTimeouts(p.handle, &timeouts); err != nil {
		_ = SetCommTimeouts(p.handle, &p.oldTimeouts)
		return fmt.Errorf("serial: set comm timeouts: %w", err)
	}
	return nil
}

func (p *port) setSerialConfig(c *Config) error {
	var dcb c_DCB
	if c.BaudRate == 0 {
		dcb.BaudRate = 9600
	} else {
		dcb.BaudRate = toDWORD(int(c.BaudRate))
	}
	if c.DataBits == 0 {
		dcb.ByteSize = 8
	} else {
		dcb.ByteSize = toBYTE(int(c.DataBits))
	}
	switch c.StopBits {
	case 0, StopBits1:
		dcb.StopBits = c_ONESTOPBIT
	case StopBits2:
		dcb.StopBits = c_TWOSTOPBITS
	default:
		return fmt.Errorf("serial: unsupported stop bits %d: %w", c.StopBits, ErrInvalidConfig)
	}
	switch c.Parity {
	case "", ParityNone:
		dcb.Parity = c_NOPARITY
	case ParityEven:
		dcb.Parity = c_EVENPARITY
		dcb.Pad_cgo_0[0] |= 0x02 // fParity
	case ParityOdd:
		dcb.Parity = c_ODDPARITY
		dcb.Pad_cgo_0[0] |= 0x02 // fParity
	default:
		return fmt.Errorf("serial: unsupported parity %q: %w", c.Parity, ErrInvalidConfig)
	}
	dcb.Pad_cgo_0[0] |= 0x01 // fBinary

	if err := GetCommState(p.handle, &p.oldDCB); err != nil {
		return fmt.Errorf("serial: get comm state: %w", err)
	}
	if err := SetCommState(p.handle, &dcb); err != nil {
		_ = SetCommState(p.handle, &p.oldDCB)
		return fmt.Errorf("serial: set comm state: %w", err)
	}
	return nil
}

func newHandle(c *Config) (handle syscall.Handle, err error) {
	handle, err = syscall.CreateFile(
		syscall.StringToUTF16Ptr(c.Address),
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0,                     // mode
		nil,                   // security
		syscall.OPEN_EXISTING, // create mode
		0,                     // attributes
		0)                     // templates
	if err != nil {
		return 0, fmt.Errorf("serial: open %s: %w", c.Address, err)
	}
	return handle, nil
}
