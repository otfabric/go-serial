//go:build ignore
// +build ignore

// SPDX-License-Identifier: MIT

package serial

// Windows api calls

//go:generate go run $GOROOT/src/syscall/mksyscall_windows.go -output zsyscall_windows.go $GOFILE

//sys GetCommState(handle syscall.Handle, dcb *c_DCB) (err error)
//sys SetCommState(handle syscall.Handle, dcb *c_DCB) (err error)
//sys GetCommTimeouts(handle syscall.Handle, timeouts *c_COMMTIMEOUTS) (err error)
//sys SetCommTimeouts(handle syscall.Handle, timeouts *c_COMMTIMEOUTS) (err error)
//sys escapeCommFunction(handle syscall.Handle, fn uint32) (err error) = EscapeCommFunction
//sys getCommModemStatus(handle syscall.Handle, status *uint32) (err error) = GetCommModemStatus
