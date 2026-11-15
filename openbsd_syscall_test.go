// SPDX-License-Identifier: MIT

package serial

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// TestOpenBSDSystemCallsByNumber checks the source files built for OpenBSD.
// There the syscall package carries out syscall.Syscall for SYS_IOCTL only
// and answers every other number, and the other calls by number, with
// ENOSYS: such a call compiles and can never succeed. The files built for
// openbsd therefore use the functions of the syscall package (Select, Fstat,
// Pipe, ...) and syscall.Syscall for ioctl alone. No OpenBSD host runs the
// tests, so this is checked on the source, on every platform.
func TestOpenBSDSystemCallsByNumber(t *testing.T) {
	if _, err := os.Stat("serial_openbsd.go"); err != nil {
		t.Skipf("the source files are not in the working directory: %v", err)
	}
	ctx := build.Default
	ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = "openbsd", "amd64", false
	pkg, err := ctx.ImportDir(".", 0)
	if err != nil {
		t.Fatal(err)
	}
	byNumber := map[string]bool{"Syscall": true, "Syscall6": true, "Syscall9": true, "RawSyscall": true, "RawSyscall6": true}
	fset := token.NewFileSet()
	ioctls := 0
	files := append(append([]string{}, pkg.GoFiles...), pkg.TestGoFiles...)
	for _, name := range files {
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := syscallSelector(call.Fun)
			if !ok || !byNumber[fn] || len(call.Args) == 0 {
				return true
			}
			num := call.Args[0]
			if conv, ok := num.(*ast.CallExpr); ok && len(conv.Args) == 1 { // uintptr(syscall.SYS_IOCTL)
				num = conv.Args[0]
			}
			if trap, _ := syscallSelector(num); fn == "Syscall" && trap == "SYS_IOCTL" {
				ioctls++
				return true
			}
			t.Errorf("%s: syscall.%s is built for openbsd with a number other than SYS_IOCTL", fset.Position(call.Pos()), fn)
			return true
		})
	}
	// tcsetattr and tcgetattr: the check looked at the OpenBSD backend.
	if ioctls < 2 {
		t.Errorf("found %d ioctl calls in the files built for openbsd (%v), want at least 2", ioctls, files)
	}
}

// syscallSelector returns name for the expression syscall.name.
func syscallSelector(e ast.Expr) (string, bool) {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "syscall" {
		return "", false
	}
	return sel.Sel.Name, true
}
