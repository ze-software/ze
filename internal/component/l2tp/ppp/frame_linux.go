// Design: docs/research/l2tpv2-implementation-guide.md -- /dev/ppp fd wrapping for blocking PPP I/O

//go:build linux

package ppp

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// NewFDFile wraps a raw file descriptor (chan fd or unit fd from
// /dev/ppp) as an io.ReadWriteCloser that performs blocking I/O.
//
// /dev/ppp is a character device whose poll() implementation may not
// reliably wake Go's epoll-based runtime poller. Using os.NewFile
// with a non-blocking fd can cause reads to hang indefinitely when
// data is available but epoll never signals readiness. Instead, we
// keep the fd in blocking mode and dedicate an OS thread per read
// (via the goroutine calling Read, which Go pins to a thread for
// blocking syscalls).
//
// Caller MUST NOT close the original fd after this call.
func NewFDFile(fd int, name string) io.ReadWriteCloser {
	// Ensure blocking mode: the fd may have been set non-blocking by
	// Go's os.OpenFile or inherited from a dup'd file description.
	unix.SetNonblock(fd, false) //nolint:errcheck // best-effort
	return &blockingFD{fd: fd, name: name}
}

type blockingFD struct {
	fd   int
	name string
}

func (f *blockingFD) Read(b []byte) (int, error) {
	n, err := unix.Read(f.fd, b)
	if n == 0 && err == nil {
		return 0, io.EOF
	}
	if err != nil {
		return 0, &os.PathError{Op: "read", Path: f.name, Err: err}
	}
	return n, nil
}

func (f *blockingFD) Write(b []byte) (int, error) {
	n, err := unix.Write(f.fd, b)
	if err != nil {
		return n, &os.PathError{Op: "write", Path: f.name, Err: err}
	}
	return n, nil
}

func (f *blockingFD) Close() error {
	return unix.Close(f.fd)
}
