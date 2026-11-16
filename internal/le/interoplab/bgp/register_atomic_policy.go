// Design: docs/architecture/testing/interop.md -- marked raw policy output at FRR.
// Related: helper_atomic_policy.go -- public SDK policy and source release.
package bgp

const (
	atomicPolicyScenario = "bgp-atomic-policy-admission-frr"
	atomicPolicyRelease  = "request interop atomic-release"
	atomicPolicyRetry    = "INTEROP_SCENARIO=" + atomicPolicyScenario + " ./le test integration interop"
)

func init() {
	registerProcessHelper(atomicPolicyScenario, runAtomicPolicy)
	specialCheckers[atomicPolicyScenario] = checkAtomicPolicyFRR
}
