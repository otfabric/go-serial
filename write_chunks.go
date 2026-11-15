// SPDX-License-Identifier: MIT

package serial

import (
	"fmt"
	"io"
)

// writeChunks writes b through write, which takes at most limit bytes per
// call, until all of b is written, a call fails, or a call comes back short.
// It is the loop of the Windows Write, where write is one WriteFile call;
// the file has no build constraint so that its tests run on every platform.
//
// A call that reports fewer bytes than it was given, without an error, ends
// the write. With timed set (Config.Timeout > 0, which is the write time-out
// as well) the error is ErrTimeout, otherwise it wraps io.ErrShortWrite. n
// counts the bytes written in every case, so n < len(b) comes only with an
// error, as io.Writer requires. An empty b makes one call, as a direct
// WriteFile does.
func writeChunks(b []byte, limit uint64, timed bool, write func([]byte) (int, error)) (n int, err error) {
	for {
		chunk := b[n:]
		if uint64(len(chunk)) > limit {
			chunk = chunk[:limit]
		}
		m, err := write(chunk)
		if err != nil {
			return n, fmt.Errorf("serial: write: %w", err)
		}
		if m < 0 || m > len(chunk) {
			return n, fmt.Errorf("serial: write: count %d reported for %d bytes", m, len(chunk))
		}
		n += m
		switch {
		case m < len(chunk) && timed:
			return n, ErrTimeout
		case m < len(chunk):
			return n, fmt.Errorf("serial: write: %w", io.ErrShortWrite)
		case n == len(b):
			return n, nil
		}
	}
}
