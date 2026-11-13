// Design: docs/architecture/route-selection.md -- actual BGP caller parity.
// Related: sysrib_bgp_metrics_daemon_fixture.go -- retained-input reselection.

package fixture

func init() {
	Register("plugin/sysrib-bgp-metric-callers", sysribBGPMetricDriver, StopsDaemon)
	Register("plugin/sysrib-bgp-metric-kernel-empty", sysribBGPMetricKernelEmpty)
}
