// Design: docs/guide/ddos-mitigation.md -- what each end of the FlowSpec plugin does to a live announcement
// Related: ddos_flowspec_orphan_cleared.go -- the clearing barrier's body
// Related: register_ddos_flowspec_exit_linux.go -- registers the Linux-only flood drivers

package fixture

// The clearing barrier of the FlowSpec orphan test,
// test/plugin/ddos-flowspec-orphan-doctor.ci, its only user. It only runs
// `ze cli` commands, so it is registered on every host.
func init() {
	Register("plugin/ddos-flowspec-orphan-cleared", ddosFlowspecOrphanCleared)
}
