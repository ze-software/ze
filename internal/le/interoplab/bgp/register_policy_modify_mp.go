// Design: docs/architecture/testing/interop.md -- an import filter that rebuilds multiprotocol sections, judged by FRR.
// Related: test/interop/scenarios/bgp-policy-modify-mp-frr/inject.msg -- the UPDATEs, one per case.
package bgp

import "time"

// scenarioPolicyModifyMP is an import prefix-list that keeps some prefixes of
// one UPDATE and refuses the others, where the prefixes ride in
// MP_REACH_NLRI. The raw injector sends each case as ONE UPDATE, so Ze must
// rebuild the attribute rather than accept or drop the UPDATE whole. FRR
// judges what was installed, and the OPEN-only relay between Ze and FRR keeps
// the bytes Ze sent, which is where an extended length and an empty
// MP_REACH_NLRI are visible: FRR's table shows neither.
//
// The cases, in the order the injector sends them:
//
//	(1) IPv6: 2001:db8:1::/48 kept, 2001:db8:dead::/48 refused.
//	(a) IPv4 unicast inside MP_REACH_NLRI: 10.61.1.0/24 kept, 10.61.2.0/24 refused.
//	(d) IPv6, every prefix refused, community 65004:4242: nothing leaves Ze.
//	(c) IPv6, thirty /64s kept and 2001:db8:beef::/48 refused, community
//	    65004:4141: the rebuilt attribute is 291 octets, so extended length.
//	(b) MP_UNREACH_NLRI withdrawing 2001:db8:1::/48, sent only once the
//	    checker has seen FRR hold it and has made FRR announce the sentinel
//	    the injector waits for.
const (
	scenarioPolicyModifyMP = "bgp-policy-modify-mp-frr"

	policyModifyMPRelay = "172.30.0.11"

	policyModifyMPKeptV6           = "2001:db8:1::/48"
	policyModifyMPRefusedV6        = "2001:db8:dead::/48"
	policyModifyMPKeptV4           = "10.61.1.0/24"
	policyModifyMPRefusedV4        = "10.61.2.0/24"
	policyModifyMPLastLarge        = "2001:db8:100:1e::/64"
	policyModifyMPRefusedLarge     = "2001:db8:beef::/48"
	policyModifyMPAllRefusedFirst  = "2001:db8:bad:1::/64"
	policyModifyMPAllRefusedSecond = "2001:db8:bad:2::/64"

	// policyModifyMPCapture is the relay's record of every frame Ze sent FRR,
	// one JSON row per frame with the frame in upper-case hex.
	policyModifyMPCapture = "/tmp/policy-mp-ze.jsonl"
	// policyModifyMPLargeMarker is COMMUNITY 65004:4141, carried by case (c).
	policyModifyMPLargeMarker = "FDEC102D"
	// policyModifyMPRefusedMarker is COMMUNITY 65004:4242, carried only by
	// case (d), whose every prefix the filter refuses.
	policyModifyMPRefusedMarker = "FDEC1092"
	// policyModifyMPExtendedReach is MP_REACH_NLRI with the extended-length
	// flag (0x90) and length 291 (0x0123) for IPv6 unicast: the thirty kept
	// /64s of case (c) behind a 16-octet next hop.
	policyModifyMPExtendedReach = "900E0123000201"

	// policyModifyMPRouter, policyModifyMPFamilyV6 and policyModifyMPSentinel
	// are the vtysh path to FRR's second sentinel, the announcement the
	// injector waits for before it sends the withdrawal of case (b).
	policyModifyMPRouter   = frrRouterBGP65002
	policyModifyMPFamilyV6 = "address-family ipv6 unicast"
	policyModifyMPSentinel = "network 2001:db8:f01::/48"

	policyModifyMPShowV6 = "show bgp ipv6 unicast json"
	policyModifyMPShowV4 = "show bgp ipv4 unicast json"
	// policyModifyMPNeighbor is FRR's view of its one session, to Ze through
	// the relay.
	policyModifyMPNeighbor = "show bgp neighbor " + policyModifyMPRelay + " json"
)

// policyModifyMPEmptyReach lists every MP_REACH_NLRI header that carries no
// NLRI at all: IPv6 unicast behind a 16- or a 32-octet next hop and IPv4
// unicast behind a 4-octet one, in the short and the extended-length form.
// The length leaves no room for a prefix, so each is an empty attribute.
var policyModifyMPEmptyReach = []string{
	"800E1500020110", "900E001500020110",
	"800E2500020120", "900E002500020120",
	"800E0900010104", "900E000900010104",
}

// policyModifyMPKeptLarge is the thirty /64s case (c) keeps,
// 2001:db8:100:1::/64 to 2001:db8:100:1e::/64.
var policyModifyMPKeptLarge = []string{
	"2001:db8:100:1::/64", "2001:db8:100:2::/64", "2001:db8:100:3::/64",
	"2001:db8:100:4::/64", "2001:db8:100:5::/64", "2001:db8:100:6::/64",
	"2001:db8:100:7::/64", "2001:db8:100:8::/64", "2001:db8:100:9::/64",
	"2001:db8:100:a::/64", "2001:db8:100:b::/64", "2001:db8:100:c::/64",
	"2001:db8:100:d::/64", "2001:db8:100:e::/64", "2001:db8:100:f::/64",
	"2001:db8:100:10::/64", "2001:db8:100:11::/64", "2001:db8:100:12::/64",
	"2001:db8:100:13::/64", "2001:db8:100:14::/64", "2001:db8:100:15::/64",
	"2001:db8:100:16::/64", "2001:db8:100:17::/64", "2001:db8:100:18::/64",
	"2001:db8:100:19::/64", "2001:db8:100:1a::/64", "2001:db8:100:1b::/64",
	"2001:db8:100:1c::/64", "2001:db8:100:1d::/64", policyModifyMPLastLarge,
}

func init() {
	capture := []string{cmdCat, policyModifyMPCapture}
	showV6 := []string{cmdVtysh, "-c", policyModifyMPShowV6}
	showV4 := []string{cmdVtysh, "-c", policyModifyMPShowV4}
	keptV6 := append([]string{policyModifyMPKeptV6}, policyModifyMPKeptLarge...)
	scenarioOperations[scenarioPolicyModifyMP] = []operation{
		{kind: opFRRSession, argument: policyModifyMPRelay, timeout: 120 * time.Second},
		// The injector sends nothing until Ze has relayed FRR's first sentinel,
		// so these waits cover FRR's start as well as the relay.
		{kind: opFRRRoute, argument: policyModifyMPKeptV6, family: frrFamilyIPv6Unicast, timeout: 120 * time.Second},
		{kind: opFRRRoute, argument: policyModifyMPKeptV4, timeout: 60 * time.Second},
		{kind: opFRRRoute, argument: policyModifyMPLastLarge, family: frrFamilyIPv6Unicast, timeout: 60 * time.Second},
		// (1), (c) and (d) in one table: every kept IPv6 prefix is held, and
		// no refused one is. The kept prefixes are the proof, because they came
		// in the same UPDATEs as the refused ones.
		{kind: opRequireAbsent, peer: peerFRR, command: showV6,
			absent: []string{policyModifyMPRefusedV6, policyModifyMPRefusedLarge, policyModifyMPAllRefusedFirst, policyModifyMPAllRefusedSecond},
			proof:  keptV6},
		// (a): IPv4 unicast that arrived in MP_REACH_NLRI.
		{kind: opRequireAbsent, peer: peerFRR, command: showV4, absent: []string{policyModifyMPRefusedV4}, proof: []string{policyModifyMPKeptV4}},
		// (c) on the wire: the rebuilt attribute left Ze in the extended-length
		// form, in the UPDATE that carries case (c)'s community.
		{kind: opWaitContains, peer: peerSpeaker2, command: capture, contains: []string{policyModifyMPLargeMarker, policyModifyMPExtendedReach}, timeout: 30 * time.Second},
		// (d) on the wire: case (d) was sent before case (c), so with (c) in
		// the capture nothing carrying (d)'s community reached FRR, and no
		// frame Ze sent holds an MP_REACH_NLRI without NLRI.
		{kind: opRequireAbsent, peer: peerSpeaker2, command: capture,
			absent: append([]string{policyModifyMPRefusedMarker}, policyModifyMPEmptyReach...),
			proof:  []string{policyModifyMPLargeMarker}},
		// (b): FRR announces the second sentinel, Ze relays it to the injector,
		// and the injector answers with the MP_UNREACH_NLRI.
		{kind: opExec, peer: peerFRR, command: []string{cmdVtysh, "-c", frrConfigureTerminal, "-c", policyModifyMPRouter, "-c", policyModifyMPFamilyV6, "-c", policyModifyMPSentinel}},
		{kind: opWaitAbsent, peer: peerFRR, command: showV6, absent: []string{policyModifyMPKeptV6}, proof: policyModifyMPKeptLarge, timeout: 90 * time.Second},
		// The injector's session outlived every case: Ze would have withdrawn
		// its routes from FRR with it.
		{kind: opRequireAbsent, peer: peerFRR, command: showV4, absent: []string{policyModifyMPRefusedV4}, proof: []string{policyModifyMPKeptV4}},
		// FRR's session to Ze never reset: no UPDATE Ze rebuilt was one FRR
		// answered with a NOTIFICATION.
		{kind: opRequireJSONFields, peer: peerFRR, command: []string{cmdVtysh, "-c", policyModifyMPNeighbor},
			fields: map[string]string{"connectionsEstablished": "1", "connectionsDropped": "0"}},
		{kind: opFRRSession, argument: policyModifyMPRelay},
	}
}
