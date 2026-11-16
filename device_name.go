// SPDX-License-Identifier: MIT

package serial

import "strings"

// windowsDeviceName returns the name under which the Windows backend opens
// address with CreateFile: a device name in the device namespace, \\.\NAME.
//
// It accepts NAME and NAME: (the colon is dropped) and, after one of the
// prefixes \\.\, \\?\, \??\, //./ and //?/, also Global\NAME and
// GLOBALROOT\Device\NAME, with Global, GLOBALROOT and Device in any case.
// NAME is not empty, not "." or "..", and holds no backslash, slash, colon
// or NUL. Anything else is refused with a *ConfigError wrapping
// ErrInvalidConfig.
//
// It is the address check of the Windows Open; the file has no build
// constraint so that its tests run on every platform.
func windowsDeviceName(address string) (string, error) {
	rest, prefixed := cutDevicePrefix(address)
	if !prefixed {
		name := strings.TrimSuffix(address, ":")
		if !validDeviceName(name) {
			return "", refusedAddress(address)
		}
		return `\\.\` + name, nil
	}
	parts := strings.Split(rest, `\`)
	switch {
	case len(parts) == 1:
	case len(parts) == 2 && strings.EqualFold(parts[0], "Global"):
	case len(parts) == 3 && strings.EqualFold(parts[0], "GLOBALROOT") && strings.EqualFold(parts[1], "Device"):
	default:
		return "", refusedAddress(address)
	}
	if !validDeviceName(parts[len(parts)-1]) {
		return "", refusedAddress(address)
	}
	return `\\.\` + rest, nil
}

// cutDevicePrefix returns address without its device namespace prefix, and
// whether it had one.
func cutDevicePrefix(address string) (string, bool) {
	for _, prefix := range []string{`\\.\`, `\\?\`, `\??\`, `//./`, `//?/`} {
		if rest, ok := strings.CutPrefix(address, prefix); ok {
			return rest, true
		}
	}
	return address, false
}

// validDeviceName reports whether name is one element of a device name.
func validDeviceName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "\\/:\x00")
}

func refusedAddress(address string) error {
	return &ConfigError{Field: "Address", Value: address, Reason: "not a device name (see Config.UncheckedAddress)", Err: ErrInvalidConfig}
}
