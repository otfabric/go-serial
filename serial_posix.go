//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
	"unsafe"
)

// port implements Port interface.
type port struct {
	fd         int
	oldTermios *syscall.Termios

	timeout time.Duration
}

const (
	rs485Enabled      = 1 << 0
	rs485RTSOnSend    = 1 << 1
	rs485RTSAfterSend = 1 << 2
	rs485RXDuringTX   = 1 << 4
	rs485Tiocs        = 0x542f
)

// rs485IoctlOpts is used to configure RS485 options in the driver.
type rs485IoctlOpts struct {
	flags                 uint32
	delay_rts_before_send uint32
	delay_rts_after_send  uint32
	padding               [5]uint32
}

// newPort allocates a new serial port (internal use).
func newPort() *port {
	return &port{fd: -1}
}

func init() {
	openPort = func(c *Config) (Port, error) {
		p := newPort()
		if err := p.open(c); err != nil {
			return nil, err
		}
		return p, nil
	}
}

// open connects to the given serial port (internal use).
func (p *port) open(c *Config) (err error) {
	termios, err := newTermios(c)
	if err != nil {
		return
	}
	// See man termios(3).
	// O_NOCTTY: no controlling terminal.
	// O_NDELAY: no data carrier detect.
	p.fd, err = syscall.Open(c.Address, syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NDELAY|syscall.O_CLOEXEC, 0666)
	if err != nil {
		return fmt.Errorf("serial: open %s: %w", c.Address, err)
	}
	// Backup current termios to restore on closing.
	p.backupTermios()
	if err = p.setTermios(termios); err != nil {
		_ = syscall.Close(p.fd)
		p.fd = -1
		p.oldTermios = nil
		return err
	}
	if err = enableRS485(p.fd, &c.RS485); err != nil {
		p.close()
		return err
	}
	p.timeout = c.Timeout
	return
}

func (p *port) Close() (err error) {
	if p.fd == -1 {
		return nil
	}
	restoreErr := p.restoreTermios()
	closeErr := syscall.Close(p.fd)
	p.fd = -1
	p.oldTermios = nil
	return errors.Join(restoreErr, closeErr)
}

// close closes without joining errors (used when open fails mid-way).
func (p *port) close() {
	if p.fd == -1 {
		return
	}
	_ = p.restoreTermios()
	_ = syscall.Close(p.fd)
	p.fd = -1
	p.oldTermios = nil
}

// Read reads from the serial port. It blocks until data is available or the configured timeout expires.
func (p *port) Read(b []byte) (n int, err error) {
	fd := p.fd
	var deadline time.Time
	if p.timeout > 0 {
		deadline = time.Now().Add(p.timeout)
	}
	for {
		ready, werr := waitFD(fd, false, deadline)
		if werr != nil {
			return 0, fmt.Errorf("serial: wait for data: %w", werr)
		}
		if !ready {
			return 0, ErrTimeout
		}
		n, err = syscall.Read(fd, b)
		switch {
		case err == syscall.EAGAIN || err == syscall.EINTR:
			// Spurious wakeup or interrupted read: wait again within the
			// same deadline.
			continue
		case err != nil:
			// syscall.Read reports -1 on failure; io.Reader needs 0 <= n.
			return 0, fmt.Errorf("serial: read: %w", err)
		case n == 0 && len(b) > 0:
			// The device was reported readable but has no data: the
			// other end hung up (an unplugged USB adapter, a closed pty).
			return 0, io.EOF
		}
		return n, nil
	}
}

// Write writes all of b to the serial port, blocking while the driver's
// output buffer is full. The descriptor is non-blocking (opened with
// O_NDELAY), so EAGAIN is handled here by waiting for writability rather
// than surfacing as an error. It returns n < len(b) only with an error,
// and never a negative n.
func (p *port) Write(b []byte) (n int, err error) {
	fd := p.fd
	for n < len(b) {
		m, werr := syscall.Write(fd, b[n:])
		if m > 0 {
			n += m
		}
		switch {
		case werr == syscall.EAGAIN:
			if _, err = waitFD(fd, true, time.Time{}); err != nil {
				return n, fmt.Errorf("serial: write: %w", err)
			}
		case werr == syscall.EINTR:
		case werr != nil:
			return n, fmt.Errorf("serial: write: %w", werr)
		case m == 0:
			return n, fmt.Errorf("serial: write: %w", io.ErrShortWrite)
		}
	}
	return n, nil
}

func (p *port) setTermios(termios *syscall.Termios) (err error) {
	if err = tcsetattr(p.fd, termios); err != nil {
		err = fmt.Errorf("serial: set termios: %w", err)
	}
	return
}

// backupTermios saves current termios setting for restore on close.
func (p *port) backupTermios() {
	oldTermios := &syscall.Termios{}
	if err := tcgetattr(p.fd, oldTermios); err != nil {
		return // best-effort; skip restore on close
	}
	p.oldTermios = oldTermios
}

// restoreTermios restores backed up termios setting on close. Returns any error from tcsetattr.
func (p *port) restoreTermios() error {
	if p.oldTermios == nil {
		return nil
	}
	err := tcsetattr(p.fd, p.oldTermios)
	p.oldTermios = nil
	return err
}

// Helpers for termios

func newTermios(c *Config) (termios *syscall.Termios, err error) {
	termios = &syscall.Termios{}
	var flag termiosFlag
	// Baud rate (zero = generic default 9600)
	if c.BaudRate == 0 {
		flag = syscall.B9600
	} else {
		var ok bool
		flag, ok = baudRates[int(c.BaudRate)]
		if !ok {
			err = fmt.Errorf("serial: unsupported baud rate %d: %w", c.BaudRate, ErrInvalidConfig)
			return
		}
	}
	termios.Cflag |= flag
	// Input baud.
	cfSetIspeed(termios, flag)
	// Output baud.
	cfSetOspeed(termios, flag)
	// Character size.
	if c.DataBits == 0 {
		flag = syscall.CS8
	} else {
		var ok bool
		flag, ok = charSizes[int(c.DataBits)]
		if !ok {
			err = fmt.Errorf("serial: unsupported data bits %d: %w", c.DataBits, ErrInvalidConfig)
			return
		}
	}
	termios.Cflag |= flag
	// Stop bits
	switch c.StopBits {
	case 0, StopBits1:
		// Default is one stop bit.
	case StopBits2:
		termios.Cflag |= syscall.CSTOPB
	default:
		err = fmt.Errorf("serial: unsupported stop bits %d: %w", c.StopBits, ErrInvalidConfig)
		return
	}
	switch c.Parity {
	case "", ParityNone:
		// No parity (empty string = generic default)
	case ParityEven:
		termios.Cflag |= syscall.PARENB
		termios.Iflag |= syscall.INPCK
	case ParityOdd:
		// PARODD: Parity is odd.
		termios.Cflag |= syscall.PARODD
		termios.Cflag |= syscall.PARENB
		termios.Iflag |= syscall.INPCK
	default:
		err = fmt.Errorf("serial: unsupported parity %q: %w", c.Parity, ErrInvalidConfig)
		return
	}
	// Control modes.
	// CREAD: Enable receiver.
	// CLOCAL: Ignore control lines.
	termios.Cflag |= syscall.CREAD | syscall.CLOCAL
	// Special characters.
	// VMIN: Minimum number of characters for noncanonical read.
	// VTIME: Time in deciseconds for noncanonical read.
	// Both are unused as NDELAY is we utilized when opening device.
	return
}

// enableRS485 enables RS485 functionality of driver via an ioctl if the config says so.
func enableRS485(fd int, config *RS485Config) error {
	if !config.Enabled {
		return nil
	}
	rs485 := rs485IoctlOpts{
		rs485Enabled,
		uint32(config.DelayRtsBeforeSend / time.Millisecond),
		uint32(config.DelayRtsAfterSend / time.Millisecond),
		[5]uint32{0, 0, 0, 0, 0},
	}

	if config.RtsHighDuringSend {
		rs485.flags |= rs485RTSOnSend
	}
	if config.RtsHighAfterSend {
		rs485.flags |= rs485RTSAfterSend
	}
	if config.RxDuringTx {
		rs485.flags |= rs485RXDuringTX
	}

	r, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(rs485Tiocs),
		uintptr(unsafe.Pointer(&rs485)))
	if errno != 0 {
		return os.NewSyscallError("SYS_IOCTL (RS485)", errno)
	}
	if r != 0 {
		return errors.New("serial: RS485 ioctl failed")
	}
	return nil
}
