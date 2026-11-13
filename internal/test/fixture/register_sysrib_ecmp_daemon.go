// Design: docs/architecture/rib/unified-locrib.md -- complete FIB targets across placements.
// Related: sysrib_ecmp_daemon_fixture.go -- live, promotion, permission and fresh-subscriber replay.
package fixture

func init() {
	Register("plugin/sysrib-ecmp-replay", sysribECMPDriver, StopsDaemon)
	Register("plugin/sysrib-ecmp-kernel-empty", sysribECMPKernelEmpty)
	Register("plugin/sysrib-ecmp-links-cleanup", sysribECMPLinksCleanup)
}
