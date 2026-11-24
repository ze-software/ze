// Design: docs/architecture/api/architecture.md -- plugin command and operation authorization through the bus send filter.
// Related: bus_send_undeclared_probe.go -- busSendUndeclaredProbeDriver registered below.

package fixture

func init() {
	// test/draft/plugin/bus-send-undeclared-refused.ci (S20): declares only
	// cli:show bgp peer; show bgp peer list succeeds while clear bgp rib in *,
	// route-install, and emit-event bgp:state are refused at the bus.
	Register("plugin/bus-send-undeclared-probe", busSendUndeclaredProbeDriver)
}
