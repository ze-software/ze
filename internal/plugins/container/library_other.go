//go:build !linux

// Design: docs/architecture/container/runtime.md -- the runtime library behind the supervisor
// Related: library_linux.go -- the library on Linux

package container

import "log/slog"

// newRuntimeLibrary answers no library off Linux: a container is made of
// Linux namespaces and control groups, so the supervisor refuses every start
// with that reason (noRuntime).
func newRuntimeLibrary(*slog.Logger) runtimeLibrary {
	return nil
}
