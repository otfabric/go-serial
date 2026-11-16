// SPDX-License-Identifier: MIT

package serial

import (
	"sync"
	"time"
)

// ioGate lets Close end the Read and Write calls of the Windows port, which
// has no descriptor to wake them through. A call registers with begin and
// unregisters with end; shut refuses new calls, cancels the ones in
// progress and returns once they have ended, so the handle is never used
// after Close closed it. The POSIX backend has its own scheme with a wake
// pipe; the file has no build constraint so that its tests run on every
// platform.
type ioGate struct {
	mu     sync.Mutex
	closed bool
	active int
	idle   chan struct{} // made by shut, closed by end when the last call ends
}

// begin registers a call. It returns ErrClosed once shut has started.
func (g *ioGate) begin() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return ErrClosed
	}
	g.active++
	return nil
}

// end unregisters a call started with begin.
func (g *ioGate) end() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active--
	if g.active == 0 && g.idle != nil {
		close(g.idle)
		g.idle = nil
	}
}

// isClosed reports whether shut has started.
func (g *ioGate) isClosed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.closed
}

// shut refuses new calls and returns true once no call is active. While a
// call is active it runs cancel at once and again every interval: a call
// can have registered without having started the operation that cancel
// ends yet. shut returns false at once when it ran before.
func (g *ioGate) shut(cancel func(), every time.Duration) bool {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return false
	}
	g.closed = true
	if g.active == 0 {
		g.mu.Unlock()
		return true
	}
	idle := make(chan struct{})
	g.idle = idle
	g.mu.Unlock()

	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		cancel()
		select {
		case <-idle:
			return true
		case <-tick.C:
		}
	}
}
