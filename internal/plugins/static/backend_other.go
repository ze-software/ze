// Design: docs/architecture/static-routes.md -- rejecting backend for non-Linux

//go:build !linux

package static

import (
	"errors"
)

var errStaticRoutesNotSupportedOnThis = errors.New("static routes: not supported on this platform (Linux required)")

var errUnsupportedPlatform = errStaticRoutesNotSupportedOnThis

type unsupportedStaticBackend struct{}

func newStaticBackendForDataPlane(dataPlane string) (routeBackend, error) {
	switch dataPlane {
	case dataPlaneKernel:
		return &unsupportedStaticBackend{}, nil
	case dataPlaneVPP:
		return nil, staticVPPChannelError(errUnsupportedPlatform)
	default:
		return nil, unrecognizedDataPlaneError(dataPlane)
	}
}

func (n *unsupportedStaticBackend) applyRoute(_ staticRoute) error  { return errUnsupportedPlatform }
func (n *unsupportedStaticBackend) removeRoute(_ staticRoute) error { return errUnsupportedPlatform }
func (n *unsupportedStaticBackend) listRoutes() ([]installedStaticRoute, error) {
	return nil, errUnsupportedPlatform
}
func (n *unsupportedStaticBackend) close() error { return nil }
