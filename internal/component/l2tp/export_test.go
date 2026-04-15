package l2tp

// SetProbeKernelModulesForTest replaces the kernel module probe for the
// duration of a test. Returns a restore function the caller MUST defer
// (or pass to t.Cleanup) so subsequent tests see the production probe.
//
// NOT safe for concurrent use. Tests calling this MUST NOT call
// t.Parallel(); the helper mutates a package-level var without
// synchronization. The ze test suite runs sequentially under
// `-race -count=1`, which makes this acceptable.
func SetProbeKernelModulesForTest(fn func() error) func() {
	old := probeKernelModulesFn
	probeKernelModulesFn = fn
	return func() { probeKernelModulesFn = old }
}
