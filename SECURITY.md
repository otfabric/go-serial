# Security policy

## Supported versions

go-serial is pre-1.0. Security fixes are released on the minor line of the latest release tag; older minor lines are not patched.

| Version | Supported |
|---------|-----------|
| 0.5.x | Yes, from its first release |
| 0.4.x | Yes, until v0.5.0 is released |
| < 0.4 | No; upgrade to the latest release |

## Reporting a vulnerability

If you believe you have found a security vulnerability in **go-serial** ([github.com/otfabric/go-serial](https://github.com/otfabric/go-serial)), please report it responsibly.

- **Do not** open a public GitHub issue for security-sensitive bugs.
- Email **security@otfabric.io**, or open a **private security advisory** on GitHub: go to the [Security](https://github.com/otfabric/go-serial/security) tab → **Advisories** → **Report a vulnerability**.

Include a clear description, steps to reproduce, affected versions, and the impact. We will acknowledge your report and work with you on a fix and disclosure timeline.

## What the library protects

go-serial is a transport: it opens a device, sets the line parameters and moves bytes. It has no authentication, no encryption and no integrity check, and a serial line has none either. Those belong to the protocol above it. There are no security settings to turn on or off.

### Trusted: the `Config`

Every field of `Config` is taken as the choice of the application or of its operator.

- `Address` is passed to the operating system's open call. On POSIX systems it is passed as given, and the opened file must be a character device, or `Open` fails before a terminal setting is read or changed; the library does not tell a serial port from another terminal device. On Windows `Open` takes a device name only, such as `COM3` or `\\.\COM10`, and opens it in the device namespace; any other form fails with `ErrInvalidConfig` before `CreateFile` is called. `Config.UncheckedAddress` passes the address to `CreateFile` unchanged, as before v0.5.0; set it only for an address from a trusted source. An application that takes the address from a source it trusts less than its operator, such as a request or a file written by another party, checks it against its own list of ports before it calls `Open`.
- The RS-485 delays are checked against the width of the kernel's field (`MaxRS485Delay`), not against what a driver supports, and are passed to the driver as given. With `Config.VerifySettings`, `Open` reads them back and fails when the driver changed them.
- `Open` changes the terminal settings of the device and `Close` restores them. With `Config.VerifySettings`, `Open` fails when the device did not take the speed, data bits, stop bits or parity, and restores the settings it found.
- The other fields are validated by `Config.Validate` and, for the baud rate, against the table of the platform in `Open`. A value that does not fit the type the operating system takes is rejected or clamped, as [LIMITATIONS.md](LIMITATIONS.md) describes.

### Not trusted: the bytes on the line

`Read` copies the received bytes into the caller's buffer and looks at none of them: no received byte is decoded, used as a length or an index, or sizes an allocation. A byte received with a parity or framing error is delivered as data ([LIMITATIONS.md](LIMITATIONS.md#not-implemented)), so the protocol above detects corruption, with a checksum for example.

## Resource use

| Resource | Bound |
|---|---|
| Descriptors | POSIX: three per open port, the device and the two ends of the pipe with which `Close` wakes a blocked `Read` or `Write`. Windows: one handle. `Close` releases them, and so does `Open` when it fails |
| Goroutines | The library starts none |
| Memory | No internal buffer. `Read` and `Write` use the caller's slice for the duration of the call |
| Time in `Read` | `Config.Timeout`. Without one: until data arrives, the line goes away, or `Close` |
| Time in `Write` | POSIX: no deadline. Windows: `Config.Timeout` for each `WriteFile` call, when it is set |

## Known limits

- POSIX: `Write` has no deadline. With a device that stops taking output it returns only when `Close` is called from another goroutine.
- POSIX: the device is not opened for exclusive use. Another process with permission on the device node can open the same port. Windows opens the port exclusively.
- The library does not report line errors, and has no flow control and no modem line support ([LIMITATIONS.md](LIMITATIONS.md)).
