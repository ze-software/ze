// Design: docs/architecture/testing/interop.md -- a route server acts on, then removes, its large control communities.
package bgp

func init() {
	specialCheckers[routeServerControlScenario] = checkRouteServerControlCommunity
}
