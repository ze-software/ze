// Design: docs/architecture/route-selection.md -- SRv6 Service SID resolvability before best-path selection.
// Related: srv6_bestpath_fixture.go -- compiled functional test drivers for RFC 9252 Section 5/6 resolvability.
// RFC: rfc/short/rfc9252.md -- Sections 5 and 6.
package fixture

func init() {
	Register("plugin/srv6-bestpath-unresolvable", srv6BestPathUnresolvableDriver, StopsDaemon)
	Register("plugin/srv6-bestpath-resolvable", srv6BestPathResolvableDriver, StopsDaemon)
	Register("plugin/srv6-bestpath-static-wins", srv6BestPathStaticWinsDriver, StopsDaemon)
	Register("plugin/srv6-bestpath-static-wins-empty", srv6BestPathStaticWinsEmptyDriver)
	Register("plugin/srv6-bestpath-bypass-config", srv6BestPathBypassConfigDriver, StopsDaemon)
}
