REM Regenerate Windows CGO types from types_windows.go and syscall stubs.
REM Run from repo root when types_windows.go or syscall_windows.go change.
go tool cgo -godefs types_windows.go | gofmt > ztypes_windows.go
go generate syscall_windows.go
