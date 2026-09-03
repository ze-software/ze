// Design: docs/architecture/testing/interop.md -- scenario-specific peer evidence.
// Related: checkers.go -- common session, route, and adjacency assertions.
package bgp

import (
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

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
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON}, contains: []string{"4200000000", "Established"}},
	},
	scenarioAddPathFRR: {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 unicast 10.10.0.0/24"}, contains: []string{injectPrefixFirst, "2 paths"}},
	},
	scenarioECMPFRR: {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 unicast 10.100.0.0/24 json"}, contains: []string{ecmpPrefix, zeLabAddress, gobgpLabAddress}},
	},
	scenarioEVPNFRR: {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON}, contains: []string{"l2VpnEvpn", frrCapabilityNegotiated}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp l2vpn evpn"}, contains: []string{"00:11:22:33:44:55", "00:11:22:33:44:66"}, timeout: 30 * time.Second},
	},
	scenarioEVPNGoBGP: {
		{kind: opWaitContains, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", "evpn", "-j"}, contains: []string{"00:11:22:33:44:55", "00:11:22:33:44:66"}, timeout: 30 * time.Second},
	},
	scenarioFlowspecFRR: {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON}, contains: []string{"ipv4Flowspec", frrCapabilityNegotiated}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 flowspec json"}, contains: []string{peerPrefixFirst, peerPrefixSecond}, timeout: 30 * time.Second},
	},
	scenarioFlowspecGoBGP: {
		{kind: opWaitContains, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", gobgpFamilyIPv4Flowspec, "-j"}, contains: []string{peerPrefixFirst, peerPrefixSecond}, timeout: 30 * time.Second},
		// GoBGP prints one bracketed term for each component it decoded, so this needle
		// says the OR of two AND groups arrived as ONE Type 4 component. Two Type 4
		// components print as two [port: ...] terms and fail here.
		{kind: opWaitContains, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", gobgpFamilyIPv4Flowspec}, contains: []string{flowspecOrOfAndRule}, timeout: 30 * time.Second},
	},
	scenarioGracefulRestartFRR: {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON}, contains: []string{"gracefulRestart", frrCapabilityNegotiated}},
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
	scenarioVPNFRR: {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON}, contains: []string{"ipv4Vpn", frrCapabilityNegotiated}},
	},
	scenarioVPNGoBGP: {
		{kind: opWaitContains, peer: peerGoBGP, command: []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", "vpnv4", "-j"}, contains: []string{peerPrefixFirst, peerPrefixSecond}, timeout: 30 * time.Second},
	},
	"bmp-frr": {
		{kind: opWaitContains, peer: peerBMP, command: []string{cmdCat, "/tmp/bmp-status.json"}, contains: []string{"initiation", "peer-up", "route-monitoring"}, timeout: 30 * time.Second},
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
	"ospf-p2p-frr": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ip ospf interface"}, contains: []string{"POINTOPOINT"}},
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
	"ospf-ext-prefix-link-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf database opaque-area"), contains: []string{"extended-prefix", "extended-link"}},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseOpaqueArea}, contains: []string{zeLabAddress}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospf-opaque-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf database opaque-area"), contains: []string{frrLabAddress}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospf-ri-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf database router-information"), contains: []string{zeLabAddress}},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseOpaqueArea}, contains: []string{zeLabAddress}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospf-sr-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf segment-routing"), contains: []string{"16000", zeLabAddress}},
		{kind: opWaitContains, peer: "ze", command: []string{"mpls", "-ls"}, contains: []string{"16100"}, timeout: 60 * time.Second},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ip ospf database segment-routing"}, contains: []string{zeLabAddress}},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"ospf-te-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf te-database"), contains: []string{frrLabAddress}},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseOpaqueArea}, contains: []string{zeLabAddress}},
	},
	"ospf-te-interas-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf database opaque-as"), contains: []string{"inter-as"}},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ip ospf database opaque-as"}, contains: []string{zeLabAddress}},
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
	"ospfv3-vlink-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ipv6 ospf6 interface"}, contains: []string{"VLINK"}, timeout: 90 * time.Second},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}},
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
		{kind: opWaitContains, peer: "ze", command: []string{"mpls", "-ls"}, contains: []string{"16100"}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPF6Neighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	scenarioRPKIFRR: {
		{kind: opWaitJSONFields, peer: "ze", command: []string{cmdCat, "/tmp/rpki-check.json"}, fields: map[string]string{fieldStatus: "ok", "9.43.0.0/24": "1", "11.43.0.0/24": "2"}, timeout: 60 * time.Second},
		{kind: opRequireAbsent, peer: "ze", command: []string{cmdCat, "/tmp/rpki-check.json"}, absent: []string{"\"10.43.0.0/24\""}, proof: []string{"\"9.43.0.0/24\"", "\"11.43.0.0/24\""}},
	},
	"rtr-stayrtr": {
		{kind: opWaitJSONFields, peer: "ze", command: zeCommand("show bgp rpki status"), minimum: map[string]int{"vrp-count-ipv4": 2, "vrp-count-ipv6": 2}, timeout: 90 * time.Second},
		{kind: opRequireContains, peer: "ze", command: zeCommand("show bgp rpki roa"), contains: []string{"9.58.0.0/16", "10.58.0.0/16", "2001:db8:58::/48", "2001:db8:59::/48", "4200000001", "65001"}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 9.58.0.0/16 4200000001"), fields: map[string]string{fieldState: rpkiStateValid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 9.58.0.0/24 4200000001"), fields: map[string]string{fieldState: rpkiStateValid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 9.58.0.0/25 4200000001"), fields: map[string]string{fieldState: rpkiStateInvalid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 9.58.0.0/16 65001"), fields: map[string]string{fieldState: rpkiStateInvalid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 10.58.0.0/16 65001"), fields: map[string]string{fieldState: rpkiStateValid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 10.58.0.0/24 65001"), fields: map[string]string{fieldState: rpkiStateInvalid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 2001:db8:58::/48 4200000001"), fields: map[string]string{fieldState: rpkiStateValid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 2001:db8:58::/64 4200000001"), fields: map[string]string{fieldState: rpkiStateInvalid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 11.58.0.0/16 65001"), fields: map[string]string{fieldState: rpkiStateNotFound}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 2001:db8:5a::/48 65001"), fields: map[string]string{fieldState: rpkiStateNotFound}},
	},
	scenarioShutdownCeaseFRR: {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdCat, frrLogPath}, contains: []string{"Cease", "Administrative Shutdown"}, timeout: 30 * time.Second},
	},
}

func relayWithdrawalExtras(prefix string) []operation {
	var evidence textbuf.Buffer
	update := evidence.Str("rcvd UPDATE about ").Str(prefix).String()
	return []operation{
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdCat, frrLogPath}, contains: []string{update, "withdrawn"}, timeout: 90 * time.Second},
		{kind: opRequireAbsent, peer: peerFRR, command: []string{cmdCat, frrLogPath}, absent: []string{"Missing well-known attribute", "rcvd UPDATE with errors in attr"}, proof: []string{update, "withdrawn"}},
	}
}

func zeCommand(command string) []string {
	return []string{"ze", zeCLICommand, "-c", command, "--user", "interop", "--format", "json"}
}
