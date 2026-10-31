//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"os"
	"testing"
)

// skipNoPTY skips a test whose pseudo-terminal cannot be had, or fails it
// when SERIAL_REQUIRE_PTY=1: the CI jobs on platforms with a PTY helper set
// the variable, so a broken setup cannot pass as a run of skips.
func skipNoPTY(t *testing.T, err error) {
	t.Helper()
	if os.Getenv("SERIAL_REQUIRE_PTY") == "1" {
		t.Fatalf("SERIAL_REQUIRE_PTY=1 and no PTY: %v", err)
	}
	t.Skipf("skip when PTY cannot be created: %v", err)
}
