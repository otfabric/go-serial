// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// TestWriteChunks covers the loop of the Windows Write on every platform:
// the split at the per-call limit, and a short count, which always comes
// with an error.
func TestWriteChunks(t *testing.T) {
	errDevice := errors.New("device error")
	type call struct {
		n   int // count the call reports; -1 means all of the chunk
		err error
	}
	all := call{n: -1}
	tests := []struct {
		name      string
		size      int
		limit     uint64
		timed     bool
		calls     []call // one per expected call
		wantN     int
		wantErr   error  // matched with errors.Is; nil means no error
		wantText  string // substring of the error text, if set
		wantSizes []int  // chunk length of every call
	}{
		{name: "empty buffer makes one call", size: 0, limit: 4, calls: []call{all}, wantN: 0, wantSizes: []int{0}},
		{name: "one byte", size: 1, limit: 4, calls: []call{all}, wantN: 1, wantSizes: []int{1}},
		{name: "limit minus one", size: 3, limit: 4, calls: []call{all}, wantN: 3, wantSizes: []int{3}},
		{name: "at the limit", size: 4, limit: 4, calls: []call{all}, wantN: 4, wantSizes: []int{4}},
		{name: "limit plus one", size: 5, limit: 4, calls: []call{all, all}, wantN: 5, wantSizes: []int{4, 1}},
		{name: "several chunks", size: 10, limit: 4, calls: []call{all, all, all}, wantN: 10, wantSizes: []int{4, 4, 2}},
		{name: "limit of one", size: 3, limit: 1, calls: []call{all, all, all}, wantN: 3, wantSizes: []int{1, 1, 1}},
		{name: "limit above the buffer", size: 3, limit: 1 << 40, calls: []call{all}, wantN: 3, wantSizes: []int{3}},

		{name: "short count with a time-out", size: 8, limit: 16, timed: true, calls: []call{{n: 5}}, wantN: 5, wantErr: ErrTimeout, wantSizes: []int{8}},
		{name: "nothing written with a time-out", size: 8, limit: 16, timed: true, calls: []call{{n: 0}}, wantN: 0, wantErr: ErrTimeout, wantSizes: []int{8}},
		{name: "short count in the second chunk", size: 10, limit: 4, timed: true, calls: []call{all, {n: 1}}, wantN: 5, wantErr: ErrTimeout, wantSizes: []int{4, 4}},
		{name: "short count without a time-out", size: 8, limit: 16, calls: []call{{n: 5}}, wantN: 5, wantErr: io.ErrShortWrite, wantText: "serial: write: ", wantSizes: []int{8}},
		{name: "nothing written without a time-out", size: 8, limit: 16, calls: []call{{n: 0}}, wantN: 0, wantErr: io.ErrShortWrite, wantSizes: []int{8}},

		{name: "error on the first call", size: 8, limit: 16, timed: true, calls: []call{{err: errDevice}}, wantN: 0, wantErr: errDevice, wantText: "serial: write: device error", wantSizes: []int{8}},
		{name: "error on a later call", size: 10, limit: 4, calls: []call{all, all, {err: errDevice}}, wantN: 8, wantErr: errDevice, wantSizes: []int{4, 4, 2}},
		{name: "error on an empty buffer", size: 0, limit: 4, calls: []call{{err: errDevice}}, wantN: 0, wantErr: errDevice, wantSizes: []int{0}},
		{name: "error wins over the count", size: 8, limit: 16, calls: []call{{n: 8, err: errDevice}}, wantN: 0, wantErr: errDevice, wantSizes: []int{8}},

		{name: "count above the chunk", size: 8, limit: 4, calls: []call{{n: 5}}, wantN: 0, wantText: "count 5 reported for 4 bytes", wantSizes: []int{4}},
		{name: "negative count", size: 8, limit: 16, calls: []call{{n: -2}}, wantN: 0, wantText: "count -2 reported for 8 bytes", wantSizes: []int{8}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, tt.size)
			for i := range buf {
				buf[i] = byte(i)
			}
			var sizes []int
			var written []byte
			n, err := writeChunks(buf, tt.limit, tt.timed, func(chunk []byte) (int, error) {
				if len(sizes) >= len(tt.calls) {
					t.Fatalf("call %d not expected (chunk of %d bytes)", len(sizes)+1, len(chunk))
				}
				c := tt.calls[len(sizes)]
				sizes = append(sizes, len(chunk))
				if c.n == -1 {
					c.n = len(chunk)
				}
				if c.err == nil && c.n >= 0 && c.n <= len(chunk) {
					written = append(written, chunk[:c.n]...)
				}
				return c.n, c.err
			})
			if n != tt.wantN {
				t.Errorf("n = %d, want %d", n, tt.wantN)
			}
			if n < 0 || n > len(buf) {
				t.Errorf("n = %d outside 0..%d", n, len(buf))
			}
			if n < len(buf) && err == nil {
				t.Errorf("n = %d of %d with a nil error", n, len(buf))
			}
			switch {
			case tt.wantErr == nil && tt.wantText == "":
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			case tt.wantErr != nil && !errors.Is(err, tt.wantErr):
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			case err == nil || !strings.Contains(err.Error(), tt.wantText):
				t.Errorf("err = %v, want text %q", err, tt.wantText)
			}
			if len(sizes) != len(tt.wantSizes) {
				t.Fatalf("chunk sizes = %v, want %v", sizes, tt.wantSizes)
			}
			for i := range sizes {
				if sizes[i] != tt.wantSizes[i] {
					t.Fatalf("chunk sizes = %v, want %v", sizes, tt.wantSizes)
				}
			}
			if string(written) != string(buf[:n]) {
				t.Errorf("bytes handed to write = %v, want the first %d of the buffer", written, n)
			}
		})
	}
}

// TestWriteChunksTimeoutIsErrTimeout: the time-out is the sentinel itself,
// as from Read.
func TestWriteChunksTimeoutIsErrTimeout(t *testing.T) {
	_, err := writeChunks(make([]byte, 2), 8, true, func([]byte) (int, error) { return 1, nil })
	if err != ErrTimeout {
		t.Fatalf("err = %#v, want ErrTimeout itself", err)
	}
}
