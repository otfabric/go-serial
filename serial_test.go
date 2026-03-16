package serial

import (
	"errors"
	"os"
	"runtime"
	"testing"
)

const (
	// socat -d -d pty,raw,echo=0 pty,raw,echo=0.
	pty1 = "/dev/ttys009"
	pty2 = "/dev/ttys010"
)

// testPortPath returns a fake port path for tests that don't open the device.
// Use a path that doesn't exist so we don't accidentally touch real hardware.
func testPortPath() string {
	if runtime.GOOS == "windows" {
		return "COM256" // non-existent port for tests that don't open
	}
	return "/dev/tty0"
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("/dev/ttyUSB0")
	if cfg.Address != "/dev/ttyUSB0" {
		t.Errorf("Address: got %q", cfg.Address)
	}
	if cfg.BaudRate != Baud9600 {
		t.Errorf("BaudRate: got %v, want Baud9600", cfg.BaudRate)
	}
	if cfg.DataBits != DataBits8 {
		t.Errorf("DataBits: got %v, want DataBits8", cfg.DataBits)
	}
	if cfg.StopBits != StopBits1 {
		t.Errorf("StopBits: got %v, want StopBits1", cfg.StopBits)
	}
	if cfg.Parity != ParityNone {
		t.Errorf("Parity: got %q, want ParityNone", cfg.Parity)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want bool // true = valid
	}{
		{"nil", nil, false},
		{"empty address", &Config{Address: ""}, false},
		{"valid minimal", &Config{Address: "/dev/tty0"}, true},
		{"valid full", &Config{Address: "/dev/tty0", BaudRate: Baud9600, DataBits: DataBits8, StopBits: StopBits1, Parity: ParityNone}, true},
		{"data bits invalid", &Config{Address: "/dev/tty0", DataBits: 4}, false},
		{"data bits valid 5", &Config{Address: "/dev/tty0", DataBits: DataBits5}, true},
		{"stop bits invalid", &Config{Address: "/dev/tty0", StopBits: 3}, false},
		{"stop bits 1", &Config{Address: "/dev/tty0", StopBits: StopBits1}, true},
		{"stop bits 2", &Config{Address: "/dev/tty0", StopBits: StopBits2}, true},
		{"parity invalid", &Config{Address: "/dev/tty0", Parity: "X"}, false},
		{"parity N", &Config{Address: "/dev/tty0", Parity: ParityNone}, true},
		{"parity E", &Config{Address: "/dev/tty0", Parity: ParityEven}, true},
		{"parity O", &Config{Address: "/dev/tty0", Parity: ParityOdd}, true},
		{"negative timeout", &Config{Address: "/dev/tty0", Timeout: -1}, false},
		{"RS485 negative delay before", &Config{Address: "/dev/tty0", RS485: RS485Config{Enabled: true, DelayRtsBeforeSend: -1}}, false},
		{"RS485 negative delay after", &Config{Address: "/dev/tty0", RS485: RS485Config{Enabled: true, DelayRtsAfterSend: -1}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.cfg)
			valid := err == nil
			if valid != tt.want {
				t.Errorf("ValidateConfig() valid=%v, want %v; err=%v", valid, tt.want, err)
			}
			if err != nil && !tt.want {
				if !errors.Is(err, ErrInvalidConfig) {
					t.Errorf("expected error to wrap ErrInvalidConfig: %v", err)
				}
			}
		})
	}
}

func TestOpenInvalidConfig(t *testing.T) {
	_, err := Open(nil)
	if err == nil {
		t.Fatal("Open(nil) should fail")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Open(nil): want ErrInvalidConfig, got %v", err)
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatal("expected *ConfigError")
	}
	if cfgErr.Field != "Config" {
		t.Errorf("ConfigError.Field = %q, want Config", cfgErr.Field)
	}

	_, err = Open(&Config{Address: ""})
	if err == nil {
		t.Fatal("Open(empty address) should fail")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Open(empty address): want ErrInvalidConfig, got %v", err)
	}
	if !errors.As(err, &cfgErr) {
		t.Fatal("expected *ConfigError for empty address")
	}
	if cfgErr.Field != "Address" {
		t.Errorf("ConfigError.Field = %q, want Address", cfgErr.Field)
	}
}

func TestParseParity(t *testing.T) {
	tests := []struct {
		in   string
		want Parity
		err  bool
	}{
		{"", ParityNone, false},
		{"N", ParityNone, false},
		{"n", ParityNone, false},
		{"E", ParityEven, false},
		{"e", ParityEven, false},
		{"O", ParityOdd, false},
		{"o", ParityOdd, false},
		{"X", "", true},
	}
	for _, tt := range tests {
		got, err := ParseParity(tt.in)
		if tt.err {
			if err == nil {
				t.Errorf("ParseParity(%q) wanted error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseParity(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseParity(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeConfig(t *testing.T) {
	// Open normalizes zero values; check that a minimal config gets defaults.
	c := &Config{Address: "/dev/tty0"}
	norm := normalizeConfig(c)
	if norm.BaudRate != Baud9600 {
		t.Errorf("normalized BaudRate: got %v, want Baud9600", norm.BaudRate)
	}
	if norm.DataBits != DataBits8 {
		t.Errorf("normalized DataBits: got %v, want DataBits8", norm.DataBits)
	}
	if norm.StopBits != StopBits1 {
		t.Errorf("normalized StopBits: got %v, want StopBits1", norm.StopBits)
	}
	if norm.Parity != ParityNone {
		t.Errorf("normalized Parity: got %v, want ParityNone", norm.Parity)
	}
}

func TestConfigNormalized(t *testing.T) {
	// Normalized() is the exported equivalent of normalizeConfig.
	c := Config{Address: "/dev/tty0"}
	norm := c.Normalized()
	if norm.BaudRate != Baud9600 || norm.DataBits != DataBits8 || norm.StopBits != StopBits1 || norm.Parity != ParityNone {
		t.Errorf("Normalized() = %+v, expected defaults", norm)
	}
}

func TestZeroValueStringAndMarshalText(t *testing.T) {
	// String() and MarshalText() reflect actual value; zero stays "0" (no default hiding).
	if got := DataBits(0).String(); got != "0" {
		t.Errorf("DataBits(0).String() = %q, want \"0\"", got)
	}
	if got := StopBits(0).String(); got != "0" {
		t.Errorf("StopBits(0).String() = %q, want \"0\"", got)
	}
	if got := BaudRate(0).String(); got != "0" {
		t.Errorf("BaudRate(0).String() = %q, want \"0\"", got)
	}
	// MarshalText for zero values
	if b, _ := DataBits(0).MarshalText(); string(b) != "0" {
		t.Errorf("DataBits(0).MarshalText() = %q, want \"0\"", string(b))
	}
	if b, _ := StopBits(0).MarshalText(); string(b) != "0" {
		t.Errorf("StopBits(0).MarshalText() = %q, want \"0\"", string(b))
	}
	if b, _ := BaudRate(0).MarshalText(); string(b) != "0" {
		t.Errorf("BaudRate(0).MarshalText() = %q, want \"0\"", string(b))
	}
	// ParseParity("") returns ParityNone (documented behavior)
	if p, err := ParseParity(""); err != nil || p != ParityNone {
		t.Errorf("ParseParity(\"\") = %v, %v; want ParityNone, nil", p, err)
	}
}

func TestConfigErrorErrorString(t *testing.T) {
	// Nil receiver
	var e *ConfigError
	if got := e.Error(); got != "<nil>" {
		t.Errorf("(*ConfigError)(nil).Error() = %q, want \"<nil>\"", got)
	}
	// With Reason
	err := &ConfigError{Field: "DataBits", Value: 9, Reason: "must be 5, 6, 7, or 8", Err: ErrInvalidConfig}
	msg := err.Error()
	if msg == "" || msg == "<nil>" {
		t.Errorf("ConfigError.Error() = %q", msg)
	}
	if msg != err.Error() {
		t.Error("Error() should be stable")
	}
	// Without Reason, with Err
	err2 := &ConfigError{Field: "X", Value: "y", Err: ErrInvalidConfig}
	if err2.Error() == "" || err2.Error() == "<nil>" {
		t.Errorf("ConfigError.Error() = %q", err2.Error())
	}
}

func TestOpenUnsupportedPlatform(t *testing.T) {
	// When openPort is nil (unsupported build), Open returns error wrapping ErrUnsupportedPlatform.
	saved := openPort
	defer func() { openPort = saved }()
	openPort = nil
	_, err := Open(&Config{Address: testPortPath()})
	if err == nil {
		t.Fatal("Open with nil openPort should fail")
	}
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Errorf("Open should wrap ErrUnsupportedPlatform: %v", err)
	}
}

func TestParseDataBits(t *testing.T) {
	for _, n := range []int{5, 6, 7, 8} {
		d, err := ParseDataBits(n)
		if err != nil {
			t.Errorf("ParseDataBits(%d): %v", n, err)
		}
		if int(d) != n {
			t.Errorf("ParseDataBits(%d) = %v", n, d)
		}
	}
	_, err := ParseDataBits(4)
	if err == nil || !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("ParseDataBits(4): want ErrInvalidConfig, got %v", err)
	}
}

func TestParseStopBits(t *testing.T) {
	for _, n := range []int{1, 2} {
		s, err := ParseStopBits(n)
		if err != nil {
			t.Errorf("ParseStopBits(%d): %v", n, err)
		}
		if int(s) != n {
			t.Errorf("ParseStopBits(%d) = %v", n, s)
		}
	}
	_, err := ParseStopBits(3)
	if err == nil || !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("ParseStopBits(3): want ErrInvalidConfig, got %v", err)
	}
}

func TestParseBaudRate(t *testing.T) {
	b, err := ParseBaudRate(9600)
	if err != nil {
		t.Errorf("ParseBaudRate(9600): %v", err)
	}
	if b != Baud9600 {
		t.Errorf("ParseBaudRate(9600) = %v, want Baud9600", b)
	}
	_, err = ParseBaudRate(0)
	if err == nil || !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("ParseBaudRate(0): want ErrInvalidConfig, got %v", err)
	}
	_, err = ParseBaudRate(-1)
	if err == nil {
		t.Error("ParseBaudRate(-1): want error")
	}
}

func TestUnmarshalTextNilReceiver(t *testing.T) {
	// Nil receiver guards return an error instead of panicking.
	var p *Parity
	if err := p.UnmarshalText([]byte("N")); err == nil {
		t.Error("nil *Parity.UnmarshalText should return error")
	}
	var d *DataBits
	if err := d.UnmarshalText([]byte("8")); err == nil {
		t.Error("nil *DataBits.UnmarshalText should return error")
	}
	var s *StopBits
	if err := s.UnmarshalText([]byte("1")); err == nil {
		t.Error("nil *StopBits.UnmarshalText should return error")
	}
	var b *BaudRate
	if err := b.UnmarshalText([]byte("9600")); err == nil {
		t.Error("nil *BaudRate.UnmarshalText should return error")
	}
}

func TestParityUnmarshalText(t *testing.T) {
	var p Parity
	for _, s := range []string{"N", "n", "E", "e", "O", "o"} {
		if err := p.UnmarshalText([]byte(s)); err != nil {
			t.Errorf("UnmarshalText(%q): %v", s, err)
		}
	}
	if err := p.UnmarshalText([]byte("X")); err == nil {
		t.Error("UnmarshalText(\"X\") wanted error")
	}
}

func TestParityString(t *testing.T) {
	if got := ParityNone.String(); got != "N" {
		t.Errorf("ParityNone.String() = %q, want N", got)
	}
	if got := ParityEven.String(); got != "E" {
		t.Errorf("ParityEven.String() = %q, want E", got)
	}
}

func TestIsUnsupportedBaudRate(t *testing.T) {
	if IsUnsupportedBaudRate(nil) {
		t.Error("IsUnsupportedBaudRate(nil) should be false")
	}
	if IsUnsupportedBaudRate(ErrInvalidConfig) {
		t.Error("IsUnsupportedBaudRate(ErrInvalidConfig) should be false")
	}
	err := &ConfigError{Field: "BaudRate", Value: 99999, Reason: "unsupported baud rate for this platform", Err: ErrInvalidConfig}
	if !IsUnsupportedBaudRate(err) {
		t.Error("IsUnsupportedBaudRate(ConfigError BaudRate unsupported) should be true")
	}
	errOther := &ConfigError{Field: "Address", Value: "", Reason: "empty address", Err: ErrInvalidConfig}
	if IsUnsupportedBaudRate(errOther) {
		t.Error("IsUnsupportedBaudRate(other ConfigError) should be false")
	}
}

func TestErrUnsupportedPlatformSentinel(t *testing.T) {
	if ErrUnsupportedPlatform == nil {
		t.Fatal("ErrUnsupportedPlatform must be non-nil")
	}
	if !errors.Is(ErrUnsupportedPlatform, ErrUnsupportedPlatform) {
		t.Error("errors.Is(ErrUnsupportedPlatform, ErrUnsupportedPlatform) should be true")
	}
}

func TestConfigErrorWithReason(t *testing.T) {
	err := ValidateConfig(&Config{Address: "/dev/tty0", DataBits: 9})
	if err == nil {
		t.Fatal("expected error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatal("expected *ConfigError")
	}
	if cfgErr.Reason == "" {
		t.Error("ConfigError should have Reason set")
	}
	if cfgErr.Field != "DataBits" || cfgErr.Reason == "" {
		t.Errorf("ConfigError: Field=%q Reason=%q", cfgErr.Field, cfgErr.Reason)
	}
	// Error() should include value and reason
	msg := cfgErr.Error()
	if msg == "" || msg == "<nil>" {
		t.Errorf("ConfigError.Error() = %q", msg)
	}
}

func TestErrTimeoutSentinel(t *testing.T) {
	// ErrTimeout is a sentinel that callers can use with errors.Is.
	if ErrTimeout == nil {
		t.Fatal("ErrTimeout must be non-nil")
	}
	if !errors.Is(ErrTimeout, ErrTimeout) {
		t.Error("errors.Is(ErrTimeout, ErrTimeout) should be true")
	}
}

func TestReadWrite(t *testing.T) {
	checkPty(t)

	config1 := Config{Address: pty1}
	port1, err := Open(&config1)
	if err != nil {
		t.Skipf("skip when no loopback serial available: %v", err)
	}
	defer func() { _ = port1.Close() }()

	config2 := Config{
		Address:  pty2,
		BaudRate: Baud57600,
		DataBits: DataBits7,
		Parity:   ParityNone,
		StopBits: StopBits2,
	}
	port2, err := Open(&config2)
	if err != nil {
		t.Skipf("skip when no loopback serial available: %v", err)
	}
	defer func() { _ = port2.Close() }()

	message := "test serial"
	n, err := port1.Write([]byte(message))
	if err != nil {
		t.Fatal(err)
	}
	if n != len(message) {
		t.Fatalf("unexpected write length %v", n)
	}
	var buf [16]byte
	n, err = port2.Read(buf[:])
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != message {
		t.Fatalf("unexpected response %q (len: %d)", buf[:n], n)
	}
}

func checkPty(t *testing.T) {
	for _, p := range [...]string{pty1, pty2} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("%v does not exist", p)
		}
	}
}
