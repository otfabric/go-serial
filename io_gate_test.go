// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// gatedCall runs one call through g in a goroutine: begin, then op, then
// end. The returned channel receives begin's error, or nil after op.
func gatedCall(g *ioGate, op func()) <-chan error {
	done := make(chan error, 1)
	go func() {
		if err := g.begin(); err != nil {
			done <- err
			return
		}
		op()
		g.end()
		done <- nil
	}()
	return done
}

// TestIOGateShutWithoutCalls: shut returns at once without cancelling when
// no call is active, a second shut returns false, and begin then fails with
// ErrClosed.
func TestIOGateShutWithoutCalls(t *testing.T) {
	var g ioGate
	cancels := 0
	if !g.shut(func() { cancels++ }, time.Hour) {
		t.Fatal("first shut = false")
	}
	if g.shut(func() { cancels++ }, time.Hour) {
		t.Fatal("second shut = true")
	}
	if cancels != 0 {
		t.Fatalf("cancel ran %d times without a call", cancels)
	}
	if err := g.begin(); !errors.Is(err, ErrClosed) {
		t.Fatalf("begin after shut = %v, want ErrClosed", err)
	}
	if !g.isClosed() {
		t.Fatal("isClosed = false after shut")
	}
}

// TestIOGateShutCancelsAndWaits: shut cancels a call in progress and does
// not return until the call has ended; a second shut meanwhile returns
// false at once, and a new call fails with ErrClosed.
func TestIOGateShutCancelsAndWaits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var g ioGate
		cancelled := make(chan struct{})
		release := make(chan struct{})
		call := gatedCall(&g, func() { <-cancelled; <-release })
		synctest.Wait() // the call is in op

		var cancels atomic.Int32
		shut := make(chan bool, 1)
		go func() {
			shut <- g.shut(func() {
				if cancels.Add(1) == 1 {
					close(cancelled)
				}
			}, time.Hour)
		}()
		synctest.Wait() // the call was cancelled and holds in op; shut waits
		select {
		case <-shut:
			t.Fatal("shut returned while the call was active")
		default:
		}
		if g.shut(func() { t.Error("second shut cancelled") }, time.Hour) {
			t.Fatal("second shut = true")
		}
		if err := <-gatedCall(&g, func() { t.Error("a call ran after shut") }); !errors.Is(err, ErrClosed) {
			t.Fatalf("call after shut = %v, want ErrClosed", err)
		}
		close(release)
		if err := <-call; err != nil {
			t.Fatalf("call = %v", err)
		}
		if !<-shut {
			t.Fatal("first shut = false")
		}
		if n := cancels.Load(); n != 1 {
			t.Fatalf("cancel ran %d times, want 1", n)
		}
	})
}

// TestIOGateShutCancelsAgain: a call that the first cancel missed (it had
// registered but not yet started its operation) is cancelled by a later
// one, every interval, until it ends.
func TestIOGateShutCancelsAgain(t *testing.T) {
	const every = 10 * time.Millisecond
	synctest.Test(t, func(t *testing.T) {
		var g ioGate
		var cancels atomic.Int32
		third := make(chan struct{})
		call := gatedCall(&g, func() { <-third })
		synctest.Wait()

		start := time.Now()
		ok := g.shut(func() {
			if cancels.Add(1) == 3 {
				close(third)
			}
		}, every)
		if !ok {
			t.Fatal("shut = false")
		}
		if err := <-call; err != nil {
			t.Fatalf("call = %v", err)
		}
		if n := cancels.Load(); n != 3 {
			t.Fatalf("cancel ran %d times, want 3", n)
		}
		if d := time.Since(start); d != 2*every {
			t.Fatalf("shut took %v, want %v", d, 2*every)
		}
	})
}

// TestIOGateSeveralCalls: shut waits for every active call, and returns
// after the last one ended.
func TestIOGateSeveralCalls(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var g ioGate
		releases := []chan struct{}{make(chan struct{}), make(chan struct{})}
		var calls []<-chan error
		for _, r := range releases {
			calls = append(calls, gatedCall(&g, func() { <-r }))
		}
		synctest.Wait()
		shut := make(chan bool, 1)
		go func() { shut <- g.shut(func() {}, time.Hour) }()
		close(releases[0])
		if err := <-calls[0]; err != nil {
			t.Fatalf("call 0 = %v", err)
		}
		synctest.Wait()
		select {
		case <-shut:
			t.Fatal("shut returned with one call still active")
		default:
		}
		close(releases[1])
		if err := <-calls[1]; err != nil {
			t.Fatalf("call 1 = %v", err)
		}
		if !<-shut {
			t.Fatal("shut = false")
		}
	})
}
