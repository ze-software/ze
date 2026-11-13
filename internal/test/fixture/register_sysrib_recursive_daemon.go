// Design: docs/architecture/rib/unified-locrib.md -- common resolver through both placements.
// Related: sysrib_recursive_daemon_fixture.go -- controlled cover loss and recovery.
package fixture

func init() {
	Register("plugin/sysrib-recursive-nexthop", sysribRecursiveDriver, StopsDaemon)
	Register("plugin/sysrib-recursive-kernel-empty", sysribRecursiveKernelEmpty)
	Register("plugin/sysrib-recursive-link-cleanup", sysribRecursiveLinkCleanup)
}
