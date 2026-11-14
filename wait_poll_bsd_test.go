//go:build freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"math"
	"testing"
	"time"
)

func TestPollTimeoutMs(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want int
	}{
		{-1, -1},
		{0, 0},
		{time.Nanosecond, 1},
		{time.Millisecond, 1},
		{time.Millisecond + 1, 2},
		{math.MaxInt32 * time.Millisecond, math.MaxInt32},
		{(math.MaxInt32 + 1) * time.Millisecond, math.MaxInt32},
		{1 << 32 * time.Millisecond, math.MaxInt32},
		{math.MaxInt64, math.MaxInt32},
	} {
		if got := pollTimeoutMs(tc.d); got != tc.want {
			t.Errorf("pollTimeoutMs(%v) = %d, want %d", tc.d, got, tc.want)
		}
	}
}
