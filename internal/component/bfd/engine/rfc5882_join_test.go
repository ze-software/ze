// VALIDATES: RFC 5882 sec 4.4 at the registry -- a request that leaves a key
// field unset joins the one session to the same remote system that it cannot be
// told apart from, in either arrival order, and never a session it can be told
// apart from.
// PREVENTS: a BGP peer with no local address and a pinned or OSPF session to
// the same system opening two sessions, and the opposite error, a request being
// merged onto a session to another system, another address pair (RFC 5883 sec
// 4.1) or another link-local zone (RFC 4007 sec 6).
package engine

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
)

// joinLoop is an unstarted loop: EnsureSession only registers sessions, which
// is all these tests read.
func joinLoop(mode api.HopMode) *Loop {
	tr, _ := transport.Pair(mode, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))
	return NewLoop(tr, clock.RealClock{})
}

func multiHop(peer, local string) api.SessionRequest {
	req := api.SessionRequest{Peer: netip.MustParseAddr(peer), VRF: api.DefaultVRF, Mode: api.MultiHop}
	if local != "" {
		req.Local = netip.MustParseAddr(local)
	}
	return req
}

func singleHop(peer, local, iface string) api.SessionRequest {
	req := api.SessionRequest{Peer: netip.MustParseAddr(peer), Interface: iface, VRF: api.DefaultVRF, Mode: api.SingleHop}
	if local != "" {
		req.Local = netip.MustParseAddr(local)
	}
	return req
}

// ensureAll registers every request in order and returns the loop's snapshot.
func ensureAll(t *testing.T, loop *Loop, reqs ...api.SessionRequest) []api.SessionState {
	t.Helper()
	for i := range reqs {
		if _, err := loop.EnsureSession(reqs[i]); err != nil {
			t.Fatalf("EnsureSession #%d (%+v): %v", i, reqs[i].Key(), err)
		}
	}
	return loop.Snapshot()
}

// refcounts answers each session's refcount, in snapshot order.
func refcounts(snap []api.SessionState) []int {
	out := make([]int, 0, len(snap))
	for i := range snap {
		out = append(out, snap[i].Refcount)
	}
	return out
}

// TestMultiHopUnpinnedJoinsThePinnedSessionInEitherOrder: a request with no
// local address and one naming 172.30.0.2, to one peer, share one session
// whichever arrives first.
func TestMultiHopUnpinnedJoinsThePinnedSessionInEitherOrder(t *testing.T) {
	pinned, unpinned := multiHop("203.0.113.9", "172.30.0.2"), multiHop("203.0.113.9", "")
	for name, order := range map[string][]api.SessionRequest{
		"pinned first":   {pinned, unpinned},
		"unpinned first": {unpinned, pinned},
	} {
		snap := ensureAll(t, joinLoop(api.MultiHop), order...)
		if len(snap) != 1 || snap[0].Refcount != 2 {
			t.Errorf("%s: refcounts %v, want one session at 2", name, refcounts(snap))
		}
	}
}

// TestMultiHopTwoLocalAddressesAreTwoSessions is RFC 5883 Section 4.1:
// "Multiple sessions between the same pair of systems must have at least one
// endpoint address distinct from one another." Two pinned local addresses are
// two address pairs, so two sessions, including when an unpinned request had
// opened the session the first one joined.
func TestMultiHopTwoLocalAddressesAreTwoSessions(t *testing.T) {
	snap := ensureAll(t, joinLoop(api.MultiHop),
		multiHop("203.0.113.9", ""),
		multiHop("203.0.113.9", "172.30.0.2"),
		multiHop("203.0.113.9", "10.1.0.2"),
	)
	if len(snap) != 2 {
		t.Fatalf("two local addresses to one peer: %d sessions, want 2 (refcounts %v)", len(snap), refcounts(snap))
	}
	if snap[0].Refcount+snap[1].Refcount != 3 {
		t.Errorf("refcounts %v, want the unpinned request on the 172.30.0.2 session", refcounts(snap))
	}
}

// TestMultiHopUnpinnedRequestWithTwoCandidatesGetsItsOwnSession: with two
// pinned address pairs to one peer, a request that names no local address
// could mean either, so it joins neither.
func TestMultiHopUnpinnedRequestWithTwoCandidatesGetsItsOwnSession(t *testing.T) {
	snap := ensureAll(t, joinLoop(api.MultiHop),
		multiHop("203.0.113.9", "172.30.0.2"),
		multiHop("203.0.113.9", "10.1.0.2"),
		multiHop("203.0.113.9", ""),
	)
	if len(snap) != 3 {
		t.Errorf("ambiguous unpinned request: %d sessions, want 3 (refcounts %v)", len(snap), refcounts(snap))
	}
}

// TestMultiHopRequestsToTwoPeersNeverJoin is the negative: an unpinned request
// is a wildcard on the local address only, never on the peer.
func TestMultiHopRequestsToTwoPeersNeverJoin(t *testing.T) {
	snap := ensureAll(t, joinLoop(api.MultiHop),
		multiHop("203.0.113.9", "172.30.0.2"),
		multiHop("203.0.113.8", ""),
	)
	if len(snap) != 2 {
		t.Errorf("two peers: %d sessions, want 2 (refcounts %v)", len(snap), refcounts(snap))
	}
}

// TestSingleHopRequestWithNoLinkJoinsTheSessionOnTheLink covers the single-hop
// configurations Canonical cannot complete for a global peer: two links on one
// subnet, a peer on no connected prefix, and no interface backend. OSPF names
// its link and address; a BGP peer names neither. A global address names one
// system in its VRF, so the two share, in either order.
func TestSingleHopRequestWithNoLinkJoinsTheSessionOnTheLink(t *testing.T) {
	ospf, bgp := singleHop("172.30.0.10", "172.30.0.2", "eth0"), singleHop("172.30.0.10", "", "")
	for name, order := range map[string][]api.SessionRequest{
		"ospf first": {ospf, bgp},
		"bgp first":  {bgp, ospf},
	} {
		snap := ensureAll(t, joinLoop(api.SingleHop), order...)
		if len(snap) != 1 || snap[0].Refcount != 2 {
			t.Errorf("%s: refcounts %v, want one session at 2", name, refcounts(snap))
		}
	}
}

// TestSingleHopTwoLinksStayTwoSessions: once a request that named no link has
// joined the eth0 session, a request on eth1 is on another link (RFC 5881
// Section 2), so it gets its own session rather than the joined one.
func TestSingleHopTwoLinksStayTwoSessions(t *testing.T) {
	snap := ensureAll(t, joinLoop(api.SingleHop),
		singleHop("172.30.0.10", "", ""),
		singleHop("172.30.0.10", "172.30.0.2", "eth0"),
		singleHop("172.30.0.10", "172.30.0.3", "eth1"),
	)
	if len(snap) != 2 {
		t.Errorf("two links: %d sessions, want 2 (refcounts %v)", len(snap), refcounts(snap))
	}
}

// TestSingleHopLinkLocalPeerWithNoLinkJoinsNothing is RFC 4007 Section 6: "the
// same non-global address may be in use in more than one zone of the same
// scope". fe80::1 on eth0 does not name the system a request for fe80::1 with
// no link means, so the request is not joined to it.
func TestSingleHopLinkLocalPeerWithNoLinkJoinsNothing(t *testing.T) {
	snap := ensureAll(t, joinLoop(api.SingleHop),
		singleHop("fe80::1", "fe80::2", "eth0"),
		singleHop("fe80::1", "", ""),
	)
	if len(snap) != 2 {
		t.Errorf("link-local peer with no link: %d sessions, want 2 (refcounts %v)", len(snap), refcounts(snap))
	}
}
