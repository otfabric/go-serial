//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// TestChaosModemLinesDuringClose drives SetDTR, SetRTS and ModemStatus from
// several goroutines while a Read and a Write run on the same port, and
// Close lands at a random moment. No call may panic or reach a closed
// descriptor; SetRTS under RTS/CTS flow control returns ErrLineInUse or
// ErrClosed and nothing else; any other error is ErrClosed or the operating
// system's (ENOTTY from a Linux PTY, which has no modem lines); a call that
// starts after Close returned gets ErrClosed; and no descriptor leaks.
func TestChaosModemLinesDuringClose(t *testing.T) {
	r := chaosRand(t)
	before := openFDs()
	for round := 0; round < chaosScale(t, 30); round++ {
		master, slave, err := openPTY()
		if err != nil {
			skipNoPTY(t, err)
		}
		flow := []FlowControl{FlowNone, FlowRTSCTS, FlowXONXOFF}[r.Intn(3)]
		p, err := Open(&Config{Address: slave, FlowControl: flow, Timeout: time.Millisecond})
		if err != nil {
			t.Fatalf("round %d %s: %v", round, flow, err)
		}
		lines := p.(ModemLines)
		desc := fmt.Sprintf("round %d %s", round, flow)

		drained := make(chan struct{})
		go func() { // the far end: take whatever the writer sends
			defer close(drained)
			_, _ = io.Copy(io.Discard, master)
		}()

		var closed atomic.Bool // set once Close has returned
		var wg sync.WaitGroup
		errs := make(chan error, 64)
		report := func(err error) {
			select {
			case errs <- err:
			default:
			}
		}
		wg.Add(2)
		go func() { // reader
			defer wg.Done()
			buf := make([]byte, 16)
			for {
				_, err := p.Read(buf)
				if errors.Is(err, ErrClosed) {
					return
				}
				if err != nil && !errors.Is(err, ErrTimeout) {
					report(fmt.Errorf("%s: Read: %w", desc, err))
					return
				}
			}
		}()
		go func() { // writer
			defer wg.Done()
			for {
				if _, err := p.Write([]byte("chaos")); err != nil {
					if !errors.Is(err, ErrClosed) {
						report(fmt.Errorf("%s: Write: %w", desc, err))
					}
					return
				}
			}
		}()
		for w := 0; w < 4; w++ {
			seed := r.Int63()
			wg.Add(1)
			go func() { // modem lines
				defer wg.Done()
				wr := rand.New(rand.NewSource(seed))
				for {
					after := closed.Load()
					var err error
					op := wr.Intn(3)
					switch op {
					case 0:
						err = lines.SetDTR(wr.Intn(2) == 0)
					case 1:
						err = lines.SetRTS(wr.Intn(2) == 0)
					default:
						_, err = lines.ModemStatus()
					}
					var errno syscall.Errno
					switch {
					case errors.Is(err, ErrClosed):
						return
					case after:
						report(fmt.Errorf("%s: op %d after Close returned: %v; want ErrClosed", desc, op, err))
						return
					case op == 1 && flow == FlowRTSCTS:
						if !errors.Is(err, ErrLineInUse) {
							report(fmt.Errorf("%s: SetRTS under RTS/CTS: %v; want ErrLineInUse", desc, err))
							return
						}
					case errors.Is(err, ErrLineInUse):
						report(fmt.Errorf("%s: op %d: ErrLineInUse with a free line", desc, op))
						return
					case err != nil && !errors.As(err, &errno):
						report(fmt.Errorf("%s: op %d: %v; want nil, ErrClosed or an errno", desc, op, err))
						return
					}
				}
			}()
		}

		time.Sleep(time.Duration(r.Intn(3000)) * time.Microsecond)
		if err := p.Close(); err != nil {
			t.Fatalf("%s: Close: %v", desc, err)
		}
		closed.Store(true)
		wg.Wait()
		_ = master.Close()
		<-drained
		close(errs)
		for err := range errs {
			t.Error(err)
		}
		if t.Failed() {
			return
		}
	}
	if after := openFDs(); before >= 0 && after > before {
		t.Fatalf("fd leak: %d open before, %d after", before, after)
	}
}
