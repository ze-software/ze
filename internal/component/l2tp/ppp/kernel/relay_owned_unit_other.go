// Design: docs/research/l2tpv2-ze-integration.md -- same-file unit stream handoff

//go:build !linux

package kernel

import (
	"io"
	"os"
)

func newInterruptibleCreatedUnit(*CreatedUnit, *os.File) (io.ReadWriteCloser, error) {
	return nil, errNotLinux
}

func newInterruptibleSessionFile(*os.File, string, func() error) (io.ReadWriteCloser, error) {
	return nil, errNotLinux
}
