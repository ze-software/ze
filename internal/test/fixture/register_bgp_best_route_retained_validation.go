// Design: docs/functional-tests.md -- real GR/RTR/ordinary-RIB socket lifecycle.
// Detail: bgp_best_route_retained_validation.go -- the joined lifecycle oracle.
package fixture

func init() {
	Register("plugin/bgp-best-route-retained-validation", bgpRetainedDriver, StopsDaemon)
}
