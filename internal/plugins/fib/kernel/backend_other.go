// Design: docs/architecture/core-design.md -- FIB noop backend
// Overview: fibkernel.go -- FIB kernel plugin
// Related: backend.go -- backend abstraction
// Related: backend_linux.go -- Linux netlink backend
//
// Rejecting backend for non-Linux platforms. Route operations return errors
// so callers know the OS routing table is not being programmed.

//go:build !linux

package fibkernel

import (
	"errors"
	"runtime"
)

var errUnsupportedPlatform = errors.New("fib-kernel: unsupported platform " + runtime.GOOS)

func newBackend() routeBackend {
	return &unsupportedBackend{}
}

type unsupportedBackend struct{}

func (u *unsupportedBackend) addRoute(_, _ string) error {
	return errUnsupportedPlatform
}

func (u *unsupportedBackend) delRoute(_ string) error {
	return errUnsupportedPlatform
}

func (u *unsupportedBackend) replaceRoute(_, _ string) error {
	return errUnsupportedPlatform
}

func (u *unsupportedBackend) listZeRoutes() ([]installedRoute, error) {
	return nil, errUnsupportedPlatform
}

func (u *unsupportedBackend) close() error { return nil }
