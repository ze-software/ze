// Design: docs/architecture/api/architecture.md -- plugin command authorization through the bus send filter.
// Related: bus_send_undeclared_probe.go -- busSendUndeclaredProbe, the observer registered below.

package fixture

func init() {
	// test/draft/plugin/bus-send-undeclared-refused.ci (promoted to
	// test/plugin/bus-send-undeclared-refused.ci): an external plugin dispatches
	// show bgp peer list (allowed) and clear bgp rib in * (undeclared, must be
	// refused at the bus instead of waived through Store.authorize).
	Register("plugin/bus-send-undeclared-probe", observeDriver(busSendUndeclaredPlugin, busSendUndeclaredProbe), StopsDaemon)
}
