//go:build !linux

// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- non-Linux stub for /proc/<pid> inspector
// Related: process.go -- ProcessInspect type and status parser
// Related: process_linux.go -- Linux /proc/<pid> reader

package procfs

// ReadProcessInspect is unsupported outside Linux.
func ReadProcessInspect(_ int) (ProcessInspect, error) {
	return ProcessInspect{}, ErrUnsupported
}
