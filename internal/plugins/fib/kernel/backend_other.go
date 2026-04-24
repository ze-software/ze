// Design: docs/architecture/core-design.md -- FIB noop backend
// Overview: fibkernel.go -- FIB kernel plugin
// Related: backend.go -- backend abstraction
// Related: backend_linux.go -- Linux netlink backend
//
// Noop backend for non-Linux platforms. Logs route operations but does not
// program the OS routing table. Darwin route socket support is defined in
// the spec but requires platform-specific route table ID conventions.

//go:build !linux

package fibkernel

import "fmt"

var errUnsupportedPlatform = fmt.Errorf("fib-kernel: not supported on this platform")

// newBackend returns an error backend on non-Linux platforms.
func newBackend() routeBackend {
	return &unsupportedBackend{}
}

// unsupportedBackend rejects all route operations on non-Linux platforms.
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
