// Design: docs/architecture/rib/unified-locrib.md -- all admitted explicit groups reach forwarding.
// Related: sysrib_nonwinner_ecmp_daemon_fixture.go -- paired daemon/kernel regression.
package fixture

func init() {
	Register("plugin/sysrib-nonwinner-explicit-ecmp", sysribNonwinnerECMPDriver, StopsDaemon)
	Register("plugin/sysrib-nonwinner-explicit-ecmp-kernel-empty", sysribNonwinnerECMPKernelEmpty)
}
