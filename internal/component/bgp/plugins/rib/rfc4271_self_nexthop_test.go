package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// selfAddr is this speaker's own end of every session in these tests: the
// address RFC 4271 Section 6.3 calls "the IP address of the receiving speaker".
const selfAddr = "198.51.100.1"

// selfNextHopRIB builds a manager with one established peer whose session-local
// end is selfAddr, so the speaker knows what "itself" means.
func selfNextHopRIB(t *testing.T, peers ...string) (*RIBManager, *locrib.RIB) {
	t.Helper()
	r := newTestRIBManager(t)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	for _, p := range peers {
		addr := netip.MustParseAddr(p)
		r.peerMeta[addr] = &peerMetadata{
			PeerASN:      65001,
			LocalASN:     65000,
			LocalAddress: netip.MustParseAddr(selfAddr),
		}
		r.bgpPeers[addr] = storage.NewPeerRIB(addr.String())
	}
	r.refreshSelfNextHopsLocked()
	return r, loc
}

// VALIDATES: a received route whose NEXT_HOP is this speaker's own address is
// not installed in the Loc-RIB, while a route naming any other next hop is.
//
// RFC requirement: RFC4271-5.1.3-2 positive -- "A BGP speaker SHALL NOT install a
// route with itself as the next hop" (Section 5.1.3). gatherCandidatesLocked
// (rib_commands.go) drops a candidate whose entryNextHopAddr is in selfNextHops,
// so checkBestPathChange has no winner to mirror into the Loc-RIB.
// RFC requirement: RFC4271-5.1.3-2 negative -- the exclusion is about the address,
// not about the peer or the prefix. The same peer, the same manager and a route
// pointing at a third party installs normally. That half is what makes the
// positive non-vacuous: a change that stopped installing routes at all passes the
// first assertion and fails this one.
// RFC requirement: RFC4271-6.3-2 positive -- Section 6.3 declares a NEXT_HOP that
// is "the IP address of the receiving speaker" semantically incorrect and says
// the route "SHOULD be ignored"; ignoring it here is that same act, and no
// NOTIFICATION is produced (RFC4271-6.3-3), because the exclusion happens in the
// decision process and never touches the session.
//
// PREVENTS: the local routing loop this gap left open. A route naming this
// speaker resolves, in this speaker's own FIB, to this speaker, so every packet
// matching the prefix is handed back to the router that just forwarded it.
func TestRFC4271SelfNextHopRouteIsNotInstalled(t *testing.T) {
	peer := "192.0.2.1"
	r, loc := selfNextHopRIB(t, peer)
	fam := family.Family{AFI: 1, SAFI: 1}
	peerAddr := netip.MustParseAddr(peer)

	self := ipv4Prefix(24, 10, 0, 0)
	r.bgpPeers[peerAddr].Insert(fam, makeAttrBytes(netip.MustParseAddr(selfAddr).As4()), self, true)
	_, ok := r.checkBestPathChange(fam, self, false, nil)
	assert.False(t, ok, "a route naming this speaker as its next hop makes no best path")
	_, found := loc.Best(fam, netip.MustParsePrefix("10.0.0.0/24"))
	assert.False(t, found,
		"RFC 4271 Section 5.1.3: a route with this speaker as the next hop must not be installed")

	// The control: the same peer, the same manager, a next hop that is not ours.
	third := ipv4Prefix(24, 10, 0, 1)
	r.bgpPeers[peerAddr].Insert(fam, makeAttrBytes([4]byte{192, 168, 1, 1}), third, true)
	_, ok = r.checkBestPathChange(fam, third, false, nil)
	require.True(t, ok)
	best, found := loc.Best(fam, netip.MustParsePrefix("10.0.1.0/24"))
	require.True(t, found, "a route pointing anywhere else is still installed")
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), best.NextHop)
}

// VALIDATES: the exclusion removes the broken route from the decision process
// rather than refusing the install, so a sound alternative to the same prefix
// still reaches the Loc-RIB.
//
// RFC requirement: RFC4271-5.1.3-2 negative -- excluding the candidate leaves
// SelectMultipath a runner-up to choose, which is what Section 9.1.2 already does
// with a next hop it cannot resolve. Refusing at install instead would blackhole
// the prefix outright.
//
// PREVENTS: the guard becoming worse than the gap. A prefix with two paths, one
// broken and one sound, must be reachable; a check that answered "do not install"
// after selection would leave it dark whenever the broken path happened to win.
func TestRFC4271SelfNextHopDoesNotShadowASoundAlternative(t *testing.T) {
	broken, sound := "192.0.2.1", "192.0.2.2"
	r, loc := selfNextHopRIB(t, broken, sound)
	fam := family.Family{AFI: 1, SAFI: 1}
	nlri := ipv4Prefix(24, 10, 0, 0)

	// The broken path wins on AS_PATH length if it is allowed to compete: both
	// carry the same attributes, so the tie would be broken by peer address and
	// 192.0.2.1 sorts first.
	r.bgpPeers[netip.MustParseAddr(broken)].Insert(fam,
		makeAttrBytes(netip.MustParseAddr(selfAddr).As4()), nlri, true)
	r.bgpPeers[netip.MustParseAddr(sound)].Insert(fam,
		makeAttrBytes([4]byte{192, 168, 1, 2}), nlri, true)

	_, ok := r.checkBestPathChange(fam, nlri, false, nil)
	require.True(t, ok, "the sound path is still a best path")
	best, found := loc.Best(fam, netip.MustParsePrefix("10.0.0.0/24"))
	require.True(t, found, "the prefix must not go dark because one path names this speaker")
	assert.Equal(t, netip.MustParseAddr("192.168.1.2"), best.NextHop,
		"the surviving candidate is the one that does not point at this speaker")
}

// VALIDATES: "itself" is derived from the local address the peer events already
// carry, and it tracks sessions coming up and going down.
//
// RFC requirement: RFC4271-5.1.3-2 positive -- refreshSelfNextHopsLocked
// (rib_self_nexthop.go) reads peerMetadata.LocalAddress, which updatePeerMetadata
// fills from PeerInfoJSON.Local.Address. There is no second list of this
// speaker's addresses to go stale.
// RFC requirement: RFC4271-5.1.3-2 negative -- an address this speaker never used
// for a session is NOT itself, so the set is a real answer rather than a test that
// matches everything.
//
// PREVENTS: the guard silently reverting to inert. If the event field were
// dropped the set would be empty, every route would pass, and the two tests above
// would still be green because they seed peerMeta by hand.
func TestRFC4271SelfNextHopSetComesFromPeerEvents(t *testing.T) {
	r := newTestRIBManager(t)
	peerAddr := netip.MustParseAddr("192.0.2.1")

	peer, err := json.Marshal(PeerInfoJSON{
		Remote: PeerRemoteInfo{Address: peerAddr.String(), AS: 65001},
		Local:  &PeerLocalInfo{Address: selfAddr, AS: 65000},
	})
	require.NoError(t, err)

	r.peerMu.Lock()
	r.updatePeerMetadata(&Event{Peer: peer}, peerAddr)
	r.peerMu.Unlock()

	require.NotNil(t, r.peerMeta[peerAddr])
	assert.Equal(t, netip.MustParseAddr(selfAddr), r.peerMeta[peerAddr].LocalAddress,
		"the local address the event carries is what the RIB stores")

	set := r.selfNextHops.Load()
	assert.True(t, isSelfNextHop(set, netip.MustParseAddr(selfAddr)),
		"the session's local end is one of this speaker's own addresses")
	assert.False(t, isSelfNextHop(set, netip.MustParseAddr("192.168.1.1")),
		"an address no session uses is not this speaker")
	assert.False(t, isSelfNextHop(set, netip.Addr{}),
		"a route with no next hop names no address, so it names none of ours")

	// The session goes away and takes its address with it.
	r.peerMu.Lock()
	delete(r.peerMeta, peerAddr)
	r.refreshSelfNextHopsLocked()
	r.peerMu.Unlock()
	assert.False(t, isSelfNextHop(r.selfNextHops.Load(), netip.MustParseAddr(selfAddr)),
		"an address this speaker no longer answers to is no longer itself")
}
