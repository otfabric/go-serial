//go:build darwin || linux || freebsd || openbsd || netbsd

// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"testing"
)

func TestOpenUnsupportedBaudRate(t *testing.T) {
	// On POSIX, unsupported baud is rejected before opening the device.
	_, err := Open(&Config{Address: "/dev/tty0", BaudRate: BaudRate(99999)})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("want ErrInvalidConfig, got %v", err)
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("want ConfigError, got %v", err)
	}
	if cfgErr.Field != "BaudRate" {
		t.Fatalf("want ConfigError Field BaudRate, got %q", cfgErr.Field)
	}
	if !IsUnsupportedBaudRate(err) {
		t.Fatal("IsUnsupportedBaudRate should be true")
	}
}
