//go:build linux

// Design: docs/guide/ddos-mitigation.md -- what each end of the FlowSpec plugin does to a live announcement
// Related: ddos_flowspec_exit_linux.go -- the two drivers' bodies
// Related: register_ddos_flowspec_orphan_cleared.go -- registers the clearing barrier on every host

package fixture

// The drivers of the two daemon-level FlowSpec exit tests:
// test/reload/ddos-flowspec-parent-removed-withdraws.ci and
// test/plugin/ddos-flowspec-orphan-doctor.ci.
func init() {
	Register("plugin/ddos-flowspec-parent-removed-driver", ddosFlowspecParentRemoved, StopsDaemon)
	Register("plugin/ddos-flowspec-orphan-flood", ddosFlowspecOrphanFlood)
}
