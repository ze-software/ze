// Design: plan/spec-static-routes.md -- noop backend for non-Linux

//go:build !linux

package static

type noopStaticBackend struct{}

func newStaticBackend() routeBackend { return &noopStaticBackend{} }

func (n *noopStaticBackend) applyRoute(_ staticRoute) error              { return nil }
func (n *noopStaticBackend) removeRoute(_ staticRoute) error             { return nil }
func (n *noopStaticBackend) listRoutes() ([]installedStaticRoute, error) { return nil, nil }
func (n *noopStaticBackend) close() error                                { return nil }
