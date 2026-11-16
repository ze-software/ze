// Design: docs/guide/ddos-mitigation.md -- what each end of the FlowSpec plugin does to a live announcement
// Related: ddos_flowspec_orphan_cleared.go -- the clearing barrier's body
// Related: register_ddos_flowspec_exit_linux.go -- registers the Linux-only flood drivers

package fixture

// These foreground drivers own their commands and return only after their
// assertions hold. None dispatches daemon shutdown, so none declares StopsDaemon.
func init() {
	Register("plugin/ddos-flowspec-orphan-cleared", ddosFlowspecOrphanCleared)
	Register("plugin/ddos-flowspec-restore-cause", ddosFlowspecRestoreCause)
	Register("plugin/ddos-flowspec-run-restart", ddosFlowspecRunRestart)
	Register("plugin/ddos-flowspec-clear-orphan", ddosFlowspecClearOrphan)
	Register("plugin/doctor-fatal-stop", doctorPluginFatalStop)
}
