// Design: docs/research/l2tpv2-implementation-guide.md -- non-Linux stub for PPP fd wrapping

//go:build !linux

package ppp

import (
	"errors"
	"io"
)

// errNotLinux is returned by stubs when invoked on non-Linux builds.
var errNotLinux = errors.New("ppp: /dev/ppp is Linux-only; transport must gate by GOOS")

// NewFDFile is a stub on non-Linux platforms. /dev/ppp is Linux-only;
// callers that reach this in a real build path indicate a bug in the
// transport's platform gating (PPP MUST run only on Linux). The stub
// returns a closed errReader so any read/write on it fails fast.
func NewFDFile(fd int, name string) io.ReadWriteCloser {
	return errFD{}
}

type errFD struct{}

func (errFD) Read([]byte) (int, error)  { return 0, errNotLinux }
func (errFD) Write([]byte) (int, error) { return 0, errNotLinux }
func (errFD) Close() error              { return nil }
