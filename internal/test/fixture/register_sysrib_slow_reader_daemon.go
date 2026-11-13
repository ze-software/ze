// Design: docs/architecture/rib/unified-locrib.md -- actual blocked consumer recovery.
// Related: sysrib_slow_reader_daemon_fixture.go -- complete current kernel table.

package fixture

func init() {
	Register("plugin/sysrib-slow-reader-reset", sysribSlowDriver, StopsDaemon)
	Register("plugin/sysrib-slow-reader-kernel-empty", sysribSlowKernelEmpty)
}
