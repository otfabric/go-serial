//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

// Chaos tests: randomized fault injection against the POSIX backend over
// pseudo-terminals. Every test logs its seed; reproduce a failure with
//
//	SERIAL_CHAOS_SEED=<seed> go test -run TestChaos -v
//
// and scale the run with SERIAL_CHAOS_ITERATIONS (default 1; -short keeps
// the per-test workload small).

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"
)

// chaosRand returns a seeded PRNG and logs the seed for reproduction.
func chaosRand(t *testing.T) *rand.Rand {
	t.Helper()
	seed := time.Now().UnixNano()
	if s := os.Getenv("SERIAL_CHAOS_SEED"); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			t.Fatalf("bad SERIAL_CHAOS_SEED %q: %v", s, err)
		}
		seed = v
	}
	t.Logf("chaos seed: %d (rerun with SERIAL_CHAOS_SEED=%d)", seed, seed)
	return rand.New(rand.NewSource(seed))
}

// chaosScale returns how many rounds a chaos test should run.
func chaosScale(t *testing.T, normal int) int {
	t.Helper()
	n := normal
	if testing.Short() {
		n = max(1, normal/5)
	}
	if s := os.Getenv("SERIAL_CHAOS_ITERATIONS"); s != "" {
		if m, err := strconv.Atoi(s); err == nil && m > 0 {
			n *= m
		}
	}
	return n
}

func randBytes(r *rand.Rand, n int) []byte {
	b := make([]byte, n)
	r.Read(b)
	return b
}

// chaosRelay copies src to dst, splitting the stream into random-sized
// fragments with random pauses — a noisy, bursty line.
func chaosRelay(dst, src *os.File, seed int64, done chan<- struct{}) {
	defer close(done)
	r := rand.New(rand.NewSource(seed))
	buf := make([]byte, 512)
	for {
		n, err := src.Read(buf[:1+r.Intn(len(buf))])
		for off := 0; off < n; {
			chunk := min(n-off, 1+r.Intn(64))
			if _, werr := dst.Write(buf[off : off+chunk]); werr != nil {
				return
			}
			off += chunk
			if r.Intn(8) == 0 {
				time.Sleep(time.Duration(r.Intn(300)) * time.Microsecond)
			}
		}
		if err != nil {
			return
		}
	}
}

// TestChaosFullDuplexFragmentingRelay runs one reader and one writer on each
// end of a link (the documented full-duplex contract) while the line
// fragments and delays traffic at random. Every byte must arrive in order.
func TestChaosFullDuplexFragmentingRelay(t *testing.T) {
	r := chaosRand(t)
	for round := 0; round < chaosScale(t, 3); round++ {
		m1, s1, err := openPTY()
		if err != nil {
			t.Skipf("skip when PTY cannot be created: %v", err)
		}
		m2, s2, err := openPTY()
		if err != nil {
			_ = m1.Close()
			t.Skipf("skip when PTY cannot be created: %v", err)
		}
		cfg := Config{BaudRate: Baud115200, Timeout: 2 * time.Second}
		cfg.Address = s1
		p1, err := Open(&cfg)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Address = s2
		p2, err := Open(&cfg)
		if err != nil {
			t.Fatal(err)
		}
		done1, done2 := make(chan struct{}), make(chan struct{})
		go chaosRelay(m2, m1, r.Int63(), done1)
		go chaosRelay(m1, m2, r.Int63(), done2)

		size := 4096 + r.Intn(60*1024)
		if testing.Short() {
			size = 1024 + r.Intn(4096)
		}
		payload1, payload2 := randBytes(r, size), randBytes(r, size)
		seeds := [4]int64{r.Int63(), r.Int63(), r.Int63(), r.Int63()}

		var wg sync.WaitGroup
		errs := make(chan error, 4)
		writer := func(p Port, data []byte, seed int64) {
			defer wg.Done()
			wr := rand.New(rand.NewSource(seed))
			for off := 0; off < len(data); {
				chunk := min(len(data)-off, 1+wr.Intn(700))
				n, err := p.Write(data[off : off+chunk])
				if n < 0 || n > chunk {
					errs <- fmt.Errorf("Write returned n=%d for %d bytes", n, chunk)
					return
				}
				if err != nil {
					errs <- fmt.Errorf("write: %w", err)
					return
				}
				off += n
			}
		}
		reader := func(p Port, want []byte, seed int64) {
			defer wg.Done()
			rr := rand.New(rand.NewSource(seed))
			got := make([]byte, 0, len(want))
			for len(got) < len(want) {
				buf := make([]byte, 1+rr.Intn(900))
				n, err := p.Read(buf)
				if n < 0 || n > len(buf) {
					errs <- fmt.Errorf("Read returned n=%d for buffer of %d", n, len(buf))
					return
				}
				got = append(got, buf[:n]...)
				if err != nil {
					errs <- fmt.Errorf("read after %d/%d bytes: %w", len(got), len(want), err)
					return
				}
			}
			if !bytes.Equal(got, want) {
				i := 0
				for i < len(got) && got[i] == want[i] {
					i++
				}
				errs <- fmt.Errorf("stream corrupted at byte %d of %d", i, len(want))
			}
		}
		wg.Add(4)
		go writer(p1, payload1, seeds[0])
		go writer(p2, payload2, seeds[1])
		go reader(p2, payload1, seeds[2])
		go reader(p1, payload2, seeds[3])
		wg.Wait()
		close(errs)

		_ = p1.Close()
		_ = p2.Close()
		_ = m1.Close()
		_ = m2.Close()
		<-done1
		<-done2
		for err := range errs {
			t.Errorf("round %d (%d bytes each way): %v", round, size, err)
		}
		if t.Failed() {
			return
		}
	}
}

// TestChaosRandomHangup pulls the plug at a random moment while data is in
// flight. Read must never report n < 0, never invent bytes, and must end
// with io.EOF or EIO instead of spinning on timeouts.
func TestChaosRandomHangup(t *testing.T) {
	r := chaosRand(t)
	for round := 0; round < chaosScale(t, 25); round++ {
		master, slave, err := openPTY()
		if err != nil {
			t.Skipf("skip when PTY cannot be created: %v", err)
		}
		p, err := Open(&Config{Address: slave, Timeout: 20 * time.Millisecond})
		if err != nil {
			_ = master.Close()
			t.Fatal(err)
		}
		sent := randBytes(r, r.Intn(8192))
		cut := r.Intn(len(sent) + 1)
		wseed := r.Int63()
		go func() {
			wr := rand.New(rand.NewSource(wseed))
			for off := 0; off < cut; {
				chunk := min(cut-off, 1+wr.Intn(256))
				if _, err := master.Write(sent[off : off+chunk]); err != nil {
					break
				}
				off += chunk
				if wr.Intn(4) == 0 {
					time.Sleep(time.Duration(wr.Intn(2000)) * time.Microsecond)
				}
			}
			_ = master.Close()
		}()

		var got []byte
		buf := make([]byte, 1+r.Intn(512))
		deadline := time.Now().Add(5 * time.Second)
		var final error
		for time.Now().Before(deadline) {
			n, err := p.Read(buf)
			if n < 0 || n > len(buf) {
				t.Fatalf("round %d: Read returned n=%d (buffer %d)", round, n, len(buf))
			}
			got = append(got, buf[:n]...)
			if err != nil && !errors.Is(err, ErrTimeout) {
				final = err
				break
			}
		}
		_ = p.Close()
		if final == nil {
			t.Fatalf("round %d: no hangup error within 5s (got %d of %d bytes)", round, len(got), cut)
		}
		if !errors.Is(final, io.EOF) && !errors.Is(final, syscall.EIO) {
			t.Fatalf("round %d: hangup reported as %v, want io.EOF or EIO", round, final)
		}
		if !bytes.HasPrefix(sent[:cut], got) {
			t.Fatalf("round %d: received %d bytes that are not a prefix of the %d sent", round, len(got), cut)
		}
	}
}

var chaosBauds = []BaudRate{0, 50, 300, 1200, 9600, 19200, 38400, 57600, 115200, 230400, 7, 12345, -9600, 1 << 40}

func randomChaosConfig(r *rand.Rand, address string) Config {
	pick := func(n int) bool { return r.Intn(n) == 0 }
	c := Config{
		Address:  address,
		BaudRate: chaosBauds[r.Intn(len(chaosBauds))],
		DataBits: DataBits(r.Intn(11)),
		StopBits: StopBits(r.Intn(4)),
		Parity:   []Parity{"", ParityNone, ParityEven, ParityOdd, "M", "e", "NONE", "\x00"}[r.Intn(8)],
		Timeout:  time.Duration(r.Intn(50)) * time.Millisecond,
	}
	if pick(10) {
		c.Timeout = -time.Duration(1 + r.Intn(1000))
	}
	if pick(4) {
		c.RS485 = RS485Config{
			Enabled:            true,
			DelayRtsBeforeSend: time.Duration(r.Intn(20)-2) * time.Millisecond,
			DelayRtsAfterSend:  time.Duration(r.Intn(20)-2) * time.Millisecond,
			RtsHighDuringSend:  pick(2),
			RtsHighAfterSend:   pick(2),
			RxDuringTx:         pick(2),
		}
	}
	return c
}

// TestChaosRandomConfigs throws random, mostly-garbage configurations at Open
// on a live pty. Open must never panic, must agree with Validate and the
// platform baud table, must reject bad input with ErrInvalidConfig, and
// every port it does return must carry data and close cleanly.
func TestChaosRandomConfigs(t *testing.T) {
	r := chaosRand(t)
	opened := 0
	for round := 0; round < chaosScale(t, 300); round++ {
		master, slave, err := openPTY()
		if err != nil {
			t.Skipf("skip when PTY cannot be created: %v", err)
		}
		cfg := randomChaosConfig(r, slave)
		if r.Intn(15) == 0 {
			cfg.Address = []string{"", "/nonexistent/tty", slave + "\x00junk", os.DevNull}[r.Intn(4)]
		}
		desc := fmt.Sprintf("round %d: %+v", round, cfg)

		validErr := cfg.Validate()
		baudOK := isBaudRateSupported(cfg.Normalized().BaudRate)
		p, err := Open(&cfg)
		switch {
		case validErr != nil || !baudOK:
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("%s: Validate=%v baudOK=%v but Open err=%v", desc, validErr, baudOK, err)
			}
		case cfg.Address != slave || cfg.RS485.Enabled:
			// Wrong device or RS-485 on a pty: OS-level failure is fine,
			// but it must be a real error, never a config error.
			if err != nil && errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("%s: valid config rejected as invalid: %v", desc, err)
			}
		default:
			if err != nil {
				t.Fatalf("%s: valid config failed to open: %v", desc, err)
			}
		}
		if err == nil {
			opened++
			msg := randBytes(r, 1+r.Intn(64))
			if _, werr := master.Write(msg); werr != nil {
				t.Fatalf("%s: master write: %v", desc, werr)
			}
			got := make([]byte, 0, len(msg))
			buf := make([]byte, 128)
			for deadline := time.Now().Add(2 * time.Second); len(got) < len(msg) && time.Now().Before(deadline); {
				n, rerr := p.Read(buf)
				got = append(got, buf[:n]...)
				if rerr != nil && !errors.Is(rerr, ErrTimeout) {
					t.Fatalf("%s: read: %v", desc, rerr)
				}
			}
			if !bytes.Equal(got, msg) {
				t.Fatalf("%s: read %q, want %q", desc, got, msg)
			}
			if cerr := p.Close(); cerr != nil {
				t.Fatalf("%s: close: %v", desc, cerr)
			}
		}
		_ = master.Close()
	}
	t.Logf("%d random configs opened successfully", opened)
}

// openFDs counts this process's open descriptors, or -1 if unknown.
func openFDs() int {
	for _, dir := range []string{"/proc/self/fd", "/dev/fd"} {
		if ents, err := os.ReadDir(dir); err == nil {
			return len(ents)
		}
	}
	return -1
}

// TestChaosOpenCloseChurn hammers Open/Close from many goroutines, mixing
// successful opens, failed opens (missing device, non-tty, RS-485 refused)
// and I/O, then checks that no file descriptor leaked.
func TestChaosOpenCloseChurn(t *testing.T) {
	r := chaosRand(t)
	before := openFDs()
	workers := 8
	cycles := chaosScale(t, 40)
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for w := 0; w < workers; w++ {
		seed := r.Int63()
		wg.Add(1)
		go func() {
			defer wg.Done()
			wr := rand.New(rand.NewSource(seed))
			for i := 0; i < cycles; i++ {
				master, slave, err := openPTY()
				if err != nil {
					errs <- err
					return
				}
				cfg := Config{Address: slave, BaudRate: chaosBauds[1+wr.Intn(9)], Timeout: time.Millisecond}
				switch wr.Intn(6) {
				case 0:
					cfg.Address = os.DevNull // opens, then termios fails
				case 1:
					cfg.Address += "-gone" // ENOENT
				case 2:
					cfg.RS485.Enabled = true // ioctl refused on pty
				}
				p, err := Open(&cfg)
				if err == nil {
					if wr.Intn(2) == 0 {
						_, _ = p.Write([]byte("ping"))
						_, _ = p.Read(make([]byte, 8))
					}
					if wr.Intn(3) == 0 {
						_ = master.Close() // hang up before Close
					}
					if cerr := p.Close(); cerr != nil && !errors.Is(cerr, syscall.EIO) {
						errs <- fmt.Errorf("close: %w", cerr)
					}
				}
				_ = master.Close()
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	after := openFDs()
	if before >= 0 && after > before {
		t.Fatalf("fd leak: %d open before churn, %d after", before, after)
	}
}

// TestChaosTermiosRestoredAfterAbuse checks that whatever settings a port
// applied, Close puts the device's original termios back — even after
// writes, reads, timeouts and a hangup in between.
func TestChaosTermiosRestoredAfterAbuse(t *testing.T) {
	r := chaosRand(t)
	for round := 0; round < chaosScale(t, 20); round++ {
		master, slave, err := openPTY()
		if err != nil {
			t.Skipf("skip when PTY cannot be created: %v", err)
		}
		// Keep the slave open independently so its termios survives
		// between the port's open and close and can be inspected.
		observer, err := os.OpenFile(slave, os.O_RDWR|syscall.O_NOCTTY, 0)
		if err != nil {
			_ = master.Close()
			t.Fatal(err)
		}
		var orig, after syscall.Termios
		if err := tcgetattr(int(observer.Fd()), &orig); err != nil {
			t.Fatal(err)
		}
		cfg := Config{
			Address:  slave,
			BaudRate: chaosBauds[1+r.Intn(9)],
			DataBits: DataBits(5 + r.Intn(4)),
			StopBits: StopBits(1 + r.Intn(2)),
			Parity:   []Parity{ParityNone, ParityEven, ParityOdd}[r.Intn(3)],
			Timeout:  time.Millisecond,
		}
		p, err := Open(&cfg)
		if err != nil {
			t.Fatalf("round %d %+v: %v", round, cfg, err)
		}
		for i := r.Intn(5); i > 0; i-- {
			_, _ = p.Write(randBytes(r, 1+r.Intn(32)))
			_, _ = p.Read(make([]byte, 16))
		}
		_ = p.Close()
		if err := tcgetattr(int(observer.Fd()), &after); err != nil {
			t.Fatal(err)
		}
		if after != orig {
			t.Fatalf("round %d %+v: termios not restored:\n got  %+v\n want %+v", round, cfg, after, orig)
		}
		_ = observer.Close()
		_ = master.Close()
	}
}

// TestChaosWriteBackpressure writes far more than the tty output buffer
// holds while the far end drains slowly and at random. Write must block
// and deliver everything rather than fail with EAGAIN or report n < 0.
func TestChaosWriteBackpressure(t *testing.T) {
	r := chaosRand(t)
	master, slave, err := openPTY()
	if err != nil {
		t.Skipf("skip when PTY cannot be created: %v", err)
	}
	defer func() { _ = master.Close() }()
	p, err := Open(&Config{Address: slave})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	payload := randBytes(r, 256*1024)
	if testing.Short() {
		payload = payload[:48*1024]
	}
	got := make(chan []byte, 1)
	dseed := r.Int63()
	go func() {
		dr := rand.New(rand.NewSource(dseed))
		time.Sleep(100 * time.Millisecond) // let the buffer fill up first
		var acc []byte
		buf := make([]byte, 4096)
		for len(acc) < len(payload) {
			n, err := master.Read(buf[:1+dr.Intn(len(buf))])
			acc = append(acc, buf[:n]...)
			if err != nil {
				break
			}
			if dr.Intn(10) == 0 {
				time.Sleep(time.Duration(dr.Intn(3)) * time.Millisecond)
			}
		}
		got <- acc
	}()
	n, err := p.Write(payload)
	if err != nil || n != len(payload) {
		t.Fatalf("Write = %d, %v; want %d, nil", n, err, len(payload))
	}
	select {
	case acc := <-got:
		if !bytes.Equal(acc, payload) {
			t.Fatalf("far end received %d bytes, mismatch with %d sent", len(acc), len(payload))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("far end never received the full payload")
	}
}

func TestWriteEmpty(t *testing.T) {
	p := &port{fd: closedFD(t)}
	if n, err := p.Write(nil); n != 0 || err != nil {
		t.Fatalf("Write(nil) = %d, %v; want 0, nil", n, err)
	}
}
