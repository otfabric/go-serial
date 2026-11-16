//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// wantNotApplied checks that err is the *ConfigError of VerifySettings for
// field, wrapping ErrInvalidConfig, and that its reason contains reported.
func wantNotApplied(t *testing.T, err error, field, reported string) {
	t.Helper()
	var ce *ConfigError
	if !errors.As(err, &ce) || !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v; want a *ConfigError wrapping ErrInvalidConfig", err)
	}
	if ce.Field != field || !strings.Contains(ce.Reason, "not applied") || !strings.Contains(ce.Reason, reported) {
		t.Fatalf("err = %v; want field %s and a reason with %q", err, field, reported)
	}
}

func mustTermios(t *testing.T, c Config) *syscall.Termios {
	t.Helper()
	tio, err := newTermios(&c)
	if err != nil {
		t.Fatal(err)
	}
	return tio
}

// TestCompareTermios: the settings the port reports are compared with those
// Open set on speed, character size, stop bits and parity, in that order.
func TestCompareTermios(t *testing.T) {
	asked := Config{BaudRate: 19200, DataBits: 7, StopBits: StopBits2, Parity: ParityOdd}
	want := mustTermios(t, asked)
	other := func(c Config) *syscall.Termios { return mustTermios(t, c) }
	tests := []struct {
		name     string
		got      func() *syscall.Termios
		field    string // "" for no error
		reported string
	}{
		{"same", func() *syscall.Termios { g := *want; return &g }, "", ""},
		{"other flags differ", func() *syscall.Termios {
			g := *want
			g.Cflag &^= syscall.CLOCAL | syscall.CREAD
			g.Iflag ^= syscall.INPCK
			g.Lflag |= syscall.ECHO
			return &g
		}, "", ""},
		{"speed", func() *syscall.Termios {
			return other(Config{BaudRate: 9600, DataBits: 7, StopBits: StopBits2, Parity: ParityOdd})
		}, "BaudRate", "9600 baud"},
		{"speed first of several", func() *syscall.Termios { return other(Config{BaudRate: 9600}) }, "BaudRate", "9600 baud"},
		{"data bits", func() *syscall.Termios {
			return other(Config{BaudRate: 19200, DataBits: 8, StopBits: StopBits2, Parity: ParityOdd})
		}, "DataBits", "8 data bits"},
		{"stop bits cleared", func() *syscall.Termios { g := *want; g.Cflag &^= syscall.CSTOPB; return &g }, "StopBits", "1 stop bit"},
		{"parity cleared, odd bit kept", func() *syscall.Termios { g := *want; g.Cflag &^= syscall.PARENB; return &g }, "Parity", "parity N"},
		{"parity even for odd", func() *syscall.Termios { g := *want; g.Cflag &^= syscall.PARODD; return &g }, "Parity", "parity E"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := compareTermios(&asked, want, tt.got())
			if tt.field == "" {
				if err != nil {
					t.Fatalf("compareTermios = %v, want nil", err)
				}
				return
			}
			wantNotApplied(t, err, tt.field, tt.reported)
		})
	}

	// Asked for one stop bit and no parity: two stop bits are reported as
	// such, and an odd bit without the parity bit is still no parity.
	plain := Config{BaudRate: 9600, DataBits: 8, StopBits: StopBits1, Parity: ParityNone}
	w := mustTermios(t, plain)
	g := *w
	g.Cflag |= syscall.CSTOPB
	wantNotApplied(t, compareTermios(&plain, w, &g), "StopBits", "2 stop bits")
	g = *w
	g.Cflag |= syscall.PARODD
	if err := compareTermios(&plain, w, &g); err != nil {
		t.Fatalf("PARODD without PARENB: %v, want nil", err)
	}
}

// TestCompareTermiosEverySpeed: each speed of the table compares equal to
// itself and unequal to every other one.
func TestCompareTermiosEverySpeed(t *testing.T) {
	for rate := range baudRates {
		c := Config{BaudRate: BaudRate(rate)}
		want := mustTermios(t, c)
		if err := compareTermios(&c, want, mustTermios(t, c)); err != nil {
			t.Fatalf("%d baud against itself: %v", rate, err)
		}
		for other := range baudRates {
			if other == rate {
				continue
			}
			err := compareTermios(&c, want, mustTermios(t, Config{BaudRate: BaudRate(other)}))
			wantNotApplied(t, err, "BaudRate", " baud")
		}
	}
}

func TestSpeedAndSizeNames(t *testing.T) {
	if got := speedName(baudRates[9600]); got != "9600 baud" {
		t.Errorf("speedName(B9600) = %q", got)
	}
	if got := speedName(1 << 20); got != "speed code 0x100000" {
		t.Errorf("speedName(1<<20) = %q", got)
	}
	if got := sizeName(charSizes[5]); got != "5" {
		t.Errorf("sizeName(CS5) = %q", got)
	}
	if got := sizeName(1 << 20); got != "CSIZE 0x100000" {
		t.Errorf("sizeName(1<<20) = %q", got)
	}
}

// TestCheckTermiosReadError: a failure to read the settings back names the
// operation and keeps the cause.
func TestCheckTermiosReadError(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip(err)
	}
	defer func() { _ = f.Close() }()
	cause := tcgetattr(int(f.Fd()), &syscall.Termios{})
	if cause == nil {
		t.Skipf("%s has terminal settings", os.DevNull)
	}
	p := &port{fd: int(f.Fd())}
	err = p.checkTermios(&Config{}, &syscall.Termios{})
	if !errors.Is(err, cause) || !strings.HasPrefix(err.Error(), "serial: read termios: ") {
		t.Fatalf("checkTermios = %v; want the prefix \"serial: read termios: \" and %v", err, cause)
	}
}

// fakeRS485Driver stands in for a driver behind the RS-485 ioctls: the
// first call sets, the second reads back. apply changes what was set, as a
// driver that clamps a value does.
type fakeRS485Driver struct {
	apply   func(*rs485IoctlOpts)
	setErr  error
	getErr  error
	applied rs485IoctlOpts
	reqs    []uint
}

func (d *fakeRS485Driver) install(t *testing.T) {
	t.Helper()
	orig := rs485Ioctl
	t.Cleanup(func() { rs485Ioctl = orig })
	rs485Ioctl = func(_ int, req uint, opts *rs485IoctlOpts) error {
		d.reqs = append(d.reqs, req)
		if len(d.reqs) == 1 {
			if d.setErr != nil {
				return d.setErr
			}
			d.applied = *opts
			if d.apply != nil {
				d.apply(&d.applied)
			}
			return nil
		}
		if d.getErr != nil {
			return d.getErr
		}
		*opts = d.applied
		return nil
	}
}

// TestApplyRS485Verify: with verify set, a delay the driver did not apply
// is an error naming the field; the flags are not compared. Without verify
// the settings are not read back. A failing ioctl names the operation.
func TestApplyRS485Verify(t *testing.T) {
	const ms = time.Millisecond
	clampBefore := func(o *rs485IoctlOpts) { o.delay_rts_before_send = min(o.delay_rts_before_send, 100) }
	tests := []struct {
		name     string
		cfg      RS485Config
		verify   bool
		driver   fakeRS485Driver
		calls    int
		field    string // "" for no error
		reported string
		cause    error  // with prefix, for an ioctl failure
		prefix   string // of the ioctl failure
	}{
		{"applied as set", RS485Config{Enabled: true, DelayRtsBeforeSend: 2 * ms, DelayRtsAfterSend: 3 * ms}, true, fakeRS485Driver{}, 2, "", "", nil, ""},
		{"zero delays", RS485Config{Enabled: true}, true, fakeRS485Driver{}, 2, "", "", nil, ""},
		{"sub-millisecond part", RS485Config{Enabled: true, DelayRtsBeforeSend: 1500 * time.Microsecond}, true, fakeRS485Driver{}, 2, "", "", nil, ""},
		{"at the clamp", RS485Config{Enabled: true, DelayRtsBeforeSend: 100 * ms}, true, fakeRS485Driver{apply: clampBefore}, 2, "", "", nil, ""},
		{"above the clamp", RS485Config{Enabled: true, DelayRtsBeforeSend: 101 * ms}, true, fakeRS485Driver{apply: clampBefore}, 2, "RS485.DelayRtsBeforeSend", "100 ms", nil, ""},
		{"largest delay", RS485Config{Enabled: true, DelayRtsBeforeSend: MaxRS485Delay}, true, fakeRS485Driver{apply: clampBefore}, 2, "RS485.DelayRtsBeforeSend", "100 ms", nil, ""},
		{"delay dropped", RS485Config{Enabled: true, DelayRtsAfterSend: 5 * ms}, true,
			fakeRS485Driver{apply: func(o *rs485IoctlOpts) { o.delay_rts_after_send = 0 }}, 2, "RS485.DelayRtsAfterSend", "0 ms", nil, ""},
		{"flags changed", RS485Config{Enabled: true, RxDuringTx: true}, true,
			fakeRS485Driver{apply: func(o *rs485IoctlOpts) { o.flags = rs485Enabled | rs485RTSOnSend }}, 2, "", "", nil, ""},
		{"not verified", RS485Config{Enabled: true, DelayRtsBeforeSend: 200 * ms}, false, fakeRS485Driver{apply: clampBefore}, 1, "", "", nil, ""},
		{"set fails", RS485Config{Enabled: true}, true, fakeRS485Driver{setErr: os.NewSyscallError("SYS_IOCTL (RS485)", syscall.ENOTTY)}, 1, "", "", syscall.ENOTTY, "serial: enable RS485: "},
		{"read back fails", RS485Config{Enabled: true}, true, fakeRS485Driver{getErr: os.NewSyscallError("SYS_IOCTL (RS485)", syscall.EINVAL)}, 2, "", "", syscall.EINVAL, "serial: read RS485 settings: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.driver
			d.install(t)
			err := applyRS485(3, &tt.cfg, tt.verify)
			if len(d.reqs) != tt.calls {
				t.Fatalf("%d ioctl calls, want %d", len(d.reqs), tt.calls)
			}
			if want := []uint{rs485Tiocs, rs485Tiocg}[:tt.calls]; d.reqs[0] != want[0] || tt.calls == 2 && d.reqs[1] != want[1] {
				t.Fatalf("ioctl requests %#x, want %#x", d.reqs, want)
			}
			switch {
			case tt.cause != nil:
				if !errors.Is(err, tt.cause) || !strings.HasPrefix(err.Error(), tt.prefix) {
					t.Fatalf("applyRS485 = %v; want the prefix %q and %v", err, tt.prefix, tt.cause)
				}
			case tt.field != "":
				wantNotApplied(t, err, tt.field, tt.reported)
			case err != nil:
				t.Fatalf("applyRS485 = %v, want nil", err)
			}
		})
	}
}

// TestOpenVerifySettings: on a pseudo-terminal, Open with VerifySettings
// succeeds for every speed of the table and for 8N1 and 8N2, which every
// pseudo-terminal takes. Linux's pseudo-terminal takes 8 data bits without
// parity whatever is asked, so there other data bits or a parity fail with
// the port closed and no descriptor left, while Open without
// VerifySettings succeeds as before. macOS takes them.
func TestOpenVerifySettings(t *testing.T) {
	master, slave, err := openPTY()
	if err != nil {
		skipNoPTY(t, err)
	}
	defer func() { _ = master.Close() }()

	open := func(c Config) (Port, error) {
		c.Address = slave
		return Open(&c)
	}
	for rate := range baudRates {
		p, err := open(Config{BaudRate: BaudRate(rate), VerifySettings: true})
		if err != nil {
			t.Fatalf("%d baud: %v", rate, err)
		}
		_ = p.Close()
	}
	if p, err := open(Config{StopBits: StopBits2, VerifySettings: true}); err != nil {
		t.Fatalf("8N2: %v", err)
	} else {
		_ = p.Close()
	}

	for _, tc := range []struct {
		name  string
		cfg   Config
		field string
	}{
		{"8E1", Config{BaudRate: 19200, Parity: ParityEven}, "Parity"},
		{"8O1", Config{BaudRate: 19200, Parity: ParityOdd}, "Parity"},
		{"7E1", Config{DataBits: 7, Parity: ParityEven}, "DataBits"},
		{"5N1", Config{DataBits: 5}, "DataBits"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := open(tc.cfg)
			if err != nil {
				t.Fatalf("without VerifySettings: %v", err)
			}
			_ = p.Close()

			before := openFDs()
			verified := tc.cfg
			verified.VerifySettings = true
			p, err = open(verified)
			switch runtime.GOOS {
			case "linux":
				if err == nil {
					_ = p.Close()
					t.Fatal("Open succeeded on a Linux pseudo-terminal")
				}
				wantNotApplied(t, err, tc.field, "")
			case "darwin":
				if err != nil {
					t.Fatalf("Open = %v, want nil on a macOS pseudo-terminal", err)
				}
				_ = p.Close()
			default:
				if err == nil {
					_ = p.Close()
					return
				}
				wantNotApplied(t, err, tc.field, "")
			}
			if after := openFDs(); after > before {
				t.Fatalf("open descriptors grew from %d to %d", before, after)
			}
		})
	}
}

// TestOpenVerifySettingsResetsPort: a failure of VerifySettings closes the
// port and restores its state, at the termios step and at the RS-485 step.
func TestOpenVerifySettingsResetsPort(t *testing.T) {
	master, slave, err := openPTY()
	if err != nil {
		skipNoPTY(t, err)
	}
	defer func() { _ = master.Close() }()

	t.Run("termios", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("only Linux's pseudo-terminal changes the settings")
		}
		before := openFDs()
		p := newPort()
		err := p.open(&Config{Address: slave, BaudRate: 9600, Parity: ParityEven, VerifySettings: true})
		wantNotApplied(t, err, "Parity", "parity N")
		if p.fd != -1 || p.oldTermios != nil || p.wake != nil {
			t.Fatalf("port not reset: fd=%d oldTermios=%v wake=%v", p.fd, p.oldTermios, p.wake)
		}
		if after := openFDs(); after > before {
			t.Fatalf("open descriptors grew from %d to %d", before, after)
		}
	})

	t.Run("RS-485", func(t *testing.T) {
		cfg := Config{Address: slave, BaudRate: 9600, VerifySettings: true,
			RS485: RS485Config{Enabled: true, DelayRtsBeforeSend: 500 * time.Millisecond}}
		d := fakeRS485Driver{apply: func(o *rs485IoctlOpts) { o.delay_rts_before_send = 100 }}
		d.install(t)
		before := openFDs()
		p := newPort()
		err := p.open(&cfg)
		if rs485Tiocs == 0 {
			if !errors.Is(err, ErrUnsupportedPlatform) {
				t.Fatalf("open = %v, want ErrUnsupportedPlatform", err)
			}
		} else {
			wantNotApplied(t, err, "RS485.DelayRtsBeforeSend", "100 ms")
		}
		if p.fd != -1 || p.oldTermios != nil || p.wake != nil {
			t.Fatalf("port not reset: fd=%d oldTermios=%v wake=%v", p.fd, p.oldTermios, p.wake)
		}
		if after := openFDs(); after > before {
			t.Fatalf("open descriptors grew from %d to %d", before, after)
		}
	})
}
