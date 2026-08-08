package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/network"
)

// TestRefreshPeerLinkScopesSkipsAnUnchangedTable pins the burst guard in
// refreshPeerLinkScopes.
//
// An interface that comes up delivers one addr-added event per address, and the
// kernel already holds every address of the burst when the first event arrives.
// Events 2..N therefore read a table identical to the one each peer's scope was
// built from. RFC 2545 Section 3's condition is decided by that table alone, so an
// equal table decides it equally, and the rebuild is a linkScope plus a
// forwarding-facts snapshot per peer per event for no change of answer.
//
// The snapshot POINTER is the observable: refreshForwardFactsIfLiveFrom stores a
// freshly built one on every call, so an unchanged pointer is proof the rebuild
// did not happen.
//
// VALIDATES: a peer whose scope already holds the current table keeps its exact
// snapshot; a peer whose scope holds a different table gets a new one; a peer that
// has read no table at all (nil scope) gets one.
// PREVENTS: the guard being widened into an always-skip, which would leave a
// surviving session answering Section 3 against a table that has moved.
func TestRefreshPeerLinkScopesSkipsAnUnchangedTable(t *testing.T) {
	current := network.ConnectedPrefixes()
	require.NotEmpty(t, current, "prerequisite: the test host has at least one connected prefix")

	newLivePeer := func(t *testing.T, r *Reactor, addr string, scope *linkScope) *Peer {
		t.Helper()
		s := NewPeerSettings(netip.MustParseAddr(addr), 65000, 65000, 0x01010101)
		s.LocalAddress = netip.MustParseAddr(addr)
		s.LinkLocal = netip.MustParseAddr("fe80::1")
		s.NextHopMode = NextHopSelf
		peer := NewPeer(s)
		r.peers[s.PeerKey()] = peer
		peer.llScope.Store(scope)
		peer.fwdFacts.Store(peer.buildForwardFacts())
		return peer
	}

	r := newIfaceTestReactor(t)

	// Its scope was built from the table refreshPeerLinkScopes is about to read.
	unchanged := newLivePeer(t, r, "::1", newLinkScopeFrom(current, netip.MustParseAddr("::1")))
	// Its scope was built from a table that no longer describes this host.
	stale := newLivePeer(t, r, "::2", newLinkScopeFrom([]netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}, netip.MustParseAddr("::2")))
	// It has read no table at all, which is not the same claim as "read an empty
	// one", so it must rebuild rather than compare equal to anything.
	never := newLivePeer(t, r, "::3", nil)

	before := map[string]*peerForwardFacts{
		"unchanged": unchanged.forwardFacts(),
		"stale":     stale.forwardFacts(),
		"never":     never.forwardFacts(),
	}

	r.refreshPeerLinkScopes()

	assert.Same(t, before["unchanged"], unchanged.forwardFacts(), "an identical table must not rebuild")
	assert.NotSame(t, before["stale"], stale.forwardFacts(), "a table that moved must rebuild")
	assert.NotSame(t, before["never"], never.forwardFacts(), "a peer that read no table must rebuild")
	assert.NotNil(t, never.llScope.Load(), "the rebuild settles the scope it lacked")
}
