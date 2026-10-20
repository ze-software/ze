// Design: docs/architecture/testing/interop.md -- a VPN-IPv4 route an ExaBGP script announces through the bridge, judged by FRR.
// Related: test/interop/scenarios/bgp-exabgp-bridge-vpnv4-frr/ze.conf -- the script and the bridge config.
// Related: register_exabgp_bridge_rtc.go -- the same shape for Route Target membership.
package bgp

import "time"

// scenarioExaBGPBridgeVPNv4FRR is AC-27 of the ExaBGP 6 API spec judged by
// another implementation. An ExaBGP script writes `announce ipv4 mpls-vpn
// 10.0.0.0/24 next-hop ... rd 65001:100 label 100`; the bridge translates it
// to `update text ... mp ipv4/mpls-vpn add ...`, SAFI 128. FRR must hold the
// route under its Route Distinguisher with its label (RFC 4364 Section 4.3.4,
// RFC 8277 Section 2).
//
// Without the bridge's mpls-vpn row (bridgeSAFI) the bridge refuses the line,
// so nothing leaves ze and the first wait below times out.
const (
	scenarioExaBGPBridgeVPNv4FRR = "bgp-exabgp-bridge-vpnv4-frr"

	exabgpBridgeVPNv4Prefix = "10.0.0.0/24"
	exabgpBridgeVPNv4RD     = "65001:100"
	// FRR's route detail prints the label it decoded as "Remote label: <n>"
	// (bgpd/bgp_route.c, route_vty_out_detail).
	exabgpBridgeVPNv4Label = "Remote label: 100"
)

func init() {
	scenarioOperations[scenarioExaBGPBridgeVPNv4FRR] = []operation{
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opWaitContains, peer: peerFRR,
			command:  []string{cmdVtysh, "-c", "show bgp ipv4 vpn rd " + exabgpBridgeVPNv4RD + " " + exabgpBridgeVPNv4Prefix},
			contains: []string{exabgpBridgeVPNv4Prefix, exabgpBridgeVPNv4RD, exabgpBridgeVPNv4Label},
			timeout:  60 * time.Second},
		{kind: opFRRSession, argument: zeLabAddress},
	}
}
