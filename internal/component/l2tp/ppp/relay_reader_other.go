// Design: docs/architecture/l2tp/followup-l2tp-call.md -- realm relay handoff (LAC role)
// Related: relay_reader_linux.go -- the Linux reader

//go:build !linux

package ppp

import "io"

// newInterruptibleFDFile refuses on non-Linux builds: /dev/ppp is Linux-only,
// so a realm relay session cannot start here.
func newInterruptibleFDFile(int, string) (io.ReadWriteCloser, error) {
	return nil, errNotLinux
}
