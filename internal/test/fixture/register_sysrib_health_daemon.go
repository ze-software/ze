// Design: docs/architecture/doctor-and-health-checks.md -- real runtime health.
// Related: sysrib_health_daemon_fixture.go -- useful FIB, doctor and telemetry observations.

package fixture

func init() {
	Register("plugin/sysrib-execution-health", sysribHealthDriver, StopsDaemon)
}
