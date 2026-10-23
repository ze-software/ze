// Design: docs/architecture/testing/interop.md -- final policy judged by a foreign daemon.
// Related: test/interop/scenarios/bgp-policy-final-frr -- immutable source UPDATE and reload inputs.
package bgp

const scenarioPolicyFinalFRR = "bgp-policy-final-frr"

func init() {
	specialCheckers[scenarioPolicyFinalFRR] = checkPolicyFinalFRR
}
