// Design: docs/architecture/forked-route-install.md -- plugin engine-operation capability gating through the bus send filter.
// Related: bus_send_op_cap_probe.go -- busSendOpCapProbeDriver registered below.

package fixture

func init() {
	// test/draft/plugin/bus-send-operation-capability.ci (S37): Run 1 refuses
	// undeclared route-install, route-remove, inject-wire-route, and emit-event;
	// Run 2 declares system-rib:route-install, bgp:raw, and acme:alert and
	// succeeds for all four.
	Register("plugin/bus-send-op-cap-probe", busSendOpCapProbeDriver)
}
