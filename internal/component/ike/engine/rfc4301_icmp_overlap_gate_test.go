// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the ICMP processing icmp-error-check turns on, transit traffic
// Related: icmp_overlap.go -- refuseICMPErrorBypass and icmpErrorSides, the refusal under test
// Related: spd_policy.go -- spdTransitPolicies, the producer of the OUT transit gate the refusal judges
// RFC: rfc/short/rfc4301.md -- Section 6.2, ICMP error processing

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// VALIDATES: refusal A covers the OUT transit gate of an `in` or `both` bypass (D-3).
// RFC 4301 Section 6.2: "If no SA exists that would carry the outbound ICMP message in
// question, and if no SPD entry would allow carriage of this outbound ICMP error message,
// then an IPsec implementation MUST map the message to the SA that would carry the return
// traffic associated with the packet that triggered the ICMP error message."
// spdTransitPolicies gives such a bypass a template-free OUT allow holding its in half's
// selectors, whose destination is the entry's LOCAL prefix. An error this node sends to
// that prefix meets the allow in icmp_route_lookup's first lookup as it meets an out
// bypass, so a peer with icmp-error-check true whose remote selectors overlap the local
// prefix would have the error mapped into its Child SA although the entry carries it in
// clear. K1 bit 7 changes nothing here: the OUT gate carries no XFRM_POLICY_ICMP (owner
// ruling 2026-11-20), and the first lookup reads no flag.
// METHOD: each refused entry is first checked to hold, in spdTransitPolicies' answer, an
// OUT gate to its local prefix, so the premise is the producer's and not this test's.
// The refusal must name the peer, the entry, the transit gate and a fix on the entry's
// local prefix (never its remote prefix, which the gate does not send to). The accepted
// entries are the cases just outside: a local prefix disjoint from the peer's remote
// selectors, an informational type, a protocol that is not ICMP, a discard, a peer with
// the check off, and a `vti bind` peer.
// PREVENTS: an `in` bypass whose transit gate Linux silently overrides with the Child SA,
// committed with no refusal; and the refusal reaching an `in` entry no error can meet.
func TestICMPOverlapBypassTransitGate(t *testing.T) {
	bypass, in, both := dataplane.SPActionBypass, ipsec.SPDDirIn, ipsec.SPDDirBoth
	const (
		localFix    = "set vpn ipsec policy icmp4 local prefix <prefix> with a prefix that does not overlap "
		protocolFix = "set vpn ipsec policy icmp4 protocol <protocol>"
		gate        = "the entry's transit gate"
	)
	refused := []struct {
		name   string
		peer   ipsec.SiteToSitePeer
		policy ipsec.SPDPolicy
		wants  []string
	}{
		{"icmp in to the peer's remote selectors", icmpOverlapPeer(t, false, false),
			icmpOverlapPolicy(t, "icmp4", bypass, in, 1, -1, "0.0.0.0/0", "192.0.2.0/24"),
			[]string{localFix + "10.2.0.0/24"}},
		{"any protocol in", icmpOverlapPeer(t, false, false),
			icmpOverlapPolicy(t, "icmp4", bypass, in, 0, -1, "10.2.0.128/25", "198.51.100.0/24"),
			[]string{localFix + "10.2.0.0/24", protocolFix}},
		// The out half sends to 203.0.113.0/24, outside the peer's remote selectors, so
		// only the gate can draw the refusal.
		{"destination unreachable both, remote prefix disjoint", icmpOverlapPeer(t, false, false),
			icmpOverlapPolicy(t, "icmp4", bypass, both, 1, 3, "10.2.0.0/24", "203.0.113.0/24"),
			[]string{localFix + "10.2.0.0/24"}},
		{"icmpv6 parameter problem in", icmpOverlapPeer(t, true, false),
			icmpOverlapPolicy(t, "icmp4", bypass, in, 58, 4, "2001:db8::/32", "::/0"),
			[]string{localFix + "2001:db8:2::/64"}},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			requireOutTransitGateTo(t, tc.policy)
			err := verifyICMPOverlap(icmpOverlapConfig(tc.peer, dataplane.SPActionDiscard, tc.policy))
			wants := append([]string{`ipsec peer "site"`, "`vpn ipsec policy icmp4`", gate,
				"set vpn ipsec site-to-site peer site icmp-error-check false"}, tc.wants...)
			requireICMPOverlapRefusal(t, err, wants...)
			requireICMPOverlapFixOmits(t, err, "icmp4 remote prefix")
		})
	}

	accepted := []struct {
		name   string
		peer   ipsec.SiteToSitePeer
		policy ipsec.SPDPolicy
	}{
		{"local prefix disjoint", icmpOverlapPeer(t, false, false),
			icmpOverlapPolicy(t, "icmp4", bypass, in, 1, -1, "10.3.0.0/16", "0.0.0.0/0")},
		{"echo request", icmpOverlapPeer(t, false, false),
			icmpOverlapPolicy(t, "icmp4", bypass, in, 1, 8, "0.0.0.0/0", "0.0.0.0/0")},
		{"tcp", icmpOverlapPeer(t, false, false),
			icmpOverlapPolicy(t, "icmp4", bypass, in, 6, -1, "0.0.0.0/0", "0.0.0.0/0")},
		{"discard", icmpOverlapPeer(t, false, false),
			icmpOverlapPolicy(t, "icmp4", dataplane.SPActionDiscard, in, 1, -1, "0.0.0.0/0", "0.0.0.0/0")},
		{"icmp-error-check false", icmpOverlapPeer(t, false, true),
			icmpOverlapPolicy(t, "icmp4", bypass, in, 1, -1, "0.0.0.0/0", "0.0.0.0/0")},
		{"vti bind", icmpOverlapVTIPeer(t),
			icmpOverlapPolicy(t, "icmp4", bypass, in, 1, -1, "0.0.0.0/0", "0.0.0.0/0")},
	}
	for _, tc := range accepted {
		t.Run("accepted "+tc.name, func(t *testing.T) {
			if err := verifyICMPOverlap(icmpOverlapConfig(tc.peer, dataplane.SPActionDiscard, tc.policy)); err != nil {
				t.Errorf("an in entry whose transit gate carries no ICMP error to the peer's remote selectors was refused: %v", err)
			}
		})
	}
}

// requireOutTransitGateTo fails unless spdTransitPolicies gives p, on a dataplane that
// gathers fragments and on one that does not, a template-free OUT gate whose destination
// is p's local prefix: the allow refusal A judges for an in or both bypass.
func requireOutTransitGateTo(t *testing.T, p ipsec.SPDPolicy) {
	t.Helper()
	for _, gathers := range []bool{true, false} {
		found := false
		gates := spdTransitPolicies(p, gathers).gates
		for i := range gates {
			gate := &gates[i]
			if gate.Dir == dataplane.SADirOut && gate.Action == dataplane.SPActionBypass && sameIPNet(gate.Dst, p.LocalPrefix) {
				found = true
			}
		}
		if !found {
			t.Fatalf("spdTransitPolicies (gathers=%v) gives %s no OUT gate to its local prefix %s, the premise of this refusal",
				gathers, p.Name, p.LocalPrefix)
		}
	}
}
