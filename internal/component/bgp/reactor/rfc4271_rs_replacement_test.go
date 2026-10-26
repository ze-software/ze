// Design: docs/guide/route-reflection.md -- the route server sends each client the elected route
// Related: session_ownership.go -- owesWithdrawal, oweAnnouncement
// Related: relay_recovery.go -- queueRecoveryOutcome, queueRecoveryWithdrawal
// Related: rs_withdrawal_ownership_test.go -- ownershipLiveSend, ownershipLiveRead
package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// rsReplacementSettled waits until the source's marker reached C, then until C
// owes nothing, then sends fence from B and waits for it at C. The marker
// follows the source's previous UPDATE on one session, so its receipt proves
// that UPDATE was written or owed at C first. An empty owed set alone does not
// bound C's history: a decision's write is flushed before its record is
// removed, and C's reader appends the frame afterwards. B's fresh fence, read
// at C, proves every write of the settled decision reached C's reader first.
func rsReplacementSettled(t *testing.T, source, b, c *lowLivePeer, marker, fence netip.Prefix, markerAttrs, fenceAttrs []byte) ownershipLiveProjection {
	t.Helper()
	ownershipLiveSend(t, source, nil, markerAttrs, marker) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, marker, markerAttrs)
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C's decision of "+marker.String()+" settled")
	ownershipLiveSend(t, b, nil, fenceAttrs, fence) // RFC 4271 Section 4.3.
	return ownershipLiveThrough(t, c, fence, fenceAttrs)
}

// TestRFC4271RouteServerOwnerWithdrawalSendsReplacement proves that the
// withdrawal of the route a non-ADD-PATH route-server client holds reaches
// that client as the replacement route, not as a withdrawal. Method: the real
// bgp-rs, bgp-rib and bgp-adj-rib-in over live sessions. B announces P, then A
// announces P with a better route, so C holds A's route. A withdraws P. C's
// TCP history, bounded by a fence once C owes nothing, ends on B's received
// attributes for P and never carries a withdrawal of P.
//
// VALIDATES: AC-1's wire half, B then A (owner, Q-4).
// PREVENTS: the owner's bare withdrawal reaching C while B's route is retained.
//
// RFC requirement: RFC4271-9.2-7 negative -- when the source of the route last sent to a non-ADD-PATH route-server client withdraws it and another source's route for the same prefix is retained, the client's wire receives an UPDATE carrying that prefix with the retained route's attributes and no withdrawal of the prefix
//
// RFC 4271 Section 9.2: "All newly installed routes and all
// newly unfeasible routes for which there is no replacement route SHALL
// be advertised to its peers by means of an UPDATE message."
//
// MUTATION: make owesWithdrawal (session_ownership.go) answer false: C
// receives A's bare withdrawal of P and never B's route.
func TestRFC4271RouteServerOwnerWithdrawalSendsReplacement(t *testing.T) {
	peers := extendedRecoveryPeers(t, false, true)
	a, b, c := peers[0], peers[1], peers[2]
	p := netip.MustParsePrefix("203.0.113.0/24")
	markB := netip.MustParsePrefix("203.0.140.0/24")
	markA := netip.MustParsePrefix("203.0.141.0/24")
	markWithdrawn := netip.MustParsePrefix("203.0.142.0/24")
	fenceHeld := netip.MustParsePrefix("203.0.143.0/24")
	fenceDecided := netip.MustParsePrefix("203.0.144.0/24")
	// RFC 4271 Section 9.1.2.2, step a: the shorter AS_PATH is preferred.
	best := firstASAttrs(4, 65002)                 // RFC 6793 Section 3.
	retained := firstASAttrs(4, 65003, 65003)      // RFC 6793 Section 3.
	markAttrsB := firstASAttrs(4, 65003)           // RFC 6793 Section 3.
	markAttrsA := firstASAttrs(4, 65002, 65002, 7) // RFC 6793 Section 3.

	ownershipLiveSend(t, b, nil, retained, p, markB) // RFC 4271 Section 4.3.
	ownershipLiveRoute(t, ownershipLiveThrough(t, c, markB, retained), p, retained)
	ownershipLiveSend(t, a, nil, best, p) // RFC 4271 Section 4.3.
	view := rsReplacementSettled(t, a, b, c, markA, fenceHeld, markAttrsA, markAttrsB)
	ownershipLiveRoute(t, view, p, best)

	ownershipLiveSend(t, a, []netip.Prefix{p}, nil) // RFC 4271 Section 4.3.
	view = rsReplacementSettled(t, a, b, c, markWithdrawn, fenceDecided, markAttrsA, markAttrsB)
	ownershipLiveRoute(t, view, p, retained)
	ownershipLiveWithdrawals(t, view, p, 0)
	ownershipLiveEstablished(t, peers)
}

// TestRFC4271RouteServerLastSourceWithdrawalIsSent proves that the withdrawal
// of a prefix nobody else announces reaches a non-ADD-PATH route-server client,
// exactly once. Method: the real bgp-rs, bgp-rib and bgp-adj-rib-in over live
// sessions. A alone announces P and C holds it. A withdraws P. C's TCP
// history, bounded by a fence once C owes nothing, no longer holds P and
// carries exactly one withdrawal of it.
//
// VALIDATES: AC-2 (owner, Q-4).
// PREVENTS: the owed withdrawal being decided and never written.
//
// RFC requirement: RFC4271-9.2-7 positive -- when the only source of a prefix withdraws it, the route-server client's wire receives an UPDATE withdrawing that prefix
//
// RFC 4271 Section 9.2: "All newly installed routes and all
// newly unfeasible routes for which there is no replacement route SHALL
// be advertised to its peers by means of an UPDATE message."
//
// MUTATION: halt queueRecoveryWithdrawal (relay_recovery.go), which writes the
// RIB's WITHDRAW decision: C never receives the withdrawal of P.
func TestRFC4271RouteServerLastSourceWithdrawalIsSent(t *testing.T) {
	peers := extendedRecoveryPeers(t, false, true)
	a, b, c := peers[0], peers[1], peers[2]
	p := netip.MustParsePrefix("203.0.113.0/24")
	markA := netip.MustParsePrefix("203.0.150.0/24")
	markWithdrawn := netip.MustParsePrefix("203.0.151.0/24")
	fenceHeld := netip.MustParsePrefix("203.0.152.0/24")
	fenceDecided := netip.MustParsePrefix("203.0.153.0/24")
	only := firstASAttrs(4, 65002)                 // RFC 6793 Section 3.
	markAttrsA := firstASAttrs(4, 65002, 65002, 7) // RFC 6793 Section 3.
	markAttrsB := firstASAttrs(4, 65003)           // RFC 6793 Section 3.

	ownershipLiveSend(t, a, nil, only, p) // RFC 4271 Section 4.3.
	view := rsReplacementSettled(t, a, b, c, markA, fenceHeld, markAttrsA, markAttrsB)
	ownershipLiveRoute(t, view, p, only)

	ownershipLiveSend(t, a, []netip.Prefix{p}, nil) // RFC 4271 Section 4.3.
	view = rsReplacementSettled(t, a, b, c, markWithdrawn, fenceDecided, markAttrsA, markAttrsB)
	ownershipLiveAbsent(t, view, p)
	ownershipLiveWithdrawals(t, view, p, 1)
	ownershipLiveAnnouncements(t, view, p, only)
	ownershipLiveEstablished(t, peers)
}

// TestRFC4271RouteServerStillFeasibleRouteIsNotWithdrawn proves that the
// withdrawal of a route that was never the best leaves the best route in
// place, on the client's wire and in bgp-rib's Loc-RIB. Method: the real
// bgp-rs, bgp-rib and bgp-adj-rib-in over live sessions, each source with its
// own NEXT_HOP. A announces P (the best) and C holds it. B announces a worse
// route for P, then withdraws it. C's TCP history, bounded by a fence after
// each step once C owes nothing, carries A's announcement of P once and no
// withdrawal of P. The shared Loc-RIB, read after bgp-rib stored a marker B
// sent behind its withdrawal, still holds P with A's NEXT_HOP.
//
// VALIDATES: the 9.2-8 negative half moved from TestRIBBestChangeNoPublishSameBest (owner, Q-4).
// PREVENTS: a withdrawal of a non-best path withdrawing, re-announcing or
// un-installing the still-feasible best.
//
// RFC requirement: RFC4271-9.2-8 negative -- when another source's worse route for a prefix is announced and then withdrawn while a non-ADD-PATH route-server client holds the still-feasible best route, the client's wire receives neither a withdrawal nor a re-announcement of that prefix, and bgp-rib's Loc-RIB still holds the still-feasible route
//
// RFC 4271 Section 9.2: "Any routes in the Loc-RIB marked as unfeasible SHALL be
// removed."
//
// MUTATION: make oweAnnouncement (session_ownership.go) answer false: C
// receives B's worse route for P. Make checkRouteBestChange
// (plugins/rib/rib_bestchange.go) take its no-candidate branch, the one that
// removes the Loc-RIB entry, while a candidate remains: the Loc-RIB does not
// hold A's route for P.
func TestRFC4271RouteServerStillFeasibleRouteIsNotWithdrawn(t *testing.T) {
	peers := extendedRecoveryPeers(t, false, true)
	a, b, c := peers[0], peers[1], peers[2]
	p := netip.MustParsePrefix("203.0.170.0/24")
	markA := netip.MustParsePrefix("203.0.160.0/24")
	markB := netip.MustParsePrefix("203.0.161.0/24")
	markWithdrawn := netip.MustParsePrefix("203.0.162.0/24")
	fenceHeld := netip.MustParsePrefix("203.0.163.0/24")
	fenceWorse := netip.MustParsePrefix("203.0.164.0/24")
	fenceDecided := netip.MustParsePrefix("203.0.165.0/24")
	nextHopA := netip.MustParseAddr("192.0.2.1")
	nextHopB := netip.MustParseAddr("192.0.2.2")
	// RFC 4271 Section 9.1.2.2, step a: the shorter AS_PATH is preferred.
	best := synthesizedNextHopAttrs(nextHopA, 65002)                // RFC 6793 Section 3.
	worse := synthesizedNextHopAttrs(nextHopB, 65003, 65003, 65003) // RFC 6793 Section 3.
	markAttrsA := firstASAttrs(4, 65002, 65002, 7)                  // RFC 6793 Section 3.
	markAttrsB := firstASAttrs(4, 65003)                            // RFC 6793 Section 3.
	loc := locrib.Default()

	ownershipLiveSend(t, a, nil, best, p) // RFC 4271 Section 4.3.
	view := rsReplacementSettled(t, a, b, c, markA, fenceHeld, markAttrsA, markAttrsB)
	ownershipLiveRoute(t, view, p, best)

	ownershipLiveSend(t, b, nil, worse, p) // RFC 4271 Section 4.3.
	rsReplacementSettled(t, b, b, c, markB, fenceWorse, markAttrsB, markAttrsB)

	ownershipLiveSend(t, b, []netip.Prefix{p}, nil) // RFC 4271 Section 4.3.
	view = rsReplacementSettled(t, b, b, c, markWithdrawn, fenceDecided, markAttrsB, markAttrsB)
	ownershipLiveRoute(t, view, p, best)
	ownershipLiveAnnouncements(t, view, p, best)
	ownershipLiveWithdrawals(t, view, p, 0)

	// bgp-rib handles one source's UPDATEs in order, so its Adj-RIB-In holding
	// B's marker proves it ran B's withdrawal of P, Loc-RIB step included,
	// first. The fence reads the Adj-RIB-In, never the Loc-RIB it judges.
	lowEventually(t, func() bool {
		return extendedRecoveryStoredAttributes("192.0.2.2", []byte{203, 0, 162}) != nil
	}, "bgp-rib stored B's marker sent behind its withdrawal")
	path, installed := loc.Best(family.IPv4Unicast, p)
	if !installed {
		t.Fatalf("Loc-RIB lost the still-feasible %s after B withdrew a route that was never the best", p)
	}
	if path.NextHop != nextHopA {
		t.Errorf("Loc-RIB best for %s has NEXT_HOP %s, want A's %s", p, path.NextHop, nextHopA)
	}
	ownershipLiveEstablished(t, peers)
}
