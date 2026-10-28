//go:build linux

// SPDX-License-Identifier: MIT

package serial

import (
	"bytes"
	"errors"
	"os"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

// signalStorm repeatedly delivers SIGURG to one OS thread until stop is
// closed. The Go runtime uses SIGURG for goroutine preemption, so it is
// always handled and safe to send spuriously; each delivery interrupts a
// blocking poll/select on that thread with EINTR.
func signalStorm(t *testing.T, tid int, stop <-chan struct{}) *sync.WaitGroup {
	t.Helper()
	pid := os.Getpid()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = syscall.Tgkill(pid, tid, syscall.SIGURG)
			time.Sleep(50 * time.Microsecond)
		}
	}()
	return &wg
}

// readOnLockedThread runs Read on a dedicated OS thread, reporting the
// thread id before it starts so a storm can target exactly that thread.
func readOnLockedThread(p Port, buf []byte) (tid <-chan int, result <-chan [2]any) {
	tidc := make(chan int, 1)
	res := make(chan [2]any, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		tidc <- syscall.Gettid()
		time.Sleep(5 * time.Millisecond) // let the storm start first
		n, err := p.Read(buf)
		res <- [2]any{n, err}
	}()
	return tidc, res
}

// TestChaosSignalStormTimeout: a reader bombarded with signals must still
// time out on schedule — not early (EINTR treated as timeout or error) and
// not late (each retry restarting the full timeout).
func TestChaosSignalStormTimeout(t *testing.T) {
	master, slave, err := openPTY()
	if err != nil {
		t.Skipf("skip when PTY cannot be created: %v", err)
	}
	defer func() { _ = master.Close() }()
	const timeout = 250 * time.Millisecond
	p, err := Open(&Config{Address: slave, Timeout: timeout})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	tidc, res := readOnLockedThread(p, make([]byte, 8))
	stop := make(chan struct{})
	storm := signalStorm(t, <-tidc, stop)
	start := time.Now()
	out := <-res
	elapsed := time.Since(start)
	close(stop)
	storm.Wait()

	if n, err := out[0].(int), out[1]; n != 0 || !errors.Is(err.(error), ErrTimeout) {
		t.Fatalf("Read under signal storm = %d, %v; want 0, ErrTimeout", n, err)
	}
	if elapsed < timeout-10*time.Millisecond || elapsed > timeout+750*time.Millisecond {
		t.Fatalf("timeout of %v took %v under signal storm", timeout, elapsed)
	}
}

// TestChaosSignalStormBlockingRead: a blocking Read (no timeout) under a
// signal storm must keep waiting and then deliver the data intact.
func TestChaosSignalStormBlockingRead(t *testing.T) {
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

	buf := make([]byte, 64)
	tidc, res := readOnLockedThread(p, buf)
	stop := make(chan struct{})
	storm := signalStorm(t, <-tidc, stop)
	time.Sleep(150 * time.Millisecond)
	msg := []byte("survived the storm")
	if _, err := master.Write(msg); err != nil {
		t.Fatal(err)
	}
	var out [2]any
	select {
	case out = <-res:
	case <-time.After(5 * time.Second):
		close(stop)
		storm.Wait()
		t.Fatal("blocking Read never returned data under signal storm")
	}
	close(stop)
	storm.Wait()
	n := out[0].(int)
	if out[1] != nil || !bytes.Equal(buf[:n], msg) {
		t.Fatalf("Read = %q, %v; want %q", buf[:n], out[1], msg)
	}
}

// TestChaosHighFileDescriptor opens the port above FD_SETSIZE (1024), as
// happens in any busy server. select(2) cannot handle such descriptors;
// Read must still wait for data, time out properly and never report a
// bogus EOF.
func TestChaosHighFileDescriptor(t *testing.T) {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil || lim.Cur < 1200 {
		t.Skipf("RLIMIT_NOFILE too low for this test (%d)", lim.Cur)
	}
	var held []int
	defer func() {
		for _, fd := range held {
			_ = syscall.Close(fd)
		}
	}()
	for {
		fd, err := syscall.Open(os.DevNull, syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, fd)
		if fd > 1100 {
			break
		}
	}
	master, slave, err := openPTY()
	if err != nil {
		t.Skipf("skip when PTY cannot be created: %v", err)
	}
	defer func() { _ = master.Close() }()
	p, err := Open(&Config{Address: slave, Timeout: 150 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	if fd := p.(*port).fd; fd < syscall.FD_SETSIZE {
		t.Fatalf("port fd %d is not above FD_SETSIZE; test setup broken", fd)
	}

	buf := make([]byte, 32)
	start := time.Now()
	n, err := p.Read(buf)
	if n != 0 || !errors.Is(err, ErrTimeout) {
		t.Fatalf("idle Read on high fd = %d, %v; want 0, ErrTimeout", n, err)
	}
	if time.Since(start) < 140*time.Millisecond {
		t.Fatalf("idle Read on high fd returned after %v, before the timeout", time.Since(start))
	}

	go func() {
		time.Sleep(30 * time.Millisecond)
		_, _ = master.Write([]byte("high fd ok"))
	}()
	n, err = p.Read(buf)
	if err != nil || string(buf[:n]) != "high fd ok" {
		t.Fatalf("Read on high fd = %q, %v; want %q", buf[:n], err, "high fd ok")
	}
	if _, err := p.Write([]byte("back")); err != nil {
		t.Fatalf("Write on high fd: %v", err)
	}
}
