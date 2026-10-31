// Design: docs/architecture/testing/interop.md -- RFC 8212 default reject judged by FRR.
// Related: test/interop/scenarios/bgp-rfc8212-default-reject-frr -- the policy-less and reload configs.
package bgp

const scenarioRFC8212DefaultRejectFRR = "bgp-rfc8212-default-reject-frr"

func init() {
	specialCheckers[scenarioRFC8212DefaultRejectFRR] = checkRFC8212DefaultRejectFRR
}
