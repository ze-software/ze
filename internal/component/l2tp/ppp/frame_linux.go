// Design: docs/research/l2tpv2-implementation-guide.md -- /dev/ppp fd wrapping for blocking PPP I/O

//go:build linux

package ppp

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// NewFDFile wraps a channel or unit descriptor for blocking PPP I/O.
// /dev/ppp must not use the runtime's epoll path: some kernels do not
// wake it reliably. A blocking descriptor passed to os.NewFile is not
// registered with that poller. os.File still serializes Close with new
// operations and retains the descriptor until in-flight syscalls return,
// preventing reads or repeated closes from using a recycled descriptor.
//
// Caller MUST NOT close the original fd after this call.
func NewFDFile(fd int, name string) io.ReadWriteCloser {
	// Ensure blocking mode: the fd may have been set non-blocking by
	// Go's os.OpenFile or inherited from a dup'd file description.
	unix.SetNonblock(fd, false) //nolint:errcheck // best-effort
	return os.NewFile(uintptr(fd), name)
}
