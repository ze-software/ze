// Design: docs/architecture/rib/unified-locrib.md -- live owner distance and defaults.
// Related: sysrib_distance_daemon_fixture.go -- actual tentative output and rollback.
package fixture

func init() {
	Register("plugin/sysrib-distance-reload", sysribDistanceDriver, StopsDaemon)
	Register("plugin/sysrib-distance-rollback", sysribDistanceRollbackDriver)
	Register("plugin/sysrib-distance-kernel-empty", sysribDistanceKernelEmpty)
}
