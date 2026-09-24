// Design: docs/architecture/diagnostics/crash-capture.md -- the leave that drains the pipe
// Related: stderr.go -- the relay this drains before the image is replaced

//go:build unix

package crashlog

import "syscall"

// Exec drains the stderr pipe and then replaces this process image with the
// program at path. It returns only when the execve was refused, because a
// successful one never returns.
//
// Every execve in Ze MUST go through here, and a raw syscall.Exec outside this
// package is refused by the crashlogExec rule in .golangci/ruleguard/modern.go.
// Init points os.Stderr at a pipe that two goroutines drain, and an execve
// destroys both, so every line still in the pipe or the relay queue is lost.
// Flush closes the pipe and waits for the relay to write what it holds, then
// puts os.Stderr back on the real stderr.
//
// Descriptor 2 itself is never redirected, so the new program inherits the real
// stderr either way. Until 2026-09-24 Init dup2'd the pipe onto descriptor 2,
// and an unflushed execve left the new image writing into a pipe nobody read:
// measured in the QEMU guest on 2026-09-11, a daemon launched that way served
// /metrics and emitted not one log line. It is safe when Init was never called:
// Flush then finds no pipe and returns.
func Exec(path string, argv, environ []string) error {
	Flush()
	return syscall.Exec(path, argv, environ) //nolint:gosec // G204: the caller chooses the program; this function only drains the stderr relay before it runs
}
