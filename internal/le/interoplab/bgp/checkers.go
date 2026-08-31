// Design: docs/architecture/testing/interop.md -- typed scenario registry.
// Related: check_engine.go -- operation semantics and fail-closed assertions.
package bgp

import (
	"context"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkers returns the complete typed checker registry for test/interop/scenarios.
func checkers() map[string]interoplab.Checker {
	checkers := make(map[string]interoplab.Checker, len(scenarioOperations))
	for name := range scenarioOperations {
		scenarioName := name
		checkers[name] = func(ctx context.Context, check *interoplab.CheckContext) error {
			return checkScenario(ctx, check, scenarioName)
		}
	}
	for name, checker := range specialCheckers {
		scenarioName := name
		scenarioChecker := checker
		checkers[name] = func(ctx context.Context, check *interoplab.CheckContext) error {
			if err := scenarioChecker(ctx, check); err != nil {
				return checkerFailure(ctx, check.Lab, scenarioName, 0, err)
			}
			return nil
		}
	}
	return checkers
}

var scenarioOperations = map[string][]operation{
	"as112-community-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: as112DirectDelegationPrefix},
		{kind: opFRRRoute, argument: as112DNAMERedirectionPrefix},
		{kind: opFRRCommunity, argument: as112DirectDelegationPrefix, contains: []string{communityNoExport}},
		{kind: opFRRCommunity, argument: as112DNAMERedirectionPrefix, contains: []string{communityNoPeer}},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	scenarioAS112OriginASFRR: {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opFRRRoute, argument: as112DirectDelegationPrefix},
		{kind: opBIRDRoute, argument: as112DirectDelegationPrefix},
		{kind: opFRRRoute, argument: as112DNAMERedirectionPrefix},
		{kind: opBIRDRoute, argument: as112DNAMERedirectionPrefix},
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
	},
	"as112-redistribute-community-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: as112DirectDelegationPrefix},
		{kind: opFRRRoute, argument: as112DNAMERedirectionPrefix},
		{kind: opFRRCommunity, argument: as112DirectDelegationPrefix, contains: []string{communityNoPeer}},
		{kind: opFRRCommunity, argument: as112DNAMERedirectionPrefix, contains: []string{communityNoPeer}},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"as112-redistribute-lab": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: as112DirectDelegationPrefix},
		{kind: opFRRRoute, argument: as112DNAMERedirectionPrefix},
		{kind: opFRRRouteAbsent, argument: as112DirectDelegationHostRoute},
		{kind: opFRRRouteAbsent, argument: as112DNAMERedirectionHostRoute},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"as112-redistribute-origin-custom-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: as112DirectDelegationPrefix},
		{kind: opFRRRoute, argument: as112DNAMERedirectionPrefix},
		{kind: opFRRRouteAbsent, argument: as112DirectDelegationHostRoute},
		{kind: opFRRRouteAbsent, argument: as112DNAMERedirectionHostRoute},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"as112-redistribute-origin-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opFRRRoute, argument: as112DirectDelegationPrefix},
		{kind: opBIRDRoute, argument: as112DirectDelegationPrefix},
		{kind: opFRRRoute, argument: as112DNAMERedirectionPrefix},
		{kind: opBIRDRoute, argument: as112DNAMERedirectionPrefix},
		{kind: opFRRRouteAbsent, argument: as112DirectDelegationHostRoute},
		{kind: opFRRRouteAbsent, argument: as112DNAMERedirectionHostRoute},
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
	},
	"bgp-4byte-asn-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
	},
	// RFC 6793 Section 4.2.2, judged by BIRD rather than by ze's own encoder. The
	// route reaches BIRD over a session ze holds to two octets, so the four-octet
	// aggregating AS can only arrive in the AS4_AGGREGATOR companion. Asking BIRD
	// for it is what makes this an interop test: ze's unit tests pin the octets ze
	// writes, and only a second implementation says whether they can be read.
	"bgp-aggregator-as4-downgrade-bird": {
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opBIRDRoute, argument: aggregatorDowngradePrefix, timeout: 90 * time.Second},
		{kind: opRequireContains, peer: peerBIRD, command: []string{cmdBirdc, "show route for " + aggregatorDowngradePrefix + " all"}, contains: []string{aggregatorDowngradePrefix, aggregatorDowngradeAS}},
		{kind: opBIRDSession, argument: birdZeProtocol},
	},
	// RFC 5880 Section 6.7.2 Simple Password, judged by BIRD rather than by ze's
	// own verifier. The session reaches Up only when BIRD accepts the section ze
	// signs AND ze accepts the section BIRD signs, so a disagreement over the
	// Auth Len arithmetic, the Key ID placement, or where the password starts
	// leaves it down. ze's unit tests pin the bytes ze writes; only a second
	// implementation says whether they can be read.
	"bfd-simple-password-bird": {
		{kind: opWaitContains, peer: peerBIRD, command: []string{cmdBirdc, birdShowBFDSessions}, contains: []string{zeLabAddress, birdBFDStateUp}, timeout: 90 * time.Second},
	},
	scenarioAddPathFRR: {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-community-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opFRRRoute, argument: injectPrefixSecond},
		{kind: opFRRCommunity, argument: injectPrefixFirst, contains: []string{"65001:100"}},
		{kind: opFRRCommunity, argument: injectPrefixFirst, contains: []string{"65001:200"}},
		{kind: opFRRCommunity, argument: injectPrefixSecond, contains: []string{"65001:0:1"}},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-ebgp-gobgp": {
		{kind: opGoBGPSession, argument: zeLabAddress},
	},
	"bgp-ebgp-ipv4-bird": {
		{kind: opBIRDSession, argument: birdZeProtocol},
	},
	"bgp-ebgp-ipv4-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
	},
	scenarioECMPFRR: {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRSession, argument: gobgpLabAddress},
		{kind: opExec, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, ecmpPrefix, "-a", gobgpFamilyIPv4, gobgpNextHop, gobgpLabAddress}},
		{kind: opFRRRoute, argument: ecmpPrefix, timeout: 30 * time.Second},
	},
	scenarioEVPNFRR: {
		{kind: opFRRSession, argument: zeLabAddress},
	},
	scenarioEVPNGoBGP: {
		{kind: opGoBGPSession, argument: zeLabAddress},
	},
	"bgp-extended-community-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	scenarioFlowspecFRR: {
		{kind: opFRRSession, argument: zeLabAddress},
	},
	scenarioFlowspecGoBGP: {
		{kind: opGoBGPSession, argument: zeLabAddress},
	},
	"bgp-flowspec-sctp-gobgp": {
		{kind: opGoBGPSession, argument: zeLabAddress},
		{kind: opExec, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", gobgpFamilyIPv4Flowspec, gobgpAdd, "match", "destination", flowspecMatchPrefix, "protocol", "==sctp", "then", "discard"}},
		{kind: opWaitContains, peer: "ze", command: []string{"nft", "list", "ruleset"}, contains: []string{"sctp", flowspecMatchPrefix}, timeout: 30 * time.Second},
		{kind: opExec, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", gobgpFamilyIPv4Flowspec, "del", "match", "destination", flowspecMatchPrefix, "protocol", "==sctp", "then", "discard"}},
		{kind: opWaitAbsent, peer: "ze", command: []string{"nft", "list", "ruleset"}, absent: []string{flowspecMatchPrefix}, proof: []string{"table"}, timeout: 30 * time.Second},
		{kind: opGoBGPSession, argument: zeLabAddress},
	},
	scenarioGracefulRestartFRR: {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand(zeShowBGPRIBStatus), minimum: map[string]int{fieldRoutesIn: 1}},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-gtsm-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-ibgp-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-ipv6-ebgp-bird": {
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opBIRDRoute, argument: injectPrefixV6First},
		{kind: opBIRDRoute, argument: injectPrefixV6Second},
		{kind: opBIRDRoute, argument: injectPrefixV6Third},
		{kind: opBIRDSession, argument: birdZeProtocol},
	},
	"bgp-ipv6-ebgp-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: injectPrefixV6First, family: frrFamilyIPv6Unicast},
		{kind: opFRRRoute, argument: injectPrefixV6Second, family: frrFamilyIPv6Unicast},
		{kind: opFRRRoute, argument: injectPrefixV6Third, family: frrFamilyIPv6Unicast},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-ipv6-ebgp-gobgp": {
		{kind: opGoBGPSession, argument: zeLabAddress},
		{kind: opGoBGPRoute, argument: injectPrefixV6First, family: frrFamilyIPv6Unicast},
		{kind: opGoBGPRoute, argument: injectPrefixV6Second, family: frrFamilyIPv6Unicast},
		{kind: opGoBGPRoute, argument: injectPrefixV6Third, family: frrFamilyIPv6Unicast},
		{kind: opGoBGPSession, argument: zeLabAddress},
	},
	"bgp-max-prefix-cease-frr": {
		{kind: opWaitLogContains, peer: "ze", contains: []string{"prefix count exceeded maximum"}, timeout: 90 * time.Second},
		{kind: opWaitAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp neighbor 172.30.0.2"}, absent: []string{"BGP state = Established"}, proof: []string{"BGP neighbor is"}, timeout: 30 * time.Second},
		{kind: opExec, peer: peerFRR, command: []string{cmdVtysh, "-c", frrConfigureTerminal, "-c", "no ip route 10.45.1.0/24 Null0"}},
		{kind: opFRRSession, argument: zeLabAddress, timeout: 60 * time.Second},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-md5-auth-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
	},
	scenarioMEDIBGPPostSelectionRemovalGoBGP: {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opGoBGPSession, argument: zeLabAddress},
		{kind: opGoBGPRoute, argument: medPrefix},
	},
	"bgp-multihop-ebgp-bird": {
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opBIRDRoute, argument: injectPrefixFirst},
		{kind: opBIRDRoute, argument: injectPrefixSecond},
		{kind: opBIRDSession, argument: birdZeProtocol},
	},
	"bgp-multihop-ebgp-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opFRRRoute, argument: injectPrefixSecond},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-multihop-ebgp-gobgp": {
		{kind: opGoBGPSession, argument: zeLabAddress},
		{kind: opGoBGPRoute, argument: injectPrefixFirst},
		{kind: opGoBGPRoute, argument: injectPrefixSecond},
		{kind: opGoBGPSession, argument: zeLabAddress},
	},
	"bgp-paths-limit-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-policy-import-export-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: "ze_policy"},
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: "ze_policy"},
	},
	"bgp-redist-late-join-dynamic-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: peerPrefixFirst},
	},
	"bgp-relay-withdraw-nexthop-self-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: injectPrefixFirst, timeout: 60 * time.Second},
		{kind: opFRRRouteAbsent, argument: injectPrefixFirst, timeout: 90 * time.Second},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-remove-private-as-as4path-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opExec, peer: peerBIRD, command: []string{cmdBirdc, "enable static_routes"}},
		{kind: opFRRRoute, argument: peerPrefixSecond},
		{kind: opFRRNoAS, argument: peerPrefixSecond, absent: []string{"4200000000"}},
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
	},
	"bgp-remove-private-as-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opGoBGPSession, argument: zeLabAddress},
		{kind: opExec, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, peerPrefixFirst, "-a", gobgpFamilyIPv4, gobgpNextHop, gobgpLabAddress}},
		{kind: opFRRRoute, argument: peerPrefixFirst},
		{kind: opFRRNoAS, argument: peerPrefixFirst, absent: []string{"64512"}},
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opGoBGPSession, argument: zeLabAddress},
	},
	"bgp-rfc7606-speaker-dup-attr": {
		{kind: opWaitLogFields, peer: peerSpeaker, timeout: 120 * time.Second, fields: map[string]string{fieldEstablished: logValueYes, "result": "PASS"}, minimum: map[string]int{"route-bearing-updates": 1}},
	},
	"bgp-role-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: peerPrefixFirst, timeout: 30 * time.Second},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-role-gobgp": {
		{kind: opGoBGPSession, argument: zeLabAddress},
		{kind: opDelayRequireContains, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpNeighbor, zeLabAddress}, contains: []string{"Established"}, delay: 5 * time.Second},
	},
	"bgp-route-reflection-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: "ze_rr"},
		{kind: opBIRDRoute, argument: "10.38.0.0/24"},
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: "ze_rr"},
	},
	"bgp-route-refresh-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opFRRRoute, argument: injectPrefixSecond},
		{kind: opFRRRoute, argument: injectPrefixThird},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opFRRRoute, argument: injectPrefixSecond},
		{kind: opFRRRoute, argument: injectPrefixThird},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-route-withdrawal-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opFRRRoute, argument: injectPrefixSecond},
		{kind: opFRRRoute, argument: injectPrefixThird},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opFRRRoute, argument: injectPrefixSecond},
		{kind: opFRRRoute, argument: injectPrefixThird},
		{kind: opFRRRouteAbsent, argument: injectPrefixSecond, timeout: 30 * time.Second},
		{kind: opFRRRouteAbsent, argument: injectPrefixSecond},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opFRRRoute, argument: injectPrefixThird},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-routes-from-bird": {
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opBIRDRoute, argument: "10.0.0.0/24"},
		{kind: opBIRDRoute, argument: "10.0.1.0/24"},
		{kind: opBIRDRoute, argument: "10.0.2.0/24"},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand(zeShowBGPRIBStatus), minimum: map[string]int{fieldRoutesIn: 3}},
		{kind: opBIRDSession, argument: birdZeProtocol},
	},
	"bgp-routes-from-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: "10.0.0.0/24"},
		{kind: opFRRRoute, argument: "10.0.1.0/24"},
		{kind: opFRRRoute, argument: "10.0.2.0/24"},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand(zeShowBGPRIBStatus), minimum: map[string]int{fieldRoutesIn: 3}},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-routes-gobgp": {
		{kind: opGoBGPSession, argument: zeLabAddress},
		{kind: opGoBGPRoute, argument: injectPrefixFirst},
		{kind: opGoBGPRoute, argument: injectPrefixSecond},
		{kind: opExec, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, "10.20.0.0/24", "-a", gobgpFamilyIPv4, gobgpNextHop, gobgpLabAddress}},
		{kind: opExec, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, "10.20.1.0/24", "-a", gobgpFamilyIPv4, gobgpNextHop, gobgpLabAddress}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand(zeShowBGPRIBStatus), minimum: map[string]int{fieldRoutesIn: 2}},
		{kind: opGoBGPSession, argument: zeLabAddress},
	},
	"bgp-routes-to-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opFRRRoute, argument: injectPrefixSecond},
		{kind: opFRRRoute, argument: injectPrefixThird},
	},
	"bgp-send-community-suppress-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
	},
	"bgp-speaker-two-instance": {
		{kind: opWaitLogFields, peer: peerSpeaker, timeout: 90 * time.Second, fields: map[string]string{fieldEstablished: logValueYes}},
		{kind: opWaitLogFields, peer: peerSpeaker2, timeout: 90 * time.Second, fields: map[string]string{fieldEstablished: logValueYes}},
	},
	"bgp-srv6-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"bgp-triangle": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRSession, argument: "172.30.0.4"},
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opBIRDSession, argument: "frr_peer"},
		{kind: opBIRDRoute, argument: peerPrefixFirst},
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRSession, argument: "172.30.0.4"},
		{kind: opBIRDSession, argument: birdZeProtocol},
	},
	scenarioVPNFRR: {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: peerPrefixFirst, family: frrFamilyIPv4VPN},
		{kind: opFRRRoute, argument: peerPrefixSecond, family: frrFamilyIPv4VPN},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	scenarioVPNGoBGP: {
		{kind: opGoBGPSession, argument: zeLabAddress},
	},
	"bmp-frr": {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: "10.44.0.0/24"},
	},
	"isis-auth-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, contains: []string{"Up"}, timeout: 90 * time.Second},
	},
	"isis-convergence-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, contains: []string{"Up"}, timeout: 90 * time.Second},
		{kind: opExec, peer: peerFRR, command: []string{"ip", ipObjectLink, ipActionSet, containerInterface, linkDown}},
		{kind: opWaitAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, absent: []string{"Up"}, proof: []string{"System"}, timeout: 30 * time.Second},
		{kind: opExec, peer: peerFRR, command: []string{"ip", ipObjectLink, ipActionSet, containerInterface, "up"}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, contains: []string{"Up"}, timeout: 90 * time.Second},
	},
	"isis-dualstack-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, contains: []string{"Up"}, timeout: 90 * time.Second},
	},
	"isis-lan-dis-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, contains: []string{"Up"}, timeout: 90 * time.Second},
	},
	"isis-redist-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, contains: []string{"Up"}, timeout: 60 * time.Second},
	},
	"ospf-auth-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-bfd-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowBFDPeers}, contains: []string{zeLabAddress, bfdStatusUp}, timeout: 60 * time.Second},
		{kind: opExec, peer: peerFRR, command: []string{"ip", ipObjectLink, ipActionSet, containerInterface, linkDown}},
		{kind: opWaitAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, absent: []string{ospfStateFull}, proof: []string{ospfNeighborHeading}, timeout: 15 * time.Second},
		{kind: opExec, peer: peerFRR, command: []string{"ip", ipObjectLink, ipActionSet, containerInterface, "up"}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowBFDPeers}, contains: []string{zeLabAddress, bfdStatusUp}, timeout: 60 * time.Second},
	},
	"ospf-broadcast-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-convergence-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
		{kind: opExec, peer: peerFRR, command: []string{"ip", ipObjectLink, ipActionSet, containerInterface, linkDown}},
		{kind: opWaitAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, absent: []string{ospfStateFull}, proof: []string{ospfNeighborHeading}, timeout: 30 * time.Second},
		{kind: opExec, peer: peerFRR, command: []string{"ip", ipObjectLink, ipActionSet, containerInterface, "up"}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-debug-inject-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-debug-te-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-ext-prefix-link-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-gr-fib-retention": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-gr-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-ipsec-ah-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
		{kind: opRequireContains, peer: "ze", command: []string{"ip", "-6", ipObjectXfrm, xfrmObjectState}, contains: []string{xfrmProtoAH, xfrmModeTransport, xfrmAuthTruncSHA256}},
		{kind: opRequireContains, peer: peerFRR, command: []string{"ip", "-6", ipObjectXfrm, xfrmObjectState}, contains: []string{xfrmProtoAH, xfrmModeTransport, xfrmAuthTruncSHA256}},
		{kind: opRequireAbsent, peer: "ze", command: []string{"ip", "-6", ipObjectXfrm, xfrmObjectState}, absent: []string{" enc "}, proof: []string{xfrmProtoAH, xfrmModeTransport}},
		{kind: opRequireContains, peer: "ze", command: []string{"ip", "-6", ipObjectXfrm, "policy"}, contains: []string{"dir out", "proto ospf", "tmpl", xfrmProtoAH, xfrmModeTransport}},
	},
	"ospf-ipsec-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
		{kind: opRequireContains, peer: "ze", command: []string{"ip", "-6", ipObjectXfrm, xfrmObjectState}, contains: []string{xfrmProtoESP, xfrmModeTransport, xfrmAuthTruncSHA256}},
		{kind: opRequireContains, peer: peerFRR, command: []string{"ip", "-6", ipObjectXfrm, xfrmObjectState}, contains: []string{xfrmProtoESP, xfrmModeTransport, xfrmAuthTruncSHA256}},
		{kind: opRequireContains, peer: "ze", command: []string{"ip", "-6", ipObjectXfrm, "policy"}, contains: []string{"dir out", "proto ospf", "tmpl", xfrmProtoESP, xfrmModeTransport}},
	},
	"ospf-ldp-sync-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-multiaf-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-multiaf-v4-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}},
	},
	"ospf-multiarea-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-multiinstance-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 120 * time.Second},
	},
	"ospf-nbma-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 120 * time.Second},
	},
	"ospf-opaque-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-p2p-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-ptmp-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-ri-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-sr-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-te-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-te-interas-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospf-virtual-link-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ip route ospf"}, contains: []string{"192.0.2.0/24"}, timeout: 60 * time.Second},
	},
	"ospfv3-bfd-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowBFDPeers}, contains: []string{bfdStatusUp}, timeout: 60 * time.Second},
		{kind: opExec, peer: peerFRR, command: []string{"ip", ipObjectLink, ipActionSet, containerInterface, linkDown}},
		{kind: opWaitAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, absent: []string{ospfStateFull}, proof: []string{ospfNeighborHeading}, timeout: 15 * time.Second},
		{kind: opExec, peer: peerFRR, command: []string{"ip", ipObjectLink, ipActionSet, containerInterface, "up"}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowBFDPeers}, contains: []string{bfdStatusUp}, timeout: 60 * time.Second},
	},
	"ospfv3-broadcast-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospfv3-debug-decode-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospfv3-debug-inject-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospfv3-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospfv3-gr-fib-retention": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospfv3-gr-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospfv3-multiarea-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Route}, contains: []string{"2001:db8:a1::/64"}, timeout: 90 * time.Second},
	},
	"ospfv3-nbma-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}},
	},
	"ospfv3-nssa-redist-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
		{kind: opGoBGPSession, argument: zeLabAddress},
		{kind: opExec, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, ospf6NSSAPrefix, "-a", "ipv6", gobgpNextHop, "fd00:1e:0::5"}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Route}, contains: []string{ospf6NSSAPrefix}, timeout: 90 * time.Second},
	},
	"ospfv3-ptmp-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 120 * time.Second},
	},
	"ospfv3-redist-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
		{kind: opGoBGPSession, argument: zeLabAddress},
		{kind: opExec, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, "2001:db8:5e5::/48", "-a", "ipv6", gobgpNextHop, "fd00:1e:0::5"}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Route}, contains: []string{"2001:db8:5e5::/48"}, timeout: 90 * time.Second},
	},
	"ospfv3-ri-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	},
	"ospfv3-sr-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 120 * time.Second},
	},
	"ospfv3-stub-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Route}, contains: []string{"::/0"}, timeout: 90 * time.Second},
	},
	"ospfv3-vlink-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}},
	},
	scenarioRPKIFRR: {
		{kind: opFRRSession, argument: zeLabAddress},
	},
	"rtr-stayrtr": {
		{kind: opWaitContains, peer: peerStayRTR, command: []string{"wget", "-q", "-O", "-", "http://127.0.0.1:9847/rpki.json"}, contains: []string{"prefix"}},
	},
	scenarioShutdownCeaseFRR: {
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: injectPrefixFirst, timeout: 60 * time.Second},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opSignal, peer: "ze", argument: "TERM"},
		{kind: opFRRRouteAbsent, argument: injectPrefixFirst, timeout: 30 * time.Second},
	},
	"vrrp-mastership-keepalived": {
		{kind: opWaitContains, peer: "ze", command: []string{"ip", "-o", "-f", ipFamilyInet, ipObjectAddr}, contains: []string{vrrpVirtualAddress}, timeout: 40 * time.Second},
		{kind: opRequireAbsent, peer: peerKeepalived, command: []string{"ip", "-o", "-f", ipFamilyInet, ipObjectAddr}, absent: []string{vrrpVirtualAddress}, proof: []string{containerInterface}},
		{kind: opSignal, peer: "ze", argument: "TERM"},
		{kind: opWaitContains, peer: peerKeepalived, command: []string{"ip", "-o", "-f", ipFamilyInet, ipObjectAddr}, contains: []string{vrrpVirtualAddress}, timeout: 12 * time.Second},
		{kind: opStart, peer: "ze"},
		{kind: opWaitContains, peer: "ze", command: []string{"ip", "-o", "-f", ipFamilyInet, ipObjectAddr}, contains: []string{vrrpVirtualAddress}, timeout: 40 * time.Second},
		{kind: opRequireAbsent, peer: peerKeepalived, command: []string{"ip", "-o", "-f", ipFamilyInet, ipObjectAddr}, absent: []string{vrrpVirtualAddress}, proof: []string{containerInterface}},
	},
}
