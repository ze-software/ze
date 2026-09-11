// Design: docs/architecture/diagnostics/crash-capture.md -- the leave that drains the pipe
// Related: stderr.go -- the redirect this undoes before the image is replaced

//go:build unix

package crashlog

import "syscall"

// Exec drains the stderr pipe and then replaces this process image with the
// program at path. It returns only when the execve was refused, because a
// successful one never returns.
//
// Every execve in Ze MUST go through here, and a raw syscall.Exec outside this
// package is refused by the crashlogExec rule in .golangci/ruleguard/modern.go.
// Init dup2s a PIPE onto descriptor 2 and drains it from a goroutine. An execve
// destroys that goroutine and the descriptor survives into the new image, so
// the replacement program writes its stderr into a pipe nobody reads: every
// line vanishes, and once 64 KiB have accumulated in the pipe buffer every
// further write BLOCKS FOREVER. Measured in the QEMU guest on 2026-09-11: a
// daemon launched past an unflushed exec served /metrics and counted 12 read
// errors while emitting not one log line.
//
// Flush puts the saved descriptor back on fd 2 first, so the new program
// inherits the real stderr. It is safe when Init was never called: Flush then
// finds no pipe and returns.
func Exec(path string, argv, environ []string) error {
	Flush()
	return syscall.Exec(path, argv, environ) //nolint:gosec // G204: the caller chooses the program; this function only restores fd 2 before it runs
}
