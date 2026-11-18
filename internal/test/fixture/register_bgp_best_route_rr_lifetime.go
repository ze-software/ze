// Design: docs/functional-tests.md -- fixture registration.
package fixture

func init() {
	Register("plugin/bgp-best-route-rr-lifetime", bgpBestRouteRRLifetimeDriver, StopsDaemon)
}
