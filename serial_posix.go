//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// port implements Port interface.
type port struct {
	fd         int
	oldTermios *syscall.Termios

	timeout time.Duration

	// lines holds the flow control and RS-485 settings that decide whether
	// SetRTS and SetDTR may drive their line.
	lines Config

	// mu guards closed, active and wake against Close. Read and Write
	// register as active while they use fd; Close marks the port closed,
	// wakes their waits through the wake pipe and closes fd only once no
	// call uses it any more, so a descriptor number is never used after
	// it was closed.
	mu     sync.Mutex
	idle   *sync.Cond // signalled when active drops to zero
	closed bool
	active int
	wake   *wakePipe // nil for ports built directly in tests

	// hook is nil outside tests. Read and Write call it before each wait
	// for the descriptor (hookWaiting) and before they unregister
	// (hookEnding); in both cases the call still counts as active.
	hook func(hookEvent)
}

// hookEvent names the point of a Read or Write at which port.hook is called.
type hookEvent int

const (
	hookWaiting hookEvent = iota // about to wait for the descriptor
	hookEnding                   // about to unregister with end
)

// event reports e to the test hook, if one is set.
func (p *port) event(e hookEvent) {
	if p.hook != nil {
		p.hook(e)
	}
}

// wakePipe lets Close interrupt a Read or Write that waits in poll/select.
type wakePipe struct{ r, w int }

func newWakePipe() (*wakePipe, error) {
	var fds [2]int
	syscall.ForkLock.RLock()
	err := syscall.Pipe(fds[:])
	if err == nil {
		syscall.CloseOnExec(fds[0])
		syscall.CloseOnExec(fds[1])
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		return nil, err
	}
	if err = syscall.SetNonblock(fds[1], true); err != nil {
		_ = syscall.Close(fds[0])
		_ = syscall.Close(fds[1])
		return nil, err
	}
	return &wakePipe{r: fds[0], w: fds[1]}, nil
}

func (w *wakePipe) signal() { _, _ = syscall.Write(w.w, []byte{0}) }

func (w *wakePipe) close() {
	_ = syscall.Close(w.r)
	_ = syscall.Close(w.w)
}

const (
	rs485Enabled      = 1 << 0
	rs485RTSOnSend    = 1 << 1
	rs485RTSAfterSend = 1 << 2
	rs485RXDuringTX   = 1 << 4
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
	// A serial port is a character device. Anything else is refused here,
	// before a terminal setting is read or changed.
	if err = checkCharDevice(p.fd); err != nil {
		_ = syscall.Close(p.fd)
		p.fd = -1
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
	if c.VerifySettings {
		if err = p.checkTermios(c, termios); err != nil {
			p.close()
			return err
		}
	}
	if err = enableRS485(p.fd, &c.RS485, c.VerifySettings); err != nil {
		p.close()
		return err
	}
	if p.wake, err = newWakePipe(); err != nil {
		p.close()
		return fmt.Errorf("serial: open %s: %w", c.Address, err)
	}
	p.timeout = c.Timeout
	p.lines = Config{FlowControl: c.FlowControl, RS485: RS485Config{Enabled: c.RS485.Enabled}}
	return
}

// checkCharDevice reports whether fd refers to a character device. For any
// other file type the error wraps ENOTTY, the error setting termios on such
// a file gives.
func checkCharDevice(fd int) error {
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		return err
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFCHR {
		return fmt.Errorf("not a character device: %w", syscall.ENOTTY)
	}
	return nil
}

// Close restores the saved termios settings and closes the descriptor. A
// Read or Write blocked in another goroutine returns ErrClosed; Close waits
// for it to give up the descriptor before closing it.
func (p *port) Close() (err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.fd == -1 {
		p.closed = true
		return nil
	}
	p.closed = true
	if p.wake != nil {
		p.wake.signal()
	}
	for p.active > 0 {
		if p.idle == nil {
			p.idle = sync.NewCond(&p.mu)
		}
		p.idle.Wait()
	}
	restoreErr := p.restoreTermios()
	if restoreErr != nil {
		restoreErr = fmt.Errorf("serial: close: restore termios: %w", restoreErr)
	}
	closeErr := syscall.Close(p.fd)
	if closeErr != nil {
		closeErr = fmt.Errorf("serial: close: %w", closeErr)
	}
	p.fd = -1
	p.oldTermios = nil
	if p.wake != nil {
		p.wake.close()
		p.wake = nil
	}
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

// begin registers an I/O call. It returns the descriptor to use and the
// wake descriptor (-1 if none), or ErrClosed once Close has started.
func (p *port) begin() (fd, wake int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return -1, -1, ErrClosed
	}
	p.active++
	wake = -1
	if p.wake != nil {
		wake = p.wake.r
	}
	return p.fd, wake, nil
}

// end unregisters an I/O call started with begin.
func (p *port) end() {
	p.event(hookEnding)
	p.mu.Lock()
	p.active--
	if p.active == 0 && p.idle != nil {
		p.idle.Broadcast()
	}
	p.mu.Unlock()
}

// Read reads from the serial port. It blocks until data is available or the configured timeout expires.
func (p *port) Read(b []byte) (n int, err error) {
	fd, wake, err := p.begin()
	if err != nil {
		return 0, err
	}
	defer p.end()
	var deadline time.Time
	if p.timeout > 0 {
		deadline = time.Now().Add(p.timeout)
	}
	for {
		p.event(hookWaiting)
		ready, werr := waitFD(fd, wake, false, deadline)
		if werr == errWoken {
			return 0, ErrClosed
		}
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
	if len(b) == 0 {
		return 0, nil
	}
	fd, wake, err := p.begin()
	if err != nil {
		return 0, err
	}
	defer p.end()
	for n < len(b) {
		m, werr := syscall.Write(fd, b[n:])
		if m > 0 {
			n += m
		}
		switch {
		case werr == syscall.EAGAIN:
			p.event(hookWaiting)
			if _, err = waitFD(fd, wake, true, time.Time{}); err != nil {
				if err == errWoken {
					return n, ErrClosed
				}
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
	// Linux keeps the speed in Cflag (CBAUD), as a code. On Darwin and the
	// BSDs the constant is the numeric rate, whose bits are mode bits there:
	// the speed goes in Ispeed and Ospeed only.
	if runtime.GOOS == "linux" {
		termios.Cflag |= flag
	}
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
	switch c.FlowControl {
	case "", FlowNone:
	case FlowRTSCTS:
		termios.Cflag |= crtscts
	case FlowXONXOFF:
		termios.Iflag |= syscall.IXON | syscall.IXOFF
		termios.Cc[syscall.VSTART] = XON
		termios.Cc[syscall.VSTOP] = XOFF
	case FlowDTRDSR:
		err = fmt.Errorf("serial: flow control %s is supported on Windows only: %w", c.FlowControl, ErrUnsupportedPlatform)
		return
	default:
		err = fmt.Errorf("serial: unsupported flow control %q: %w", c.FlowControl, ErrInvalidConfig)
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

// enableRS485 enables RS485 functionality of driver via an ioctl if the
// config says so. With verify set it reads the settings back and fails when
// the driver applied other delays.
func enableRS485(fd int, config *RS485Config, verify bool) error {
	if !config.Enabled {
		return nil
	}
	if rs485Tiocs == 0 {
		return fmt.Errorf("serial: RS485 is supported on Linux only: %w", ErrUnsupportedPlatform)
	}
	return applyRS485(fd, config, verify)
}

// rs485Ioctl sends the RS-485 ioctl req with opts; tests replace it.
var rs485Ioctl = func(fd int, req uint, opts *rs485IoctlOpts) error {
	r, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(req),
		uintptr(unsafe.Pointer(opts)))
	if errno != 0 {
		return os.NewSyscallError("SYS_IOCTL (RS485)", errno)
	}
	if r != 0 {
		return fmt.Errorf("RS485 ioctl returned %d", r)
	}
	return nil
}

// applyRS485 sets the RS-485 options of config with TIOCSRS485 and, with
// verify set, reads them back with TIOCGRS485 and compares the delays.
func applyRS485(fd int, config *RS485Config, verify bool) error {
	if config.DelayRtsBeforeSend < 0 || config.DelayRtsBeforeSend > MaxRS485Delay ||
		config.DelayRtsAfterSend < 0 || config.DelayRtsAfterSend > MaxRS485Delay {
		return fmt.Errorf("serial: RS485 delay out of range: %w", ErrInvalidConfig)
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

	want := rs485
	if err := rs485Ioctl(fd, rs485Tiocs, &rs485); err != nil {
		return fmt.Errorf("serial: enable RS485: %w", err)
	}
	if !verify {
		return nil
	}
	var got rs485IoctlOpts
	if err := rs485Ioctl(fd, rs485Tiocg, &got); err != nil {
		return fmt.Errorf("serial: read RS485 settings: %w", err)
	}
	return checkRS485(config, &want, &got)
}

// checkRS485 compares the delays of want, the options Open set, with got,
// the options the driver reports. The flags are not compared: a driver
// changes them by rules of its own (when both RTS levels are equal, for
// one).
func checkRS485(config *RS485Config, want, got *rs485IoctlOpts) error {
	if got.delay_rts_before_send != want.delay_rts_before_send {
		return notApplied("RS485.DelayRtsBeforeSend", config.DelayRtsBeforeSend, fmt.Sprintf("%d ms", got.delay_rts_before_send))
	}
	if got.delay_rts_after_send != want.delay_rts_after_send {
		return notApplied("RS485.DelayRtsAfterSend", config.DelayRtsAfterSend, fmt.Sprintf("%d ms", got.delay_rts_after_send))
	}
	return nil
}

// notApplied is the error of Config.VerifySettings for a setting the port
// did not take.
func notApplied(field string, value any, reported string) error {
	return &ConfigError{Field: field, Value: value, Reason: "not applied: the port reports " + reported, Err: ErrInvalidConfig}
}

// checkTermios reads the terminal settings back from the port and compares
// them with want, the settings open set.
func (p *port) checkTermios(c *Config, want *syscall.Termios) error {
	got := &syscall.Termios{}
	if err := tcgetattr(p.fd, got); err != nil {
		return fmt.Errorf("serial: read termios: %w", err)
	}
	return compareTermios(c, want, got)
}

// compareTermios returns an error naming the first setting of c in which
// got, the settings the port reports, differs from want, the settings open
// set: the output speed, the character size, the stop bits, the parity or
// the flow control. Other flags are not compared. Parity is compared as
// none, even or odd, so a PARODD bit without PARENB counts as no parity;
// flow control as none, RTS/CTS or XON/XOFF.
func compareTermios(c *Config, want, got *syscall.Termios) error {
	if s := termiosSpeed(got); s != termiosSpeed(want) {
		return notApplied("BaudRate", c.BaudRate, speedName(s))
	}
	if size := got.Cflag & syscall.CSIZE; size != want.Cflag&syscall.CSIZE {
		return notApplied("DataBits", c.DataBits, sizeName(size)+" data bits")
	}
	if stop := got.Cflag & syscall.CSTOPB; stop != want.Cflag&syscall.CSTOPB {
		reported := "1 stop bit"
		if stop != 0 {
			reported = "2 stop bits"
		}
		return notApplied("StopBits", c.StopBits, reported)
	}
	if parity := termiosParity(got); parity != termiosParity(want) {
		return notApplied("Parity", c.Parity, "parity "+parity.String())
	}
	if flow := termiosFlow(got); flow != termiosFlow(want) {
		return notApplied("FlowControl", c.FlowControl, "flow control "+flow.String())
	}
	return nil
}

// termiosFlow returns the flow control that t sets: RTS/CTS when every
// CRTSCTS bit is set, XON/XOFF when IXON and IXOFF are, else none.
func termiosFlow(t *syscall.Termios) FlowControl {
	switch {
	case t.Cflag&crtscts == crtscts:
		return FlowRTSCTS
	case t.Iflag&(syscall.IXON|syscall.IXOFF) == syscall.IXON|syscall.IXOFF:
		return FlowXONXOFF
	}
	return FlowNone
}

// speedName returns the rate of a speed code of this platform, or the code.
func speedName(code termiosFlag) string {
	for rate, c := range baudRates {
		if c == code {
			return strconv.Itoa(rate) + " baud"
		}
	}
	return fmt.Sprintf("speed code %#x", code)
}

// sizeName returns the number of data bits of a CSIZE value.
func sizeName(size termiosFlag) string {
	for bits, s := range charSizes {
		if s == size {
			return strconv.Itoa(bits)
		}
	}
	return fmt.Sprintf("CSIZE %#x", size)
}

// termiosParity returns the parity that the flags of t select.
func termiosParity(t *syscall.Termios) Parity {
	switch {
	case t.Cflag&syscall.PARENB == 0:
		return ParityNone
	case t.Cflag&syscall.PARODD != 0:
		return ParityOdd
	default:
		return ParityEven
	}
}
