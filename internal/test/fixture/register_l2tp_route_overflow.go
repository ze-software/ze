// Design: docs/guide/l2tp.md -- daemon framed-route refusal and diagnostics.
// Related: l2tp_route_overflow_fixture.go -- genuine PPP observer.
// Related: l2tp_route_overflow_radius.go -- bounded RADIUS wire profiles.
package fixture

func init() {
	Register("l2tp/radius-framed-route-overflow", routeOverflowDriver, StopsDaemon)
	Register("l2tp/radius-framed-route-overflow/radius", routeOverflowRadius)
}
