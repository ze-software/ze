// Design: docs/architecture/static-routes.md -- rejecting backend for non-Linux

//go:build !linux

package static

import (
	"errors"
)

var errStaticRoutesNotSupportedOnThis = errors.New("static routes: not supported on this platform (Linux required)")

var errUnsupportedPlatform = errStaticRoutesNotSupportedOnThis

type unsupportedStaticBackend struct{}

func newStaticBackend() routeBackend { return &unsupportedStaticBackend{} }

func (n *unsupportedStaticBackend) applyRoute(_ staticRoute) error  { return errUnsupportedPlatform }
func (n *unsupportedStaticBackend) removeRoute(_ staticRoute) error { return errUnsupportedPlatform }
func (n *unsupportedStaticBackend) listRoutes() ([]installedStaticRoute, error) {
	return nil, errUnsupportedPlatform
}
func (n *unsupportedStaticBackend) close() error { return nil }
