//go:build windows

// SPDX-License-Identifier: MIT

package serial

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"syscall"
	"time"
	"unsafe"
)

type port struct {
	// gate counts the Read and Write calls in progress. Close shuts it,
	// cancels the calls with CancelIoEx until they have returned, and only
	// then restores the settings and closes the handle, so a handle is
	// never used after it was closed.
	gate   ioGate
	handle syscall.Handle
	// waitForData makes Read retry when ReadFile returns no bytes: set for
	// Timeout == 0, where Read waits until data arrives.
	waitForData bool
	// timedWrite is set for Timeout > 0, which is the write time-out too:
	// a WriteFile that comes back short then ran out of time.
	timedWrite bool

	oldDCB      c_DCB
	oldTimeouts c_COMMTIMEOUTS

	// lines holds the flow control and RS-485 settings that decide whether
	// SetRTS and SetDTR may drive their line.
	lines Config
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

// cancelEvery is how often Close cancels the I/O of the port again while a
// Read or Write has not returned. A call can register just before it starts
// ReadFile or WriteFile, after the first cancel, and a Write can wait for a
// Read on the synchronous handle and start its WriteFile once that Read was
// cancelled.
const cancelEvery = 10 * time.Millisecond

// Close cancels a Read or Write in progress, which returns ErrClosed, and
// waits until it has returned. It then restores the saved timeouts and DCB
// and closes the handle. It is safe to call more than once and from several
// goroutines: a second Close returns nil at once, also while the first one
// waits.
func (p *port) Close() error {
	if !p.gate.shut(func() { _ = cancelIoEx(p.handle, nil) }, cancelEvery) {
		return nil
	}
	if p.handle == syscall.InvalidHandle {
		return nil
	}
	var errs []error
	if err := SetCommTimeouts(p.handle, &p.oldTimeouts); err != nil {
		errs = append(errs, fmt.Errorf("serial: close: restore comm timeouts: %w", err))
	}
	if err := SetCommState(p.handle, &p.oldDCB); err != nil {
		errs = append(errs, fmt.Errorf("serial: close: restore comm state: %w", err))
	}
	if err := syscall.CloseHandle(p.handle); err != nil {
		errs = append(errs, fmt.Errorf("serial: close: %w", err))
	}
	p.handle = syscall.InvalidHandle
	return errors.Join(errs...)
}

// readFile is ReadFile, and cancelIoEx is CancelIoEx; tests replace them.
var (
	readFile   = syscall.ReadFile
	cancelIoEx = syscall.CancelIoEx
)

// Read reads from the serial port. It blocks until data is available or the
// configured timeout expires. It returns ErrClosed when Close ran before or
// during the call.
func (p *port) Read(b []byte) (n int, err error) {
	if err = p.gate.begin(); err != nil {
		return 0, err
	}
	defer p.gate.end()
	if len(b) == 0 {
		return 0, nil
	}
	b = clampIO(b)
	for {
		start := time.Now()
		var done uint32
		if err = readFile(p.handle, b, &done, nil); err != nil {
			if p.gate.isClosed() {
				return 0, ErrClosed // cancelled by Close
			}
			return 0, fmt.Errorf("serial: read: %w", err)
		}
		if done > 0 {
			return int(done), nil
		}
		if p.gate.isClosed() {
			return 0, ErrClosed
		}
		// With Timeout == 0, ReadFile gives up after ReadTotalTimeoutConstant
		// (about 49.7 days) and Read waits again. A zero-byte return before
		// that is the driver reporting that the device went away.
		if !p.waitForData || time.Since(start) < foreverWait-time.Minute {
			return 0, ErrTimeout
		}
	}
}

// writeFile is WriteFile; tests replace it.
var writeFile = syscall.WriteFile

// Write writes all of b, in several WriteFile calls when b is longer than
// one call takes. It returns n < len(b) only with an error: ErrTimeout when
// Config.Timeout > 0 and a call wrote fewer bytes than it was given in that
// time, an error wrapping io.ErrShortWrite when that happens with no
// time-out set, and ErrClosed when Close ran before or during the call.
func (p *port) Write(b []byte) (n int, err error) {
	if err = p.gate.begin(); err != nil {
		return 0, err
	}
	defer p.gate.end()
	n, err = writeChunks(b, maxIO, p.timedWrite, func(chunk []byte) (int, error) {
		if p.gate.isClosed() {
			return 0, ErrClosed
		}
		var done uint32
		if err := writeFile(p.handle, chunk, &done, nil); err != nil {
			return 0, err
		}
		return int(done), nil
	})
	if err != nil && p.gate.isClosed() {
		return n, ErrClosed // cancelled by Close, or Close ran between two pieces
	}
	return n, err
}

// foreverWait is how long one ReadFile waits with Timeout == 0.
const foreverWait = (c_MAXDWORD - 1) * time.Millisecond

// maxIO is the largest length ReadFile and WriteFile take (a DWORD).
var maxIO uint64 = math.MaxUint32

// clampIO limits b to what one ReadFile call can transfer; the caller sees
// a short count and reads again, as io.Reader allows.
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
		p.timedWrite = true
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
	if err := GetCommState(p.handle, &p.oldDCB); err != nil {
		return fmt.Errorf("serial: get comm state: %w", err)
	}
	dcb, err := newDCB(c, &p.oldDCB)
	if err != nil {
		return err
	}
	if err := SetCommState(p.handle, &dcb); err != nil {
		_ = SetCommState(p.handle, &p.oldDCB)
		return fmt.Errorf("serial: set comm state: %w", err)
	}
	p.lines = Config{FlowControl: c.FlowControl, RS485: RS485Config{Enabled: c.RS485.Enabled}}
	return nil
}

// newDCB returns the DCB for c. With XON/XOFF flow control it takes the
// thresholds from old, the driver's DCB, or defaults where old has none.
func newDCB(c *Config, old *c_DCB) (c_DCB, error) {
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
		return dcb, fmt.Errorf("serial: unsupported stop bits %d: %w", c.StopBits, ErrInvalidConfig)
	}
	switch c.Parity {
	case "", ParityNone:
		dcb.Parity = c_NOPARITY
	case ParityEven:
		dcb.Parity = c_EVENPARITY
	case ParityOdd:
		dcb.Parity = c_ODDPARITY
	default:
		return dcb, fmt.Errorf("serial: unsupported parity %q: %w", c.Parity, ErrInvalidConfig)
	}
	switch c.FlowControl {
	case "", FlowNone, FlowRTSCTS, FlowDTRDSR:
	case FlowXONXOFF:
		dcb.XonChar, dcb.XoffChar = int8(XON), int8(XOFF)
		dcb.XonLim, dcb.XoffLim = old.XonLim, old.XoffLim
		if dcb.XonLim == 0 {
			dcb.XonLim = dcbDefaultXonLimit
		}
		if dcb.XoffLim == 0 {
			dcb.XoffLim = dcbDefaultXoffLim
		}
	default:
		return dcb, fmt.Errorf("serial: unsupported flow control %q: %w", c.FlowControl, ErrInvalidConfig)
	}
	binary.LittleEndian.PutUint32(dcb.Pad_cgo_0[:], dcbFlags(c))
	return dcb, nil
}

// SetDTR asserts or clears Data Terminal Ready (EscapeCommFunction).
func (p *port) SetDTR(on bool) error {
	return p.setLine("DTR", false, on)
}

// SetRTS asserts or clears Request To Send (EscapeCommFunction).
func (p *port) SetRTS(on bool) error {
	return p.setLine("RTS", true, on)
}

func (p *port) setLine(name string, rts, on bool) error {
	if err := p.gate.begin(); err != nil {
		return err
	}
	defer p.gate.end()
	if err := lineInUse(&p.lines, rts); err != nil {
		return err
	}
	if err := escapeCommFunction(p.handle, commEscape(rts, on)); err != nil {
		return fmt.Errorf("serial: set %s: %w", name, err)
	}
	return nil
}

// ModemStatus reads CTS, DSR, RI and DCD (GetCommModemStatus).
func (p *port) ModemStatus() (ModemStatus, error) {
	if err := p.gate.begin(); err != nil {
		return ModemStatus{}, err
	}
	defer p.gate.end()
	var bits uint32
	if err := getCommModemStatus(p.handle, &bits); err != nil {
		return ModemStatus{}, fmt.Errorf("serial: modem status: %w", err)
	}
	return commModemStatus(bits), nil
}

// createFile opens name for reading and writing with CreateFile; tests
// replace it.
var createFile = func(name *uint16) (syscall.Handle, error) {
	return syscall.CreateFile(
		name,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0,                     // mode
		nil,                   // security
		syscall.OPEN_EXISTING, // create mode
		0,                     // attributes
		0)                     // templates
}

// newHandle opens the port. Address must be a device name, which is opened
// in the device namespace, unless UncheckedAddress is set.
func newHandle(c *Config) (handle syscall.Handle, err error) {
	address := c.Address
	if !c.UncheckedAddress {
		if address, err = windowsDeviceName(c.Address); err != nil {
			return 0, err
		}
	}
	// UTF16PtrFromString rejects addresses containing NUL instead of
	// panicking like the deprecated StringToUTF16Ptr.
	name, err := syscall.UTF16PtrFromString(address)
	if err != nil {
		return 0, fmt.Errorf("serial: open %q: %w", c.Address, err)
	}
	handle, err = createFile(name)
	if err != nil {
		return 0, fmt.Errorf("serial: open %s: %w", c.Address, err)
	}
	return handle, nil
}
