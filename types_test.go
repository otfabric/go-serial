package serial

import (
	"errors"
	"testing"
)

func TestParityStringDefaultBranch(t *testing.T) {
	// Unknown parity value falls through to string(p).
	if got := Parity("X").String(); got != "X" {
		t.Errorf("Parity(\"X\").String() = %q, want X", got)
	}
	if got := Parity("").String(); got != "" {
		t.Errorf("Parity(\"\").String() = %q, want empty", got)
	}
}

func TestParityMarshalText(t *testing.T) {
	tests := []struct {
		p    Parity
		want string
	}{
		{ParityNone, "N"},
		{ParityEven, "E"},
		{ParityOdd, "O"},
		{Parity("?"), "?"},
	}
	for _, tt := range tests {
		b, err := tt.p.MarshalText()
		if err != nil {
			t.Errorf("MarshalText(%q): %v", tt.p, err)
			continue
		}
		if string(b) != tt.want {
			t.Errorf("MarshalText(%q) = %q, want %q", tt.p, b, tt.want)
		}
	}
}

func TestDataBitsStringAndMarshalTextNonZero(t *testing.T) {
	want := map[DataBits]string{
		DataBits5: "5", DataBits6: "6", DataBits7: "7", DataBits8: "8",
	}
	for _, n := range []DataBits{DataBits5, DataBits6, DataBits7, DataBits8} {
		if got := n.String(); got != want[n] {
			t.Errorf("DataBits(%d).String() = %q, want %q", n, got, want[n])
		}
		b, err := n.MarshalText()
		if err != nil {
			t.Errorf("DataBits(%d).MarshalText(): %v", n, err)
			continue
		}
		if string(b) != want[n] {
			t.Errorf("DataBits(%d).MarshalText() = %q, want %q", n, b, want[n])
		}
	}
}

func TestStopBitsStringAndMarshalTextNonZero(t *testing.T) {
	for _, n := range []StopBits{StopBits1, StopBits2} {
		want := map[StopBits]string{StopBits1: "1", StopBits2: "2"}[n]
		if n.String() != want {
			t.Errorf("StopBits(%d).String() = %q, want %q", n, n.String(), want)
		}
		b, err := n.MarshalText()
		if err != nil {
			t.Errorf("StopBits(%d).MarshalText(): %v", n, err)
			continue
		}
		if string(b) != want {
			t.Errorf("StopBits(%d).MarshalText() = %q, want %q", n, b, want)
		}
	}
}

func TestBaudRateStringAndMarshalTextNonZero(t *testing.T) {
	for _, rate := range []BaudRate{Baud1200, Baud115200, BaudRate(500000)} {
		b, err := rate.MarshalText()
		if err != nil {
			t.Errorf("BaudRate(%d).MarshalText(): %v", rate, err)
			continue
		}
		if string(b) != rate.String() {
			t.Errorf("MarshalText(%d) = %q, String = %q", rate, b, rate.String())
		}
	}
}

func TestDataBitsUnmarshalText(t *testing.T) {
	for _, want := range []DataBits{DataBits5, DataBits6, DataBits7, DataBits8} {
		var d DataBits
		if err := d.UnmarshalText([]byte(want.String())); err != nil {
			t.Errorf("UnmarshalText(%q): %v", want.String(), err)
			continue
		}
		if d != want {
			t.Errorf("UnmarshalText(%q) = %v, want %v", want.String(), d, want)
		}
	}
	for _, text := range []string{"", "abc", "9", "256"} {
		var d DataBits
		err := d.UnmarshalText([]byte(text))
		if err == nil {
			t.Errorf("UnmarshalText(%q) wanted error", text)
			continue
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("UnmarshalText(%q): want ErrInvalidConfig wrap, got %v", text, err)
		}
	}
}

func TestStopBitsUnmarshalText(t *testing.T) {
	for _, want := range []StopBits{StopBits1, StopBits2} {
		var s StopBits
		if err := s.UnmarshalText([]byte(want.String())); err != nil {
			t.Errorf("UnmarshalText(%q): %v", want.String(), err)
			continue
		}
		if s != want {
			t.Errorf("UnmarshalText(%q) = %v, want %v", want.String(), s, want)
		}
	}
	for _, text := range []string{"", "x", "0", "3"} {
		var s StopBits
		err := s.UnmarshalText([]byte(text))
		if err == nil {
			t.Errorf("UnmarshalText(%q) wanted error", text)
			continue
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("UnmarshalText(%q): want ErrInvalidConfig wrap, got %v", text, err)
		}
	}
}

func TestBaudRateUnmarshalText(t *testing.T) {
	var b BaudRate
	if err := b.UnmarshalText([]byte("19200")); err != nil {
		t.Fatalf("UnmarshalText(19200): %v", err)
	}
	if b != Baud19200 {
		t.Errorf("got %d, want Baud19200", b)
	}
	for _, text := range []string{"", "0", "-1", "not-a-number"} {
		var br BaudRate
		err := br.UnmarshalText([]byte(text))
		if err == nil {
			t.Errorf("UnmarshalText(%q) wanted error", text)
			continue
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("UnmarshalText(%q): want ErrInvalidConfig wrap, got %v", text, err)
		}
	}
}

func TestBaudRateUnmarshalTextRoundTrip(t *testing.T) {
	const custom = BaudRate(500000)
	b, err := custom.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	var out BaudRate
	if err := out.UnmarshalText(b); err != nil {
		t.Fatal(err)
	}
	if out != custom {
		t.Errorf("round-trip: got %d, want %d", out, custom)
	}
}
