// Design: docs/functional-tests.md -- RS whole-lifecycle socket fixture.
package fixture

func init() {
	Register("plugin/bgp-best-route-rs-lifetime", bgpBestRouteRSLifetimeDriver, StopsDaemon)
}
