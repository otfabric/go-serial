go-serial Requirements

Purpose

go-serial must provide a small, reliable, cross-platform Go library for opening, configuring, reading from, writing to, and closing serial ports. The library must remain transport-focused and protocol-neutral, while making it easy for higher-layer protocols such as Modbus RTU to supply their own presets.

Product Goals

The library must:
	•	expose a clean and idiomatic Go API
	•	work across supported desktop/server operating systems
	•	support common UART and RS-485 use cases
	•	avoid protocol-specific behavior in the root package
	•	provide predictable configuration defaults
	•	return clear, inspectable errors
	•	avoid hidden logging and other side effects
	•	be thoroughly covered by tests that focus on validation and public API behavior
	•	include concise, professional documentation and examples

Non-Goals

The library must not:
	•	implement Modbus framing, CRC, timing, or master/slave logic
	•	become a generic terminal emulator or feature-heavy serial toolkit
	•	expose protocol-specific assumptions in the root package
	•	log to stdout/stderr from the library itself
	•	require users to understand platform internals to use the common API

API Requirements

Package structure

The repository must keep:
	•	root package serial for generic serial transport
	•	subpackage modbus for Modbus RTU-oriented presets only

The root package must feel protocol-neutral.

Public types

The public API should remain small and explicit.

Required public surface:
	•	type Config
	•	type RS485Config
	•	type Port interface
	•	func Open(*Config) (Port, error)
	•	func New() Port
	•	func DefaultConfig(address string) Config

Required Modbus preset surface:
	•	func modbus.DefaultRTUConfig(address string) serial.Config

Configuration quality

Config must be well documented and validated.

Validation must cover at least:
	•	empty address
	•	unsupported baud rate when the implementation only supports fixed rates
	•	unsupported data bits
	•	unsupported stop bits
	•	unsupported parity values
	•	invalid negative timeout values
	•	invalid negative RS-485 delays

The package should expose typed validation errors or sentinel errors where that improves errors.Is / errors.As behavior.

Type quality

The API should prefer strong types where they materially improve correctness and readability.

Recommended improvements:
	•	replace string parity values with a dedicated Parity type
	•	optionally replace raw stop-bit integers with a dedicated StopBits type if this improves clarity without making the API heavy
	•	keep zero-value behavior explicit and documented

If strong types are introduced, maintain ergonomic defaults and simple examples.

Error handling

Errors must be suitable for inspection and composition.

Requirements:
	•	preserve ErrTimeout as a sentinel timeout error
	•	expose validation-related sentinels such as ErrInvalidConfig when appropriate
	•	wrap underlying OS/syscall failures with %w
	•	keep user-facing error text concise and specific
	•	avoid vague messages such as “unknown error” unless no better signal exists

Where appropriate, the package should implement small helper error types, for example field-validation errors, if this materially improves debuggability.

Logging

The library must not produce log output during normal operation or cleanup.

Requirements:
	•	remove package-level use of log.Printf
	•	never write directly to stdout/stderr from library code
	•	return errors instead of logging whenever possible
	•	if a best-effort cleanup cannot be surfaced directly, prefer silent cleanup over surprising logs from library internals

The example program may log because it is an application, not a library.

Documentation

Documentation must be professional and concise.

Required documentation:
	•	package-level docs for serial
	•	package-level docs for modbus
	•	doc comments for all exported identifiers
	•	README sections for purpose, installation, generic usage, Modbus preset usage, RS-485 notes, timeout behavior, supported platforms, and non-goals
	•	migration note from goburrow/serial to gofabric/go-serial

Examples must show:
	•	generic usage with serial.DefaultConfig
	•	Modbus-oriented usage with modbus.DefaultRTUConfig
	•	explicit timeout handling with errors.Is(err, serial.ErrTimeout)

Implementation Requirements

Configuration validation

Validation should happen before opening the device whenever possible.

A dedicated Validate function or Config.Validate() method is recommended if it improves API clarity.

Default behavior

The generic root package defaults must remain neutral.

Generic defaults should remain effectively equivalent to:
	•	9600 baud
	•	8 data bits
	•	1 stop bit
	•	no parity

Modbus defaults must remain isolated to the modbus subpackage.

POSIX behavior

POSIX code must:
	•	keep supported platforms working as before
	•	avoid unnecessary side effects
	•	restore termios state on close where possible
	•	wrap syscall failures with context
	•	keep RS-485 support in the root package

Windows behavior

Windows code must:
	•	preserve open/read/write/close support
	•	preserve timeout handling
	•	wrap underlying failures with context
	•	align parity/default behavior with the generic root API

Testing Requirements

The test strategy must not depend on physical serial hardware for core coverage.

Required tests:
	•	DefaultConfig behavior
	•	modbus.DefaultRTUConfig behavior
	•	config validation success/failure cases
	•	parity parsing/validation behavior
	•	stop-bit validation behavior
	•	timeout sentinel behavior where unit-testable
	•	error wrapping expectations for validation paths

Recommended tests:
	•	table-driven tests for supported/unsupported config values
	•	tests for zero-value/default normalization behavior
	•	example tests or doctest-style examples

Optional integration tests may be added later for PTY-backed Unix verification, but these should not be required for standard CI unless stable and portable.

Backward Compatibility Requirements

This repository is already a renamed/forked module, so selective cleanup is acceptable. However:
	•	keep the public API intentionally small
	•	avoid gratuitous breaking changes
	•	document intentional behavioral changes in defaults or validation
	•	provide migration notes when changing exported types or defaults

Quality Bar

The refactor is successful when:
	•	the root package reads like an idiomatic generic serial transport library
	•	the public API is easy to understand without reading platform code
	•	configuration errors are caught early and explained clearly
	•	the library does not log internally
	•	tests cover the important user-visible behavior
	•	docs make the layering between serial and modbus obvious
	•	the package feels production-ready for reuse in other Go projects