// SPDX-License-Identifier: MIT

package serial

import (
	"errors"
	"strings"
	"testing"
)

// TestWindowsDeviceName: device names are opened in the device namespace;
// every other form is refused with a *ConfigError for Address wrapping
// ErrInvalidConfig.
func TestWindowsDeviceName(t *testing.T) {
	accepted := []struct{ address, name string }{
		{"COM1", `\\.\COM1`},
		{"COM1:", `\\.\COM1`},
		{"com9", `\\.\com9`},
		{"COM10", `\\.\COM10`},
		{"COM256", `\\.\COM256`},
		{"CNCA0", `\\.\CNCA0`},
		{`\\.\COM10`, `\\.\COM10`},
		{`\\?\COM10`, `\\.\COM10`},
		{`\??\COM3`, `\\.\COM3`},
		{`//./COM4`, `\\.\COM4`},
		{`//?/COM5`, `\\.\COM5`},
		{`\\?\FTDIBUS#VID_0403+PID_6001+A50285BIA#0000#{86e0d1e0-8089-11d0-9ce4-08003e301f73}`,
			`\\.\FTDIBUS#VID_0403+PID_6001+A50285BIA#0000#{86e0d1e0-8089-11d0-9ce4-08003e301f73}`},
		{`\\.\Global\COM1`, `\\.\Global\COM1`},
		{`\\.\global\COM1`, `\\.\global\COM1`},
		{`\\?\GLOBALROOT\Device\Serial0`, `\\.\GLOBALROOT\Device\Serial0`},
		{`\\.\globalroot\device\VCP0`, `\\.\globalroot\device\VCP0`},
		{"a", `\\.\a`},
		{"C:", `\\.\C`}, // the device C, which is not the drive C:
		{"...", `\\.\...`},
	}
	for _, tc := range accepted {
		got, err := windowsDeviceName(tc.address)
		if err != nil || got != tc.name {
			t.Errorf("windowsDeviceName(%q) = %q, %v; want %q", tc.address, got, err, tc.name)
		}
	}

	refused := []string{
		"", ":", "COM1::", `C:\COM1`, "C:COM1", `\COM1`, `COM1\`, "dir/COM1", "COM1/",
		".", "..", `.\COM1`, `..\COM1`, "a\x00b",
		`\\server\share\file`, `\\server\share`, `//server/share`, `\\?\UNC\server\share`, `\\.\UNC\server\share`,
		`\\.\pipe\name`, `\\?\pipe\name`, `\\.\mailslot\name`,
		`\\.\`, `\\?\`, `\??\`, `\\.\.`, `\\.\..`, `\\.\COM1:`, `\\.\C:`, `\\?\C:\file`, `\\.\COM1\x`, `\\.\COM1/x`,
		`\\.\Global\`, `\\.\Global\UNC\server\share`, `\\.\Global\COM1\x`, `\\.\Local\COM1`,
		`\\?\GLOBALROOT\Device`, `\\?\GLOBALROOT\Device\`, `\\?\GLOBALROOT\Device\Mup\server\share`,
		`\\?\GLOBALROOT\Device\NamedPipe\name`, `\\?\GLOBALROOT\??\COM1`, `\\?\GLOBALROOT\Device\Serial0:`,
		`\\?\USB#VID_1234#{86e0d1e0-8089-11d0-9ce4-08003e301f73}\ref`,
	}
	for _, address := range refused {
		got, err := windowsDeviceName(address)
		var ce *ConfigError
		if err == nil || !errors.Is(err, ErrInvalidConfig) || !errors.As(err, &ce) || ce.Field != "Address" || ce.Value != address {
			t.Errorf("windowsDeviceName(%q) = %q, %v; want a *ConfigError for Address wrapping ErrInvalidConfig", address, got, err)
		}
	}
}

// FuzzWindowsDeviceName: the check returns for every input, refuses with a
// *ConfigError wrapping ErrInvalidConfig, and accepts only a name in the
// device namespace: \\.\ and then one element, Global\ and one element, or
// GLOBALROOT\Device\ and one element, with no slash, colon or NUL.
func FuzzWindowsDeviceName(f *testing.F) {
	for _, s := range []string{"COM1", "COM1:", `\\.\COM10`, `\\?\GLOBALROOT\Device\Serial0`, `\\.\Global\COM1`,
		`\\server\share`, `\\.\pipe\x`, `C:\x`, `//./COM1`, "a\x00"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, address string) {
		name, err := windowsDeviceName(address)
		if err != nil {
			var ce *ConfigError
			if !errors.As(err, &ce) || !errors.Is(err, ErrInvalidConfig) || name != "" {
				t.Fatalf("windowsDeviceName(%q) = %q, %v", address, name, err)
			}
			return
		}
		rest, ok := strings.CutPrefix(name, `\\.\`)
		if !ok || strings.ContainsAny(rest, "/:\x00") {
			t.Fatalf("windowsDeviceName(%q) = %q", address, name)
		}
		parts := strings.Split(rest, `\`)
		switch {
		case len(parts) == 1:
		case len(parts) == 2 && strings.EqualFold(parts[0], "Global"):
		case len(parts) == 3 && strings.EqualFold(parts[0], "GLOBALROOT") && strings.EqualFold(parts[1], "Device"):
		default:
			t.Fatalf("windowsDeviceName(%q) = %q", address, name)
		}
		if last := parts[len(parts)-1]; last == "" || last == "." || last == ".." {
			t.Fatalf("windowsDeviceName(%q) = %q", address, name)
		}
	})
}
