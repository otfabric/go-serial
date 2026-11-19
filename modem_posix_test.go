//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"bytes"
	"errors"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// fakeModem replaces the modem-line ioctl with a port whose lines are the
// bits of state: TIOCMBIS and TIOCMBIC change it, TIOCMGET reads it.
type fakeModem struct {
	state int32
	fail  error
	reqs  []uintptr
}

func (m *fakeModem) install(t *testing.T) {
	t.Helper()
	orig := modemIoctl
	t.Cleanup(func() { modemIoctl = orig })
	modemIoctl = func(_ int, req uintptr, bits *int32) error {
		m.reqs = append(m.reqs, req)
		if m.fail != nil {
			return m.fail
		}
		switch req {
		case syscall.TIOCMBIS:
			m.state |= *bits
		case syscall.TIOCMBIC:
			m.state &^= *bits
		case syscall.TIOCMGET:
			*bits = m.state
		default:
			t.Fatalf("unexpected modem ioctl %#x", req)
		}
		return nil
	}
}

// TestModemLinesIoctl: SetDTR and SetRTS set and clear their own bit, and
// ModemStatus maps the TIOCMGET bits, against a replaced ioctl.
func TestModemLinesIoctl(t *testing.T) {
	m := &fakeModem{}
	m.install(t)
	p := newPort()
	p.fd = 42
	var lines ModemLines = p

	steps := []struct {
		do   func() error
		want int32
	}{
		{func() error { return lines.SetDTR(true) }, syscall.TIOCM_DTR},
		{func() error { return lines.SetRTS(true) }, syscall.TIOCM_DTR | syscall.TIOCM_RTS},
		{func() error { return lines.SetDTR(false) }, syscall.TIOCM_RTS},
		{func() error { return lines.SetRTS(false) }, 0},
	}
	for i, s := range steps {
		if err := s.do(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if m.state != s.want {
			t.Fatalf("step %d: lines %#x; want %#x", i, m.state, s.want)
		}
	}

	for bits, want := range map[int32]ModemStatus{
		0:                 {},
		syscall.TIOCM_CTS: {CTS: true},
		syscall.TIOCM_DSR: {DSR: true},
		syscall.TIOCM_RI:  {RI: true},
		syscall.TIOCM_CD:  {DCD: true},
		syscall.TIOCM_DTR: {},
		syscall.TIOCM_CTS | syscall.TIOCM_DSR | syscall.TIOCM_RI | syscall.TIOCM_CD: {CTS: true, DSR: true, RI: true, DCD: true},
	} {
		m.state = bits
		got, err := lines.ModemStatus()
		if err != nil || got != want {
			t.Errorf("TIOCMGET %#x: %+v, %v; want %+v", bits, got, err, want)
		}
	}

	m.fail = syscall.ENOTTY
	if err := lines.SetDTR(true); !errors.Is(err, syscall.ENOTTY) || err.Error() != "serial: set DTR: "+syscall.ENOTTY.Error() {
		t.Errorf("SetDTR on a port without modem lines: %v", err)
	}
	if _, err := lines.ModemStatus(); !errors.Is(err, syscall.ENOTTY) {
		t.Errorf("ModemStatus on a port without modem lines: %v", err)
	}
}

// TestModemLinesInUseAndClosed: a line that flow control or RS-485 drives
// is refused before any ioctl, and a closed port returns ErrClosed.
func TestModemLinesInUseAndClosed(t *testing.T) {
	m := &fakeModem{}
	m.install(t)
	p := newPort()
	p.fd = 42
	p.lines = Config{FlowControl: FlowRTSCTS}
	if err := p.SetRTS(true); !errors.Is(err, ErrLineInUse) {
		t.Errorf("SetRTS under RTS/CTS: %v", err)
	}
	if err := p.SetDTR(true); err != nil {
		t.Errorf("SetDTR under RTS/CTS: %v", err)
	}
	p.lines = Config{RS485: RS485Config{Enabled: true}}
	if err := p.SetRTS(false); !errors.Is(err, ErrLineInUse) {
		t.Errorf("SetRTS under RS-485: %v", err)
	}
	if len(m.reqs) != 1 {
		t.Errorf("ioctls sent: %d; want 1 (the SetDTR)", len(m.reqs))
	}

	// ErrClosed comes first, also for a line that flow control drives.
	p.lines = Config{FlowControl: FlowRTSCTS}
	p.closed = true
	if err := p.SetRTS(true); !errors.Is(err, ErrClosed) {
		t.Errorf("SetRTS under RTS/CTS after Close: %v", err)
	}
	p.lines = Config{}
	if err := p.SetDTR(true); !errors.Is(err, ErrClosed) {
		t.Errorf("SetDTR after Close: %v", err)
	}
	if _, err := p.ModemStatus(); !errors.Is(err, ErrClosed) {
		t.Errorf("ModemStatus after Close: %v", err)
	}
}

// TestFlowControlTermios: the termios bits of each flow control, and the
// read-back comparison of them.
func TestFlowControlTermios(t *testing.T) {
	none := mustTermios(t, Config{})
	rts := mustTermios(t, Config{FlowControl: FlowRTSCTS})
	xon := mustTermios(t, Config{FlowControl: FlowXONXOFF})
	if none.Cflag&crtscts != 0 || none.Iflag&(syscall.IXON|syscall.IXOFF) != 0 {
		t.Errorf("no flow control sets flow bits: %+v", none)
	}
	if rts.Cflag&crtscts != crtscts || rts.Iflag&(syscall.IXON|syscall.IXOFF) != 0 {
		t.Errorf("RTS/CTS: %+v", rts)
	}
	if xon.Iflag&(syscall.IXON|syscall.IXOFF) != syscall.IXON|syscall.IXOFF || xon.Cflag&crtscts != 0 ||
		xon.Cc[syscall.VSTART] != XON || xon.Cc[syscall.VSTOP] != XOFF {
		t.Errorf("XON/XOFF: %+v", xon)
	}
	if _, err := newTermios(&Config{FlowControl: FlowDTRDSR}); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Errorf("DTR/DSR: %v", err)
	}
	if _, err := newTermios(&Config{FlowControl: "bogus"}); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("unknown flow control: %v", err)
	}

	cfg := Config{FlowControl: FlowRTSCTS}
	if err := compareTermios(&cfg, rts, rts); err != nil {
		t.Errorf("same: %v", err)
	}
	wantNotApplied(t, compareTermios(&cfg, rts, none), "FlowControl", "flow control none")
	cfg = Config{FlowControl: FlowXONXOFF}
	wantNotApplied(t, compareTermios(&cfg, xon, rts), "FlowControl", "flow control rts-cts")
	cfg = Config{}
	wantNotApplied(t, compareTermios(&cfg, none, xon), "FlowControl", "flow control xon-xoff")
}

// TestOpenFlowControlOverPTY: Open sets the flow control on a real terminal
// and, with VerifySettings, reads it back; DTR/DSR flow control is refused.
// A pseudo-terminal has no modem lines, so the modem-line ioctls fail there
// with the operating system's error.
func TestOpenFlowControlOverPTY(t *testing.T) {
	master, slave, err := openPTY()
	if err != nil {
		skipNoPTY(t, err)
	}
	defer func() { _ = master.Close() }()
	for _, f := range []FlowControl{FlowNone, FlowRTSCTS, FlowXONXOFF} {
		p, err := Open(&Config{Address: slave, FlowControl: f, VerifySettings: true})
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		_ = p.Close()
	}
	if _, err := Open(&Config{Address: slave, FlowControl: FlowDTRDSR}); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Errorf("DTR/DSR: %v", err)
	}

	p, err := Open(&Config{Address: slave})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	lines, ok := p.(ModemLines)
	if !ok {
		t.Fatal("the port Open returns does not implement ModemLines")
	}
	if runtime.GOOS == "linux" {
		if err := lines.SetDTR(true); !errors.Is(err, syscall.ENOTTY) {
			t.Errorf("SetDTR on a pseudo-terminal: %v", err)
		}
		if _, err := lines.ModemStatus(); !errors.Is(err, syscall.ENOTTY) {
			t.Errorf("ModemStatus on a pseudo-terminal: %v", err)
		}
	}
}

// TestXONXOFFOverPTY: with XON/XOFF flow control, an XOFF from the other
// end holds what the port writes until an XON, and neither character
// reaches Read.
func TestXONXOFFOverPTY(t *testing.T) {
	master, slave, err := openPTY()
	if err != nil {
		skipNoPTY(t, err)
	}
	defer func() { _ = master.Close() }()
	// openPTY leaves the master blocking (os.File.Fd does); a non-blocking
	// duplicate takes read deadlines.
	fd, err := syscall.Dup(int(master.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	master = os.NewFile(uintptr(fd), "ptmx")
	defer func() { _ = master.Close() }()
	if err := master.SetReadDeadline(time.Now().Add(time.Second)); errors.Is(err, os.ErrNoDeadline) {
		t.Skip("the pseudo-terminal master takes no read deadline here")
	}
	p, err := Open(&Config{Address: slave, FlowControl: FlowXONXOFF, Timeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	if _, err := master.Write([]byte{XOFF}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	written := make(chan error, 1)
	go func() {
		_, err := p.Write([]byte("held"))
		written <- err
	}()
	buf := make([]byte, 16)
	_ = master.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if n, _ := master.Read(buf); n > 0 {
		t.Fatalf("read %q while stopped by XOFF", buf[:n])
	}
	if _, err := master.Write([]byte{XON, 'k'}); err != nil {
		t.Fatal(err)
	}
	var got []byte
	_ = master.SetReadDeadline(time.Now().Add(2 * time.Second))
	for len(got) < 4 {
		n, err := master.Read(buf)
		if err != nil {
			t.Fatalf("after XON: %q, %v", got, err)
		}
		got = append(got, buf[:n]...)
	}
	if !bytes.Equal(got, []byte("held")) {
		t.Errorf("after XON: %q; want %q", got, "held")
	}
	if err := <-written; err != nil {
		t.Errorf("Write: %v", err)
	}
	n, err := p.Read(buf)
	if err != nil || string(buf[:n]) != "k" {
		t.Errorf("Read = %q, %v; want only %q, without XON and XOFF", buf[:n], err, "k")
	}
}
