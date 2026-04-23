// Design: plan/spec-static-routes.md -- backend abstraction

package static

type routeBackend interface {
	applyRoute(r staticRoute) error
	removeRoute(r staticRoute) error
	listRoutes() ([]installedStaticRoute, error)
	close() error
}

type installedStaticRoute struct {
	prefix  string
	nextHop string
}
