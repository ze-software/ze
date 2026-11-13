// Design: docs/architecture/doctor-and-health-checks.md -- real owner loss/recovery.
// Related: sysrib_restart_daemon_fixture.go -- actual process generation and kernel survivors.
package fixture

func init() {
	Register("plugin/sysrib-feed-restart", sysribRestartDriver, StopsDaemon)
	Register("plugin/sysrib-restart-kernel-empty", sysribRestartKernelEmpty)
	Register("plugin/sysrib-restart-link-cleanup", sysribRestartLinkCleanup)
}
