package rib

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC9117AuthorizationUsesLongestCoveringRoute drives received UPDATEs, not
// the authorization helper. Each pair has a /8 and /16 covering a /24 rule.
// Internal cases isolate originator selection; external cases hold the rule's
// originator equal to the /16 and vary only the leftmost AS.
// MUTATION: selecting the shortest covering unicast prefix reverses the internal
// cases; comparing the external AS against the /8 refuses the valid external case.
// RFC requirement: RFC8955-6-1 positive -- a received rule matching the longest of two covering unicast routes becomes eligible, a candidate and an installed rule.
// RFC requirement: RFC8955-6-1 negative -- matching only the shorter covering route cannot authorize a received rule.
// RFC requirement: RFC9117-4.1-1 positive -- a non-local-domain rule matching the /16 originator is installed despite the /8 having a different originator.
// RFC requirement: RFC9117-4.1-1 negative -- a non-local-domain rule matching only the /8 originator is retained but has no candidate or install.
// RFC requirement: RFC9117-4.2-1 positive -- an external rule with the /16 originator and leftmost AS is installed despite the /8 having a different originator and AS.
// RFC requirement: RFC9117-4.2-1 negative -- an external rule with the /16 originator but only the /8 leftmost AS is retained without a candidate or install.
func TestRFC9117AuthorizationUsesLongestCoveringRoute(t *testing.T) {
	shortPeer := netip.MustParseAddr("192.0.2.1")
	longPeer := netip.MustParseAddr("192.0.2.2")
	flowPeer := netip.MustParseAddr("192.0.2.3")
	shortOrigin := netip.MustParseAddr("203.0.113.1")
	longOrigin := netip.MustParseAddr("203.0.113.2")
	for _, tc := range []struct {
		name       string
		peerAS     uint32
		firstAS    uint32
		originator netip.Addr
		want       bool
	}{
		{"internal longest originator", 65000, 65002, longOrigin, true},
		{"internal shorter originator", 65000, 65002, shortOrigin, false},
		{"external longest AS", 65100, 65002, longOrigin, true},
		{"external shorter AS", 65100, 65001, longOrigin, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, bus := flowValidationFixture(t)
			flowValidationReceive(t, r, shortPeer, 65001, 1, family.IPv4Unicast,
				[]byte{8, 10}, flowValidationAttrs(flowValidationPath(2, 65001), shortOrigin, 0, nil), false)
			flowValidationReceive(t, r, longPeer, 65002, 2, family.IPv4Unicast,
				[]byte{16, 10, 1}, flowValidationAttrs(flowValidationPath(2, 65002), longOrigin, 0, nil), false)
			raw := flowspecNLRI(24, 10, 1, 1)
			drop := []byte{0x80, 6, 0, 0, 0, 0, 0, 0}
			flowValidationReceive(t, r, flowPeer, tc.peerAS, 3, flowspecFamily, raw,
				flowValidationAttrs(flowValidationPath(2, tc.firstAS), tc.originator, 0, drop), false)
			key := ribevents.ValidationRoute{Peer: flowPeer, Family: flowspecFamily, NLRI: string(raw)}
			if got := ribevents.RouteEligible(key, 3); got != tc.want {
				t.Fatalf("eligibility = %v, want %v", got, tc.want)
			}
			if !ribevents.RoutePresent(key) {
				t.Fatal("received rule was not retained")
			}
			if got := len(gatherCandidatesHeld(r, flowspecFamily, raw, false)); got != map[bool]int{false: 0, true: 1}[tc.want] {
				t.Fatalf("candidate count = %d, want eligible %v", got, tc.want)
			}
			events := flowValidationEvents(bus)
			if !tc.want {
				if len(events) != 0 {
					t.Fatalf("unauthorized rule installed: %+v", events)
				}
				return
			}
			if len(events) != 1 || events[0].Withdraw || !bytes.Equal(events[0].NLRI, raw) || !bytes.Equal(events[0].ExtendedCommunities, drop) {
				t.Fatalf("selected event = %+v", events)
			}
		})
	}
}
