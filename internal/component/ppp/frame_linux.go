// Design: docs/research/l2tpv2-implementation-guide.md -- /dev/ppp fd wrapping for blocking PPP I/O

//go:build linux

package ppp

import (
	"io"
	"os"
)

// NewFDFile wraps a raw file descriptor (chan fd or unit fd from
// /dev/ppp) as an io.ReadWriteCloser using Go's runtime poller.
//
// Caller MUST NOT close the original fd after this call -- ownership
// transfers to the returned file. Closing the returned file releases
// the fd.
//
// Reads and writes are blocking from the goroutine's perspective but
// non-blocking at the kernel level (Go's runtime parks the goroutine
// via netpoll instead of blocking the OS thread).
//
// name appears in error messages and Go runtime logs; pass something
// descriptive like "ppp1.chan" or "ppp1.unit" to ease debugging.
func NewFDFile(fd int, name string) io.ReadWriteCloser {
	return os.NewFile(uintptr(fd), name)
}
