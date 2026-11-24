// Design: docs/architecture/forked-route-install.md -- plugin route-install and route-remove operation capability gating through the bus send filter.
// Related: bus_send_op_cap_probe.go -- busSendOpCapProbe, the observer registered below.

package fixture

func init() {
	// test/draft/plugin/bus-send-operation-capability.ci (promoted to
	// test/plugin/bus-send-operation-capability.ci): an external plugin invokes
	// route-install and route-remove without declaring system-rib:route-install;
	// both must be refused at the bus instead of reaching locrib.Default().
	Register("plugin/bus-send-op-cap-probe", observeDriver(busSendOpCapPlugin, busSendOpCapProbe), StopsDaemon)
}
