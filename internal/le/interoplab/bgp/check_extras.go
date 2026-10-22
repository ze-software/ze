// Design: docs/architecture/testing/interop.md -- scenario-specific peer evidence.
// Related: checkers.go -- common session, route, and adjacency assertions.
// RFC: rfc/short/rfc5709.md -- Section 3.2, the expired last key keeps verifying.
// RFC: rfc/short/rfc3623.md -- Section 3.1, the broadcast helper names X by interface address.
package bgp

import (
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The RFC 9136 IP Prefix route the EVPN scenarios announce with the ESI as
// its Overlay Index, and the fields FRR must show for it.
const (
	evpnOverlayESI           = "00:01:02:03:04:05:06:07:08:09"
	evpnOverlayPrefixAddress = "10.77.0.0"
	evpnOverlayPrefix        = evpnOverlayPrefixAddress + "/24"
	// frrEVPNOverlayRoute is how FRR keys the route in its EVPN table.
	frrEVPNOverlayRoute = "[5]:[0]:[24]:[" + evpnOverlayPrefixAddress + "]"
	// gobgpEVPNOverlayNLRI is GoBGP's JSON reading of the route's NLRI: the
	// ESI octets 00 01..09 (type 0, arbitrary) beside the prefix and an
	// all-zero GW IP. The A-D routes carry the same ESI, so the needle
	// spans the prefix to pin it on the IP Prefix route.
	gobgpEVPNOverlayNLRI = `"esi":"ESI_ARBITRARY | 01:02:03:04:05:06:07:08:09","etag":0,"prefix":"` + evpnOverlayPrefix + `","gateway":"0.0.0.0"`
)

// The ospf-gr-helper-broadcast-frr facts. FRR's router-id in frr.conf, which is
// deliberately not its eth0 address, and the Ze counter whose "flushed" series
// exists only after a helper session was entered and ended by the flush.
const (
	ospfGRBroadcastFRRRouterID = "10.255.0.3"
	ospfGRHelperExitsMetric    = "ze_ospf_gr_helper_exits_total"
)

var ospfAdjacencyContinuity = operation{
	kind:    opRequireAbsent,
	peer:    "ze",
	command: zeCommand("show metrics values"),
	absent:  []string{"ll-down"},
	proof:   []string{"ze_ospf_nsm_events_total", "exchange-done"},
}

var scenarioExtras = map[string][]operation{
	scenarioAS112OriginASFRR: {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowAS112PrefixJSON}, contains: []string{as112DirectDelegationPrefix, "112"}},
		{kind: opRequireContains, peer: peerBIRD, command: []string{cmdBirdc, "show route for 192.175.48.0/24 all"}, contains: []string{as112DirectDelegationPrefix, "65001", "112"}},
	},
	"as112-redistribute-lab": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowAS112PrefixJSON}, contains: []string{as112DirectDelegationPrefix, "65001", "112"}},
	},
	"as112-redistribute-origin-custom-frr": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowAS112PrefixJSON}, contains: []string{as112DirectDelegationPrefix, "65001", "112"}},
	},
	"as112-redistribute-origin-frr": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowAS112PrefixJSON}, contains: []string{as112DirectDelegationPrefix, "112"}},
		{kind: opRequireContains, peer: peerBIRD, command: []string{cmdBirdc, "show route for 192.175.48.0/24 all"}, contains: []string{as112DirectDelegationPrefix, "65001", "112"}},
	},
	"bgp-4byte-asn-frr": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON}, contains: []string{"4200000000", stateEstablished}},
	},
	// FRR 10.3 prints "Paths: (2 available, ...)" and one "AddPath ID: RX <id>,"
	// line per path, so the two identifiers ze wrote as 0.0.0.1 and 0.0.0.2
	// must arrive as 1 and 2.
	scenarioAddPathFRR: {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 unicast 10.10.0.0/24"}, contains: []string{injectPrefixFirst, "Paths: (2 available", "AddPath ID: RX 1,", "AddPath ID: RX 2,"}},
	},
	scenarioECMPFRR: {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 unicast 10.100.0.0/24 json"}, contains: []string{ecmpPrefix, zeLabAddress, gobgpLabAddress}},
	},
	scenarioEVPNFRR: {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON}, contains: []string{"l2VpnEvpn", frrCapabilityNegotiated}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp l2vpn evpn"}, contains: []string{"00:11:22:33:44:55", "00:11:22:33:44:66"}, timeout: 30 * time.Second},
		// RFC 9136 Section 3.1: FRR accepts the IP Prefix route announced
		// with overlay-index esi. FRR 10.3.1 prints no type-5 ESI in its
		// "route type prefix", "route detail" or "rd <rd> overlay" views,
		// so bgp-evpn-gobgp asserts the ESI octets.
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp l2vpn evpn route type prefix"}, contains: []string{frrEVPNOverlayRoute}, timeout: 30 * time.Second},
	},
	// RFC 7432 Section 8.2.1: the two configured Ethernet A-D per ES routes
	// reach GoBGP with the ESI Label each redundancy-mode requires. GoBGP
	// renders the three label octets as one 24-bit value: 001000 is 4096 and
	// 002000 is 8192.
	scenarioEVPNGoBGP: {
		{kind: opWaitContains, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", "evpn", "-j"}, contains: []string{"00:11:22:33:44:55", "00:11:22:33:44:66",
			`"subtype":1,"label":4096,"is_single_active":false`, `"subtype":1,"label":8192,"is_single_active":true`}, timeout: 30 * time.Second},
		// RFC 9136 Section 3.1: the IP Prefix route announced with
		// overlay-index esi reaches GoBGP with its non-zero ESI and a zero
		// GW IP in the NLRI.
		{kind: opWaitContains, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", "evpn", "-j"}, contains: []string{gobgpEVPNOverlayNLRI}, timeout: 30 * time.Second},
	},
	// RFC 9830 origination: both policies arrive with their NLRI fields, the
	// Binding SID label at the top of the 20-bit range and both Type A segment
	// labels exactly as configured.
	scenarioSRPolicyGoBGP: {
		{kind: opWaitContains, peer: peerGoBGP, command: []string{cmdGoBGPPaths, gobgpAFIIPv4, gobgpSAFISRPolicy}, contains: []string{srPolicyOriginatedIPv4, srPolicyBindingSIDTop, srPolicySegmentLabel16, srPolicySegmentLabelTop, srPolicyOriginatedBSIDSRv6, srPolicyBindingSIDSRv6}, timeout: 60 * time.Second},
		{kind: opWaitContains, peer: peerGoBGP, command: []string{cmdGoBGPPaths, gobgpAFIIPv6, gobgpSAFISRPolicy}, contains: []string{srPolicyOriginatedIPv6, srPolicyBindingSIDTop, srPolicySegmentLabel16, srPolicySegmentLabelTop}, timeout: 60 * time.Second},
		{kind: opGoBGPSession, argument: zeLabAddress},
	},
	// RFC 9830 Section 5: A, B and the control C arrive well-formed; A beside
	// a 13-octet NLRI is treat-as-withdraw, then B without attribute 23 is.
	// Each absence carries C as its proof, so a dead Ze-GoBGP session or a
	// reset injector session (Ze withdraws C with it, and the injector takes
	// one connection only) fails the step instead of passing it. GoBGP v4.10.0
	// refuses an NLRI of a length other than 96 or 192 bits (SRPolicyNLRI.
	// DecodeFromBytes, pkg/packet/bgp/sr_policy.go), maps the MP_REACH error to
	// AFI/SAFI disable and then turns that into a session reset
	// (pkg/server/fsm.go, handlingError). v3.31.0 kept the session instead, and
	// the revert-red recorded then (2026-10-21, commit f40f1fea9b) went red at
	// A's absence because GoBGP refused the UPDATE and kept A. On v4.10.0 the
	// reset drops C, the step's proof, but a session that re-establishes inside
	// the wait brings C back without A, so whether a relayed 13-octet NLRI still
	// turns this step red is unproven until the revert is re-run on v4.10.0.
	scenarioSRPolicyMalformedReceiveGoBGP: {
		{kind: opWaitContains, peer: peerGoBGP, command: []string{cmdGoBGPPaths, gobgpAFIIPv4, gobgpSAFISRPolicy}, contains: []string{srPolicyReceivedA, srPolicyReceivedB, srPolicyReceivedControl}, timeout: 120 * time.Second},
		{kind: opWaitAbsent, peer: peerGoBGP, command: []string{cmdGoBGPPaths, gobgpAFIIPv4, gobgpSAFISRPolicy}, absent: []string{srPolicyReceivedA}, proof: []string{srPolicyReceivedControl}, timeout: 120 * time.Second},
		{kind: opWaitAbsent, peer: peerGoBGP, command: []string{cmdGoBGPPaths, gobgpAFIIPv4, gobgpSAFISRPolicy}, absent: []string{srPolicyReceivedB}, proof: []string{srPolicyReceivedControl}, timeout: 120 * time.Second},
		{kind: opRequireContains, peer: peerGoBGP, command: []string{cmdGoBGPPaths, gobgpAFIIPv4, gobgpSAFISRPolicy}, contains: []string{srPolicyReceivedControl}},
		{kind: opGoBGPSession, argument: zeLabAddress},
	},
	scenarioFlowspecFRR: {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON}, contains: []string{"ipv4Flowspec", frrCapabilityNegotiated}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 flowspec json"}, contains: []string{peerPrefixFirst, peerPrefixSecond}, timeout: 30 * time.Second},
	},
	scenarioMEDIBGPPostSelectionRemovalGoBGP: {
		{kind: opWaitLogContains, peer: "ze", contains: []string{"RAW-MED-DROP: removed MULTI_EXIT_DISC"}, timeout: 120 * time.Second},
		{kind: opRequireContains, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", gobgpFamilyIPv4, medPrefix}, contains: []string{medPrefix, "65005", zeLabAddress, "Med: 100"}},
	},
	scenarioPathASNLeakFRR: {
		// FRR announces both prefixes in one batch, so waiting for the clean
		// route at BIRD does not prove ze has finished with the leaked one. The
		// delay is that settling time, and the query doubles as the proof that
		// ze answers `show bgp rib` at all.
		{kind: opDelayRequireContains, peer: "ze", command: zeCommand("show bgp rib"), contains: []string{pathASNCleanPrefix}, delay: 5 * time.Second},
		// The import chain rejected the leaked path before the route was cached,
		// so ze holds the clean prefix and not the leaked one.
		{kind: opRequireAbsent, peer: "ze", command: zeCommand("show bgp rib"), absent: []string{pathASNLeakedPrefix}, proof: []string{pathASNCleanPrefix}},
		// And a foreign daemon downstream never saw it either. The clean prefix
		// is the proof, because it is the route ze DID relay on the same session:
		// a BIRD whose table is empty, or whose session never came up, cannot
		// satisfy it and cannot pass this absence by holding nothing.
		{kind: opBIRDRouteAbsent, argument: pathASNLeakedPrefix, proof: []string{pathASNCleanPrefix}},
		// The session survives the drop. That is the whole point of the filter:
		// a max-prefix teardown would take the clean route with the leak.
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: "ze_leak"},
	},
	"bgp-policy-import-export-frr": {
		// A WAIT, not an immediate read. BIRD reports the session Established as
		// soon as it is up, and the assertions above return at that instant,
		// which is one second before FRR has advertised anything. The needles
		// are the ones an immediate read carried, so this waits for the state it
		// already asserted and asserts nothing less.
		{kind: opWaitContains, peer: peerBIRD, command: []string{cmdBirdc, "show route for 10.39.1.0/24 all"}, contains: []string{"10.39.1.0/24", "BGP.local_pref: 250", "BGP.med: 77"}, timeout: 60 * time.Second},
		// The accepted prefix is the proof, because it is the route ze DID relay
		// on the same session: a BIRD whose table is empty, or whose session with
		// ze never came up, cannot satisfy it and cannot pass this absence by
		// holding nothing. The birdc banner satisfies every one of those states,
		// so it is not evidence that the export policy rejected anything.
		{kind: opBIRDRouteAbsent, argument: "10.39.2.0/24", proof: []string{"10.39.1.0/24"}},
	},
	"bgp-relay-withdraw-nexthop-self-frr": relayWithdrawalExtras(injectPrefixFirst),
	"bgp-send-community-suppress-frr": {
		{kind: opRequireAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 unicast 10.52.0.0/24"}, absent: []string{suppressedCommunityFirst, suppressedCommunitySecond, suppressedCommunityThird, suppressedLargeCommunity}, proof: []string{"10.52.0.0/24"}},
		{kind: opRequireAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 unicast 10.52.1.0/24"}, absent: []string{suppressedCommunityFirst, suppressedCommunitySecond, suppressedCommunityThird, suppressedLargeCommunity}, proof: []string{"10.52.1.0/24"}},
		{kind: opRequireAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 unicast 10.52.2.0/24"}, absent: []string{suppressedCommunityFirst, suppressedCommunitySecond, suppressedCommunityThird, suppressedLargeCommunity}, proof: []string{"10.52.2.0/24"}},
		{kind: opRequireAbsent, peer: peerBIRD, command: []string{cmdBirdc, "show route for 10.52.0.0/24 all"}, absent: []string{birdExtendedCommunityAttribute, birdLargeCommunityAttribute}, proof: []string{"10.52.0.0/24", birdSuppressedCommunityFirst, birdSuppressedCommunitySecond}},
		{kind: opRequireAbsent, peer: peerBIRD, command: []string{cmdBirdc, "show route for 10.52.1.0/24 all"}, absent: []string{birdExtendedCommunityAttribute, birdLargeCommunityAttribute}, proof: []string{"10.52.1.0/24", birdSuppressedCommunityFirst, birdSuppressedCommunitySecond}},
		{kind: opRequireAbsent, peer: peerBIRD, command: []string{cmdBirdc, "show route for 10.52.2.0/24 all"}, absent: []string{birdExtendedCommunityAttribute, birdLargeCommunityAttribute}, proof: []string{"10.52.2.0/24", birdSuppressedCommunityFirst, birdSuppressedCommunitySecond}},
	},
	"bgp-srv6-frr": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON}, contains: []string{"ipv6Vpn", frrCapabilityNegotiated}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv6 vpn"}, contains: []string{"2001:db8:customer::/48"}, timeout: 30 * time.Second},
		{kind: opWaitJSONFields, peer: "ze", command: zeCommand("show bgp rib count"), minimum: map[string]int{"count": 1}, timeout: 30 * time.Second},
		{kind: opFRRSession, argument: zeLabAddress},
	},
	scenarioVPNGoBGP: {
		{kind: opWaitContains, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", "vpnv4", "-j"}, contains: []string{peerPrefixFirst, peerPrefixSecond}, timeout: 30 * time.Second},
	},
	"bmp-frr": {
		{kind: opWaitContains, peer: peerBMP, command: []string{cmdCat, "/tmp/bmp-status.json"}, contains: []string{"initiation", "peer-up", "route-monitoring"}, timeout: 30 * time.Second},
	},
	// RFC 7854 Section 4.8, read back by a collector that is not ze. Every
	// field the pipeline filters on is pmacct's reading of ze's bytes: the
	// message type it parsed, the peer it attributed the report to, the
	// per-peer flags, and the stat type named out of pmacct's own table.
	scenarioStatisticsPMACCT: {
		{kind: opWaitContains, peer: peerPMACCT, command: []string{"sh", "-c", pmacctStatisticsReport},
			contains: []string{pmacctDuplicateCounterName}, timeout: 60 * time.Second},
		{kind: opWaitContains, peer: peerPMACCT, command: []string{"sh", "-c", pmacctStatisticsPeriodic},
			contains: []string{"periodic"}, timeout: 60 * time.Second},
	},
	"isis-auth-frr": {
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, contains: []string{"Up"}, delay: 5 * time.Second},
	},
	"isis-dualstack-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISDatabase}, contains: []string{"ze-ds"}, timeout: 60 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ipv6 route isis"}, contains: []string{"I"}, timeout: 60 * time.Second},
	},
	"isis-redist-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ip route isis"}, contains: []string{peerPrefixFirst}, timeout: 60 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 unicast"}, contains: []string{peerPrefixFirst}, timeout: 60 * time.Second},
	},
	"ospf-ldp-sync-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ip ospf database router 172.30.0.2"}, contains: []string{frrLabAddress, "TOS 0 Metric: 65535"}, timeout: 45 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ip ospf database router 172.30.0.2"}, contains: []string{frrLabAddress, "TOS 0 Metric: 10"}, timeout: 120 * time.Second},
	},
	"ospf-multiinstance-frr": {
		{kind: opWaitContains, peer: peerBIRD, command: []string{cmdBirdc, "show ospf neighbors ospf5"}, contains: []string{ospfStateFull}, timeout: 120 * time.Second},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{zeLabAddress, ospfStateFull}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospf-nbma-frr": {
		{kind: opWaitContainsAny, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseNetwork}, contains: []string{zeLabAddress, frrLabAddress}, timeout: 60 * time.Second},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseRouter}, contains: []string{zeLabAddress}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospf-ptmp-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseRouter}, contains: []string{zeLabAddress}, timeout: 60 * time.Second},
		// RFC 2328 Section 9.5: a point-to-multipoint interface elects no
		// Designated Router, so no router originates a network-LSA and FRR omits
		// the whole "Net Link States" section from `show ip ospf database`. FRR
		// prints that section with one row for each network-LSA it holds, so the
		// heading appears the moment a DR election produces one.
		//
		// ze's router id is the proof, because it is the advertising router of the
		// router-LSA FRR holds only over a working adjacency: an empty database, a
		// dead session, and a peer that received nothing each fail it, and none of
		// them can pass this absence by holding nothing. The section heading of the
		// type-filtered `show ip ospf database network` satisfies every one of
		// those states, so it is not evidence that a DR election was avoided.
		{kind: opRequireAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabase}, absent: []string{"Net Link States"}, proof: []string{zeLabAddress}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"isis-lan-dis-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISDatabase}, contains: []string{".00-01"}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, contains: []string{"Up"}, delay: 5 * time.Second},
	},
	"ospf-broadcast-frr": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{"Full/", "DR"}},
		// FRR spells the section "Net Link States", and it prints that heading for
		// every area whatever the area holds, so the heading alone says only that
		// vtysh answered. ze's router id is the network-LSA content: ze.conf gives
		// ze OSPF priority 100 so ze wins the election, and FRR reads back ze as
		// the link state id and as an attached router of the LSA ze originated.
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseNetwork}, contains: []string{"Net Link States", zeLabAddress}, timeout: 60 * time.Second},
	},
	"ospf-gr-frr": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseRouter}, contains: []string{zeLabAddress}},
		{kind: opExec, peer: peerFRR, command: []string{cmdVtysh, "-c", "clear ip ospf process graceful-restart"}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 30 * time.Second},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospf-multiaf-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6DatabaseRouter}, contains: []string{zeLabAddress}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	// The interop proof for the OSPF accept-lifetime receive window. FRR signs
	// correctly with the shared key, so nothing is wrong on the wire; Ze's copy of
	// that key is outside its accept-lifetime, so Ze refuses every packet and the
	// adjacency stays below Full. The 45-second delay is longer than the 40-second
	// dead interval, so an adjacency that was going to form has had the time. Ze's
	// own Hellos keep reaching FRR throughout, which is what the zeLabAddress proof
	// anchor asserts: absence of "Full" means refused, not silent.
	"ospf-accept-lifetime-frr": {
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{zeLabAddress}, delay: 45 * time.Second},
		{kind: opRequireAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, absent: []string{ospfStateFull}, proof: []string{zeLabAddress}},
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf interface"), contains: []string{"\"neighbor-count\": 0"}},
	},
	// The interop proof for RFC 5709 Section 3.2 on receive. Ze's only key has a
	// send-lifetime and an accept-lifetime that both closed in 2020, so it is the
	// chain's expired last key: Ze keeps signing with it (selectSendKey) and keeps
	// verifying FRR with it (lastKeyExtendedIndex). The base operation waits for
	// FRR's Full; Ze's own neighbor table must agree, and the 45-second delay
	// outlasts the 40-second dead interval, so a Full that verification could not
	// hold would have dropped by then. Without the receive extension Ze refuses
	// every FRR packet and FRR never leaves Init, so the base wait goes red.
	"ospf-accept-lifetime-last-key-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf neighbor"), contains: []string{zeOSPFNeighborFull}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 45 * time.Second},
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf neighbor"), contains: []string{zeOSPFNeighborFull}},
	},
	// Ze as RFC 3623 helper on a broadcast segment, against FRR's real planned
	// restart. FRR's router-id is not its eth0 address, so Ze finds the helped
	// neighbor only by matching the Grace-LSA's IP interface address TLV against
	// neighbor addresses (graceRestarterV4). No helper exit exists before the
	// restart; after it, the flush of FRR's Grace-LSA must end a helper session
	// with reason "flushed", which only a session that was entered and matched by
	// the flushed body can produce. strict-lsa-checking is off in ze.conf, and the
	// 100-second wait is shorter than the 120-second grace period, so neither a
	// topology-change nor a grace-expiry exit can stand in for it. Both spellings
	// of the series are accepted because the metrics text arrives inside a JSON
	// string. This proves acceptance of FRR's Grace-LSA, not the 50044040c5
	// identity fix: an honest FRR names itself in both the TLV and the Advertising
	// Router, so the pre-fix lookup passes too (ze.conf header).
	"ospf-gr-helper-broadcast-frr": {
		{kind: opWaitContains, peer: "ze", command: zeCommand("show ospf neighbor"), contains: []string{zeOSPFNeighborFull, ospfGRBroadcastFRRRouterID}, timeout: 60 * time.Second},
		{kind: opRequireAbsent, peer: "ze", command: zeCommand("show metrics values"), absent: []string{ospfGRHelperExitsMetric}, proof: []string{"ze_ospf_nsm_events_total"}},
		{kind: opExec, peer: peerFRR, command: []string{cmdVtysh, "-c", "clear ip ospf process graceful-restart"}},
		{kind: opWaitContainsAny, peer: "ze", command: zeCommand("show metrics values"), contains: []string{
			ospfGRHelperExitsMetric + `{family=\"ipv4\",reason=\"flushed\"}`,
			ospfGRHelperExitsMetric + `{family="ipv4",reason="flushed"}`,
		}, timeout: 100 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 60 * time.Second},
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf neighbor"), contains: []string{zeOSPFNeighborFull, ospfGRBroadcastFRRRouterID}},
	},
	// The interop proof for OSPF auto-cost: FRR reads the metric Ze derived out of
	// Ze's Router-LSA. Ze sets no `cost` on eth0 and a reference-bandwidth of
	// 470000 Mbit/s; eth0 is a Docker veth, whose driver declares 10 Gbit/s, so the
	// kernel reports 10000 and the derived cost is 47. Ze advertised 1 for every
	// interface with no `cost` before auto-cost existed, so the peer's view of 47 is
	// what discriminates the feature from its absence.
	// The native reload fixture supplies an explicit cost after Full. Restoring
	// the initial config removes that leaf. Each reload must reach FRR's LSDB.
	// InterfaceDown records ll-down in the persistent NSM event counter, so its
	// absence catches even a reset that has already re-established Full.
	"ospf-auto-cost-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseRouter}, contains: []string{zeLabAddress, "Metric: 47"}, timeout: 60 * time.Second},
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf interface"), contains: []string{"\"cost\": 47"}},
		ospfAdjacencyContinuity,
		{kind: opExec, peer: "ze", command: []string{"sh", "-c", "cp " + zeMountedReloadConfig + " " + zeRunningConfig}},
		{kind: opSignal, peer: "ze", argument: signalHUP},
		{kind: opWaitContains, peer: "ze", command: zeCommand("show ospf interface"), contains: []string{"\"cost\": 17"}, timeout: 30 * time.Second},
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf neighbor"), contains: []string{zeOSPFNeighborFull}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseRouter}, contains: []string{zeLabAddress, "Metric: 17"}, timeout: 30 * time.Second},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}},
		ospfAdjacencyContinuity,
		{kind: opExec, peer: "ze", command: []string{"sh", "-c", "cp " + zeMountedConfig + " " + zeRunningConfig}},
		{kind: opSignal, peer: "ze", argument: signalHUP},
		{kind: opWaitContains, peer: "ze", command: zeCommand("show ospf interface"), contains: []string{"\"cost\": 47"}, timeout: 30 * time.Second},
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf neighbor"), contains: []string{zeOSPFNeighborFull}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseRouter}, contains: []string{zeLabAddress, "Metric: 47"}, timeout: 30 * time.Second},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}},
		ospfAdjacencyContinuity,
	},
	"ospf-p2p-frr": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ip ospf interface"}, contains: []string{"POINTOPOINT"}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	// RFC 2328 Section 8.1: Ze sources its packets from the donor, FRR runs
	// the link unnumbered, and each side routes to the other's donor through
	// the link, Ze with the on-link gateway FRR also installs.
	"ospf-p2p-unnumbered-frr": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ip ospf interface eth0"}, contains: []string{"UNNUMBERED", "POINTOPOINT"}},
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf neighbor"), contains: []string{zeOSPFNeighborFull, frrUnnumberedDonor}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ip route " + zeUnnumberedDonor + "/32"}, contains: []string{"Known via \"ospf\"", "eth0"}, timeout: 30 * time.Second},
		{kind: opWaitContains, peer: "ze", command: []string{"ip", "-4", "route", ipActionShow, frrUnnumberedDonor}, contains: []string{"via " + frrUnnumberedDonor + " dev eth0", "onlink"}, timeout: 30 * time.Second},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospf-debug-inject-frr": {
		{kind: opExec, peer: "ze", command: zeCommand("debug ospf inject enable")},
		{kind: opExec, peer: "ze", command: zeCommand("debug ip ospf inject opaque scope area id 1 hex 0001000401020304")},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseOpaqueArea}, contains: []string{"1"}, timeout: 60 * time.Second},
		{kind: opExec, peer: "ze", command: zeCommand("debug ip ospf inject opaque scope area id 1 withdraw")},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 30 * time.Second},
	},
	"ospfv3-debug-inject-frr": {
		{kind: opExec, peer: "ze", command: zeCommand("debug ospf inject enable")},
		{kind: opExec, peer: "ze", command: zeCommand("debug ipv6 ospf inject lsa scope area type 0x2009 id 1 hex 00000000")},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ipv6 ospf6 database"}, contains: []string{"0x2009"}, timeout: 60 * time.Second},
		{kind: opExec, peer: "ze", command: zeCommand("debug ipv6 ospf inject lsa scope area type 0x2009 id 1 withdraw")},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, timeout: 30 * time.Second},
	},
	"ospfv3-debug-decode-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf ipv6 database detail"), contains: []string{"\"router\"", "\"scope\"", "\"decoded\""}},
	},
	// The opaque-area reads wait rather than read once at Full: FRR originates a
	// Type-10 opaque LSA on a timer that starts when an opaque-capable neighbor
	// appears (ospf_opaque_lsa_originate_schedule, delay min_ls_interval), so at
	// the instant FRR shows Full its TE LSA may not exist yet (measured
	// 2026-10-21: Ze's opaque-area empty 11s into the run).
	"ospf-ext-prefix-link-frr": {
		{kind: opWaitContains, peer: "ze", command: zeCommand("show ospf database opaque-area"), contains: []string{"extended-prefix", "extended-link"}, timeout: 60 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseOpaqueArea}, contains: []string{zeLabAddress}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospf-opaque-frr": {
		{kind: opWaitContains, peer: "ze", command: zeCommand("show ospf database opaque-area"), contains: []string{frrLabAddress}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospf-ri-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf database router-information"), contains: []string{zeLabAddress}},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseOpaqueArea}, contains: []string{zeLabAddress}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospf-sr-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf segment-routing"), contains: []string{"16000", zeLabAddress}},
		{kind: opWaitContains, peer: "ze", command: []string{cmdMPLS, "-ls"}, contains: []string{"16100"}, timeout: 60 * time.Second},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ip ospf database segment-routing"}, contains: []string{zeLabAddress}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospfv3-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6DatabaseRouter}, contains: []string{zeLabAddress}, timeout: 60 * time.Second},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospfv3-gr-frr": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospfv3-ptmp-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6DatabaseRouter}, contains: []string{zeLabAddress}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospfv3-broadcast-frr": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ipv6 ospf6 database network"}, contains: []string{zeLabAddress}},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ipv6 ospf6 database link"}, contains: []string{zeLabAddress}},
	},
	"ospfv3-nssa-redist-frr": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ipv6 ospf6 route"}, contains: []string{ospf6NSSAPrefix, "NSSA"}},
		// RFC 3101 Section 2.3: an AS-External-LSA is never flooded into an NSSA,
		// so FRR's AS-scoped database holds no LSA at all and its "ASE" type token
		// appears nowhere in the database listing. ze's redistributed prefix is the
		// proof, because FRR holds it only as the payload of the type-7 NSSA-LSA ze
		// originated over a working adjacency: an empty database, a dead session,
		// and a peer that received nothing each fail it, and none of them can pass
		// this absence by holding nothing. A type name spelled in the section
		// heading satisfies every one of those states, so it is not evidence that
		// the type-5 leak was avoided.
		{kind: opRequireAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Database}, absent: []string{"ASE"}, proof: []string{ospf6NSSAPrefix, zeLabAddress}},
	},
	"ospfv3-ri-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf database router-information"), contains: []string{zeLabAddress}},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ipv6 ospf6 database router-information"}, contains: []string{zeLabAddress}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospfv3-sr-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf ipv6 segment-routing"), contains: []string{"16000"}},
		{kind: opWaitContains, peer: "ze", command: []string{cmdMPLS, "-ls"}, contains: []string{"16100"}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	scenarioRPKIFRR: {
		{kind: opFRRRoute, argument: "9.43.0.0/24"},
		{kind: opFRRRoute, argument: "10.43.0.0/24"},
		{kind: opFRRRoute, argument: "11.43.0.0/24"},
		{kind: opWaitRPKI, peer: "ze", command: []string{cmdCat, "/tmp/rpki-check.json"}, timeout: 60 * time.Second},
	},
	scenarioShutdownCeaseFRR: {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdCat, frrLogPath}, contains: []string{"Cease", "Administrative Shutdown"}, timeout: 30 * time.Second},
	},
}

func relayWithdrawalExtras(prefix string) []operation {
	var evidence textbuf.Buffer
	update := evidence.Str("rcvd UPDATE about ").Str(prefix).String()
	return []operation{
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdCat, frrLogPath}, contains: []string{update, frrWithdrawnMarker}, timeout: 90 * time.Second},
		{kind: opRequireAbsent, peer: peerFRR, command: []string{cmdCat, frrLogPath}, absent: []string{"Missing well-known attribute", "rcvd UPDATE with errors in attr"}, proof: []string{update, frrWithdrawnMarker}},
	}
}

func zeCommand(command string) []string {
	return []string{"ze", zeCLICommand, "-c", command, "--user", "interop", "--format", "json"}
}
