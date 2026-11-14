//go:build windows

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type port struct {
	mu     sync.Mutex // guards handle against concurrent Close calls
	handle syscall.Handle
	// waitForData makes Read retry when ReadFile returns no bytes: set for
	// Timeout == 0, where Read waits until data arrives.
	waitForData bool

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
	// Windows: any positive baud that fits the DCB's 32-bit field is passed
	// to the driver; actual support is driver-dependent (unlike POSIX,
	// which uses fixed baud tables per platform).
	isBaudRateSupported = func(b BaudRate) bool { return b > 0 && uint64(b) <= math.MaxUint32 }
}

// open connects to the given serial port (internal use).
func (p *port) open(c *Config) (err error) {
	p.handle, err = newHandle(c)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			_ = syscall.CloseHandle(p.handle)
			p.handle = syscall.InvalidHandle
		}
	}()
	err = p.setSerialConfig(c)
	if err != nil {
		return
	}
	if err = p.setTimeouts(c); err != nil {
		_ = SetCommState(p.handle, &p.oldDCB)
	}
	return
}

func (p *port) Close() (err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
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
	if len(b) == 0 {
		return 0, nil
	}
	b = clampIO(b)
	for {
		start := time.Now()
		var done uint32
		if err = syscall.ReadFile(p.handle, b, &done, nil); err != nil {
			return 0, fmt.Errorf("serial: read: %w", err)
		}
		if done > 0 {
			return int(done), nil
		}
		// With Timeout == 0, ReadFile gives up after ReadTotalTimeoutConstant
		// (about 49.7 days) and Read waits again. A zero-byte return before
		// that is the driver reporting that the device went away.
		if !p.waitForData || time.Since(start) < foreverWait-time.Minute {
			return 0, ErrTimeout
		}
	}
}

// Write writes data to the serial port.
func (p *port) Write(b []byte) (n int, err error) {
	b = clampIO(b)
	var done uint32
	if err = syscall.WriteFile(p.handle, b, &done, nil); err != nil {
		return 0, fmt.Errorf("serial: write: %w", err)
	}
	return int(done), nil
}

// foreverWait is how long one ReadFile waits with Timeout == 0.
const foreverWait = (c_MAXDWORD - 1) * time.Millisecond

// maxIO is the largest length ReadFile and WriteFile take (a DWORD).
var maxIO uint64 = math.MaxUint32

// clampIO limits b to what one ReadFile or WriteFile call can transfer;
// the caller sees a short count and continues as io.Reader/io.Writer allow.
func clampIO(b []byte) []byte {
	if uint64(len(b)) > maxIO {
		return b[:maxIO]
	}
	return b
}

// timeoutMs converts a positive Timeout to COMMTIMEOUTS milliseconds:
// sub-millisecond values round up to 1 and the result stays below
// MAXDWORD, which has a special meaning for ReadTotalTimeoutConstant.
func timeoutMs(d time.Duration) uint32 {
	ms := d / time.Millisecond
	if d%time.Millisecond != 0 {
		ms++
	}
	return uint32(min(max(ms, 1), c_MAXDWORD-1))
}

func (p *port) setTimeouts(c *Config) error {
	// Read returns as soon as a byte arrived, or with zero bytes after
	// ReadTotalTimeoutConstant. Timeout == 0 uses the longest constant and
	// Read retries, so it returns the data that arrived instead of waiting
	// for a full buffer. Write has a timeout only when Timeout > 0.
	timeouts := c_COMMTIMEOUTS{
		ReadIntervalTimeout:        c_MAXDWORD,
		ReadTotalTimeoutMultiplier: c_MAXDWORD,
		ReadTotalTimeoutConstant:   c_MAXDWORD - 1,
	}
	if c.Timeout > 0 {
		timeout := timeoutMs(c.Timeout)
		timeouts.ReadTotalTimeoutConstant = timeout
		timeouts.WriteTotalTimeoutConstant = timeout
	} else {
		p.waitForData = true
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
	dcb := c_DCB{DCBlength: uint32(unsafe.Sizeof(c_DCB{}))}
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
	// UTF16PtrFromString rejects addresses containing NUL instead of
	// panicking like the deprecated StringToUTF16Ptr.
	name, err := syscall.UTF16PtrFromString(c.Address)
	if err != nil {
		return 0, fmt.Errorf("serial: open %q: %w", c.Address, err)
	}
	handle, err = syscall.CreateFile(
		name,
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
