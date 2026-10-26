// Design: docs/guide/route-reflection.md -- the route server sends each client the elected route
// Related: session_ownership.go -- oweAnnouncement
// Related: rfc4271_rs_replacement_test.go -- rsReplacementSettled
// Related: rs_withdrawal_ownership_test.go -- ownershipLiveSend, ownershipLiveRead
package reactor

import (
	"net/netip"
	"testing"
)

// TestRFC7947RouteServerWorseReannouncementYieldsOtherSourceRoute proves the
// route server runs the decision process for a non-ADD-PATH client, not
// forward-all. Method: the real bgp-rs, bgp-rib and bgp-adj-rib-in over live
// sessions. A announces P (the best route), B announces P (second best), then
// A re-announces P with a route worse than B's. C's wire ends on an UPDATE
// carrying B's received attributes for P, with no withdrawal of P. Under
// forward-all, C would end on A's worse re-announcement, the newest arrival.
// C's history is judged only behind a fence: once C owes nothing, B announces
// a fresh marker, and C's receipt of it proves every write of the settled
// election reached C's reader first. An empty owed set alone does not: the
// election's write is flushed before its record is removed, but C's reader
// appends the frame afterwards, so a read on the empty set alone can miss it.
//
// VALIDATES: AC-22's re-announcement half (owner, Q-3; round 11, I-5).
// PREVENTS: forward-all, the newest arrival displacing a better route.
//
// RFC requirement: RFC7947-x-5 positive -- when the source of the route a non-ADD-PATH route-server client holds re-announces that prefix with a route worse than another source's route for the same prefix, the client's wire receives an UPDATE carrying the other source's received attributes.
//
// RFC 7947 Section 2.1: "the route server SHOULD perform one or more BGP
// Decision Processes to select routes for subsequent advertisement to its
// clients".
//
// MUTATION: make oweAnnouncement (session_ownership.go) answer false: C ends
// on A's worse re-announcement.
func TestRFC7947RouteServerWorseReannouncementYieldsOtherSourceRoute(t *testing.T) {
	peers := extendedRecoveryPeers(t, false, true)
	a, b, c := peers[0], peers[1], peers[2]
	p := netip.MustParsePrefix("203.0.113.0/24")
	markA := netip.MustParsePrefix("203.0.130.0/24")
	markB := netip.MustParsePrefix("203.0.131.0/24")
	markWorse := netip.MustParsePrefix("203.0.132.0/24")
	fenceHeld := netip.MustParsePrefix("203.0.133.0/24")
	fenceDecided := netip.MustParsePrefix("203.0.134.0/24")
	// RFC 4271 Section 9.1.2.2, step a: the shorter AS_PATH is preferred.
	best := firstASAttrs(4, 65002)                 // RFC 6793 Section 3.
	second := firstASAttrs(4, 65003, 65003)        // RFC 6793 Section 3.
	worse := firstASAttrs(4, 65002, 65002, 65002)  // RFC 6793 Section 3.
	markAttrsB := firstASAttrs(4, 65003)           // RFC 6793 Section 3.
	markAttrsA := firstASAttrs(4, 65002, 65002, 7) // RFC 6793 Section 3.

	ownershipLiveSend(t, a, nil, best, p, markA) // RFC 4271 Section 4.3.
	view := ownershipLiveThrough(t, c, markA, best)
	ownershipLiveRoute(t, view, p, best)

	ownershipLiveSend(t, b, nil, second, p)         // RFC 4271 Section 4.3.
	ownershipLiveSend(t, b, nil, markAttrsB, markB) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, markB, markAttrsB)
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C's election of P settled")
	ownershipLiveSend(t, b, nil, markAttrsB, fenceHeld) // RFC 4271 Section 4.3.
	ownershipLiveRoute(t, ownershipLiveThrough(t, c, fenceHeld, markAttrsB), p, best)

	ownershipLiveSend(t, a, nil, worse, p)              // RFC 4271 Section 4.3.
	ownershipLiveSend(t, a, nil, markAttrsA, markWorse) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, markWorse, markAttrsA)
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C's election after the worse route settled")
	ownershipLiveSend(t, b, nil, markAttrsB, fenceDecided) // RFC 4271 Section 4.3.
	view = ownershipLiveThrough(t, c, fenceDecided, markAttrsB)
	ownershipLiveRoute(t, view, p, second)
	ownershipLiveWithdrawals(t, view, p, 0)
	ownershipLiveEstablished(t, peers)
}

// TestRFC7947RouteServerWorseLaterRouteDoesNotDisplace proves the route server
// elects for a non-ADD-PATH client rather than forwarding the newest arrival.
// Method: the real bgp-rs, bgp-rib and bgp-adj-rib-in over live sessions. A
// announces P (the best) and C holds it. B then announces a worse route for P.
// C's TCP history, bounded by a fence once C owes nothing, carries A's
// announcement of P once and nothing else for P: neither B's route nor A's
// route sent again.
//
// VALIDATES: AC-22's later-arrival half (owner, Q-3).
// PREVENTS: forward-all, a worse later route displacing the held best; and a
// decision that elects the held route writing it a second time.
//
// RFC requirement: RFC7947-x-5 negative -- when a second source announces a worse route for a prefix a non-ADD-PATH route-server client already holds from the first source, the client's wire receives no UPDATE for that prefix
//
// RFC 7947 Section 2.1: "the route server SHOULD perform one or more BGP
// Decision Processes to select routes for subsequent advertisement to its
// clients".
//
// MUTATION: make oweAnnouncement (session_ownership.go) answer false: C
// receives B's worse route for P.
func TestRFC7947RouteServerWorseLaterRouteDoesNotDisplace(t *testing.T) {
	peers := extendedRecoveryPeers(t, false, true)
	a, b, c := peers[0], peers[1], peers[2]
	p := netip.MustParsePrefix("203.0.113.0/24")
	markA := netip.MustParsePrefix("203.0.180.0/24")
	markB := netip.MustParsePrefix("203.0.181.0/24")
	fenceHeld := netip.MustParsePrefix("203.0.182.0/24")
	fenceDecided := netip.MustParsePrefix("203.0.183.0/24")
	// RFC 4271 Section 9.1.2.2, step a: the shorter AS_PATH is preferred.
	best := firstASAttrs(4, 65002)                 // RFC 6793 Section 3.
	worse := firstASAttrs(4, 65003, 65003, 65003)  // RFC 6793 Section 3.
	markAttrsA := firstASAttrs(4, 65002, 65002, 7) // RFC 6793 Section 3.
	markAttrsB := firstASAttrs(4, 65003)           // RFC 6793 Section 3.

	ownershipLiveSend(t, a, nil, best, p) // RFC 4271 Section 4.3.
	view := rsReplacementSettled(t, a, b, c, markA, fenceHeld, markAttrsA, markAttrsB)
	ownershipLiveRoute(t, view, p, best)

	ownershipLiveSend(t, b, nil, worse, p) // RFC 4271 Section 4.3.
	view = rsReplacementSettled(t, b, b, c, markB, fenceDecided, markAttrsB, markAttrsB)
	ownershipLiveRoute(t, view, p, best)
	ownershipLiveAnnouncements(t, view, p, best)
	ownershipLiveWithdrawals(t, view, p, 0)
	ownershipLiveEstablished(t, peers)
}
