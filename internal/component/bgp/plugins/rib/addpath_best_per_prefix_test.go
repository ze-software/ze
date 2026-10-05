// Design: docs/architecture/plugin/rib-storage-design.md -- best-path candidates
// Related: rfc8277_addpath_comparable_test.go -- the labeled-unicast case of the same election

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// unicastAttrs builds IPv4 unicast path attributes: ORIGIN IGP, an empty
// AS_PATH, NEXT_HOP nh, MED med and LOCAL_PREF localPref.
func unicastAttrs(nh [4]byte, med, localPref uint32) []byte {
	return []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, nh[0], nh[1], nh[2], nh[3], // NEXT_HOP
		0x80, 0x04, 0x04, byte(med >> 24), byte(med >> 16), byte(med >> 8), byte(med), // MED
		0x40, 0x05, 0x04, byte(localPref >> 24), byte(localPref >> 16), byte(localPref >> 8), byte(localPref), // LOCAL_PREF
	}
}

// addPathKey is the ADD-PATH wire key of 10.0.0.0/8 under pathID.
func addPathKey(pathID uint32) []byte {
	return []byte{byte(pathID >> 24), byte(pathID >> 16), byte(pathID >> 8), byte(pathID), 8, 10}
}

// newAddPathPeer registers an iBGP peer whose IPv4 unicast family is stored
// with ADD-PATH.
func newAddPathPeer(r *RIBManager, peer netip.Addr) *storage.PeerRIB {
	r.peerMeta[peer] = &peerMetadata{PeerASN: 65000, LocalASN: 65000}
	rib := storage.NewPeerRIB(peer.String())
	rib.SetAddPath(famV4, true)
	r.bgpPeers[peer] = rib
	return rib
}

// TestAddPathUnicastOneElectionPerPrefix checks that two paths of one prefix
// on one ADD-PATH session are elected once, for plain IPv4 unicast.
//
// VALIDATES: AC-2: path 7 (MED 10) and path 9 (MED 20) are both candidates of
// one selection for 10.0.0.0/8, one best record exists for the prefix, and
// the best-change names path 7 whichever UPDATE triggered the election.
// PREVENTS: one best per path identifier, the label-free twin of RFC 8277
// Section 3.1.
func TestAddPathUnicastOneElectionPerPrefix(t *testing.T) {
	r := newTestRIBManagerWithBus(newTestEventBus())
	peer := netip.MustParseAddr("192.0.2.31")
	rib := newAddPathPeer(r, peer)

	rib.Insert(famV4, unicastAttrs([4]byte{10, 0, 0, 1}, 10, 100), addPathKey(7))
	first, ok := r.checkBestPathChange(famV4, addPathKey(7), true, nil)
	require.True(t, ok)
	assert.Equal(t, uint32(7), first.PathID)

	rib.Insert(famV4, unicastAttrs([4]byte{10, 0, 0, 2}, 20, 100), addPathKey(9))
	_, ok = r.checkBestPathChange(famV4, addPathKey(9), true, nil)
	assert.False(t, ok, "path 9 (MED 20) does not displace path 7 (MED 10)")

	for _, key := range [][]byte{addPathKey(7), addPathKey(9)} {
		assert.Len(t, gatherCandidatesHeld(t, r, famV4, key, true), 2, "both paths are candidates (key %x)", key)
	}
	total := 0
	for _, d := range r.bestPrev.shardDepth(famV4) {
		total += d
	}
	assert.Equal(t, 1, total, "one best record for the prefix")
}

// TestAddPathMixedModeKeyNeverReadsAsAnotherPrefix checks that a peer stored
// without ADD-PATH is asked by prefix, never with an ADD-PATH wire key.
//
// VALIDATES: AC-8: a non-ADD-PATH peer holding 0.0.0.0/0 contributes no
// candidate to the election of an ADD-PATH peer's 10.0.0.0/8 path 7.
// PREVENTS: the four path-id octets of the key parsing as a zero-length
// prefix in the non-ADD-PATH peer, which made its default route a candidate.
func TestAddPathMixedModeKeyNeverReadsAsAnotherPrefix(t *testing.T) {
	r := newTestRIBManagerWithBus(newTestEventBus())
	addPathPeer := netip.MustParseAddr("192.0.2.31")
	plainPeer := netip.MustParseAddr("192.0.2.1")
	newAddPathPeer(r, addPathPeer).Insert(famV4, unicastAttrs([4]byte{10, 0, 0, 1}, 10, 100), addPathKey(7))
	r.peerMeta[plainPeer] = &peerMetadata{PeerASN: 65000, LocalASN: 65000}
	r.bgpPeers[plainPeer] = storage.NewPeerRIB(plainPeer.String())
	r.bgpPeers[plainPeer].Insert(famV4, unicastAttrs([4]byte{10, 0, 0, 9}, 0, 500), []byte{0})

	candidates := gatherCandidatesHeld(t, r, famV4, addPathKey(7), true)
	require.Len(t, candidates, 1, "only the ADD-PATH peer holds 10.0.0.0/8")
	assert.Equal(t, addPathPeer, candidates[0].PeerIP)

	best, ok := r.checkBestPathChange(famV4, addPathKey(7), true, nil)
	require.True(t, ok)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), best.NextHop, "the 0/0 route never wins 10/8")
}

// TestAddPathLocRIBHoldsOneBGPPath checks the Loc-RIB mirror of an ADD-PATH
// election: one BGP path per prefix, the RFC 4271 winner.
//
// VALIDATES: AC-9: path 7 (LOCAL_PREF 200, MED 50) beats path 9 (LOCAL_PREF
// 100, MED 10) on LOCAL_PREF, and the Loc-RIB holds exactly one BGP path for
// 10.0.0.0/8, carrying path 7's next hop.
// PREVENTS: a Loc-RIB path per path identifier, which locrib.selectBest would
// rank on distance and MED alone and so install path 9.
func TestAddPathLocRIBHoldsOneBGPPath(t *testing.T) {
	r := newTestRIBManagerWithBus(newTestEventBus())
	r.locRIB.Store(locrib.NewRIB())
	rib := newAddPathPeer(r, netip.MustParseAddr("192.0.2.31"))
	pfx := netip.MustParsePrefix("10.0.0.0/8")

	rib.Insert(famV4, unicastAttrs([4]byte{10, 0, 0, 9}, 10, 100), addPathKey(9))
	r.checkBestPathChange(famV4, addPathKey(9), true, nil)
	rib.Insert(famV4, unicastAttrs([4]byte{10, 0, 0, 7}, 50, 200), addPathKey(7))
	change, ok := r.checkBestPathChange(famV4, addPathKey(7), true, nil)
	require.True(t, ok)
	assert.Equal(t, uint32(7), change.PathID)

	group, ok := r.locRIB.Load().Lookup(famV4, pfx)
	require.True(t, ok)
	bgpPaths := 0
	for _, p := range group.Paths {
		if p.Source == bgpProtocolID {
			bgpPaths++
			assert.Equal(t, netip.MustParseAddr("10.0.0.7"), p.NextHop)
		}
	}
	assert.Equal(t, 1, bgpPaths, "one BGP path per prefix in the Loc-RIB")
}

// TestAddPathWithdrawalKeepsOrPromotes checks withdrawal of each path of an
// ADD-PATH prefix.
//
// VALIDATES: AC-10: withdrawing the non-best path 9 changes nothing; then
// withdrawing the best path 7 promotes path 9, and withdrawing path 9 last
// withdraws the prefix, each change naming its path.
// PREVENTS: a withdrawal judged by the withdrawn path id alone.
func TestAddPathWithdrawalKeepsOrPromotes(t *testing.T) {
	r := newTestRIBManagerWithBus(newTestEventBus())
	rib := newAddPathPeer(r, netip.MustParseAddr("192.0.2.31"))
	rib.Insert(famV4, unicastAttrs([4]byte{10, 0, 0, 7}, 10, 100), addPathKey(7))
	r.checkBestPathChange(famV4, addPathKey(7), true, nil)
	rib.Insert(famV4, unicastAttrs([4]byte{10, 0, 0, 9}, 20, 100), addPathKey(9))
	r.checkBestPathChange(famV4, addPathKey(9), true, nil)

	rib.Remove(famV4, addPathKey(9))
	_, ok := r.checkBestPathChange(famV4, addPathKey(9), true, nil)
	assert.False(t, ok, "withdrawing the non-best path keeps the best")

	rib.Insert(famV4, unicastAttrs([4]byte{10, 0, 0, 9}, 20, 100), addPathKey(9))
	r.checkBestPathChange(famV4, addPathKey(9), true, nil)
	rib.Remove(famV4, addPathKey(7))
	promoted, ok := r.checkBestPathChange(famV4, addPathKey(7), true, nil)
	require.True(t, ok)
	assert.Equal(t, ribevents.BestChangeUpdate, promoted.Action)
	assert.Equal(t, uint32(9), promoted.PathID)
	assert.Equal(t, netip.MustParseAddr("10.0.0.9"), promoted.NextHop)

	rib.Remove(famV4, addPathKey(9))
	gone, ok := r.checkBestPathChange(famV4, addPathKey(9), true, nil)
	require.True(t, ok)
	assert.Equal(t, ribevents.BestChangeWithdraw, gone.Action)
	assert.Equal(t, uint32(9), gone.PathID, "the withdraw names the path that was best")
}
