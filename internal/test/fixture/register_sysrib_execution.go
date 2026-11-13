// Design: docs/guide/route-policy.md -- both system RIB placements reach the daemon authority.
// Related: sysrib_execution_fixture.go -- the real-daemon and kernel observer.

package fixture

func init() {
	Register("plugin/sysrib-execution-parity", sysribExecutionDriver, StopsDaemon)
	Register("plugin/sysrib-execution-kernel-empty", sysribExecutionKernelEmpty)
}
