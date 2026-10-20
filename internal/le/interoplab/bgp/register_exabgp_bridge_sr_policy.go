// Design: docs/architecture/testing/interop.md -- an SR Policy an ExaBGP script announces through the bridge, judged by GoBGP.
// Related: test/interop/scenarios/bgp-exabgp-bridge-sr-policy-gobgp/ze.conf -- the script and the bridge config.
package bgp

import "time"

// scenarioExaBGPBridgeSRPolicyGoBGP is AC-28 of the ExaBGP 6 API spec judged
// by another implementation. An ExaBGP script writes `announce ipv4 sr-policy
// ... preference 300 segment-list ...`; the bridge translates it to `update
// text ... mp ipv4/sr-policy add ...`, whose section is parsed by the
// registered sr-policy config-route parser. GoBGP must hold the policy with
// its NLRI, its Preference sub-TLV and both segment lists' Type A labels.
//
// Before af18a709d8 the update text encoder refused `preference`, so the
// policy never left ze and the first wait below times out.
const (
	scenarioExaBGPBridgeSRPolicyGoBGP = "bgp-exabgp-bridge-sr-policy-gobgp"

	// The NLRI as GoBGP's SRPolicyNLRI.String prints it through gobgp-paths
	// (srPolicyOriginatedBSIDSRv6 names the same NLRI in bgp-sr-policy-gobgp).
	exabgpBridgeSRPolicyNLRI = "Distinguisher: 0, Color 300, Endpoint: 10.0.0.3"
	// The tail of GoBGP v3.31.0's TunnelEncapSubTLVSRPreference.String,
	// "{Flags: 0x%02x, Preference: %d}" (pkg/packet/bgp/sr_policy.go).
	exabgpBridgeSRPolicyPreference = "Preference: 300}"
	exabgpBridgeSRPolicyLabelFirst = "Label: 16001 TC: 0"
	exabgpBridgeSRPolicyLabelOther = "Label: 16002 TC: 0"
)

func init() {
	scenarioOperations[scenarioExaBGPBridgeSRPolicyGoBGP] = []operation{
		{kind: opGoBGPSession, argument: zeLabAddress},
		{kind: opWaitContains, peer: peerGoBGP, command: []string{cmdGoBGPPaths, gobgpAFIIPv4, gobgpSAFISRPolicy},
			contains: []string{exabgpBridgeSRPolicyNLRI, exabgpBridgeSRPolicyPreference, exabgpBridgeSRPolicyLabelFirst, exabgpBridgeSRPolicyLabelOther},
			timeout:  60 * time.Second},
		{kind: opGoBGPSession, argument: zeLabAddress},
	}
}
