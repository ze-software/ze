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
	"bgp-policy-import-export-frr": {
		{kind: opRequireContains, peer: peerBIRD, command: []string{cmdBirdc, "show route for 10.39.1.0/24 all"}, contains: []string{"10.39.1.0/24", "BGP.local_pref: 250", "BGP.med: 77"}},
		{kind: opBIRDRouteAbsent, argument: "10.39.2.0/24", proof: []string{"BIRD"}},
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
		{kind: opRequireAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseNetwork}, absent: []string{zeLabAddress}, proof: []string{"Network Link States"}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseRouter}, contains: []string{zeLabAddress}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, delay: 5 * time.Second},
	},
	"isis-lan-dis-frr": {
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISDatabase}, contains: []string{".00-01"}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, contains: []string{"Up"}, delay: 5 * time.Second},
	},
	"ospf-broadcast-frr": {
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{"Full/", "DR"}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseNetwork}, contains: []string{"Network Link States"}, timeout: 60 * time.Second},
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
		{kind: opRequireAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ipv6 ospf6 database external"}, absent: []string{ospf6NSSAPrefix}, proof: []string{"AS-External"}},
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
