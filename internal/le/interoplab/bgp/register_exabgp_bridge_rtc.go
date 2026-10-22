// Design: docs/architecture/testing/interop.md -- a Route Target membership an ExaBGP script announces through the bridge, judged by GoBGP.
// Related: test/interop/scenarios/bgp-exabgp-bridge-rtc-gobgp/ze.conf -- the script and the bridge config.
// Related: register_exabgp_bridge_sr_policy.go -- the same shape for SR Policy.
package bgp

import "time"

// scenarioExaBGPBridgeRTCGoBGP is AC-26 of the ExaBGP 6 API spec judged by
// another implementation. An ExaBGP script writes `announce ipv4 rtc
// origin-as 65001 route-target 65001:100 next-hop ...`; the bridge translates
// it to `update text ... mp ipv4/rtc add ...`, which the rtc plugin encodes as
// the RFC 4684 Section 4 NLRI. GoBGP must hold the membership. The default
// membership is not sent: GoBGP v3.31.0 panicked on one, and v4.10.0 no
// longer does, but the scenario was not widened when the image moved (ze.conf).
//
// Without the bridge's rtc row (bridgeSAFI) the bridge refuses the line, so
// nothing leaves ze and the first wait below times out.
const (
	scenarioExaBGPBridgeRTCGoBGP = "bgp-exabgp-bridge-rtc-gobgp"

	// GoBGP v4.10.0's RouteTargetMembershipNLRI.String prints "<origin
	// AS>:<route target>/<prefix length in bits>" (pkg/packet/bgp/bgp.go), so
	// this membership reads "65001:65001:100/96".
	exabgpBridgeRTCMembership = "65001:65001:100"
)

func init() {
	scenarioOperations[scenarioExaBGPBridgeRTCGoBGP] = []operation{
		{kind: opGoBGPSession, argument: zeLabAddress},
		{kind: opWaitContains, peer: peerGoBGP, command: []string{cmdGoBGPPaths, gobgpAFIIPv4, gobgpSAFIRTC},
			contains: []string{exabgpBridgeRTCMembership},
			timeout:  60 * time.Second},
		{kind: opGoBGPSession, argument: zeLabAddress},
	}
}
