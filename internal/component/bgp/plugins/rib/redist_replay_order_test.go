package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgpredist "github.com/ze-software/ze/internal/component/bgp/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// TestRedistReplayEmitsUnderEmitLock proves the redistribution replay emits
// while holding redistEmitMu exclusive, so a withdraw elected after the replay
// walked the table is published after the replay's Add.
//
// VALIDATES: replayRedistribute holds redistEmitMu from the walk to its last
// emit; a subscriber probes it with TryRLock at the replay emit.
// PREVENTS: a replay Add reaching the redistribute orchestrator after the
// withdraw of the same best path, which revives a withdrawn BGP route.
func TestRedistReplayEmitsUnderEmitLock(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	peerAddr := netip.MustParseAddr("192.0.2.1")
	r.peerMeta[peerAddr] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	r.bgpPeers[peerAddr] = storage.NewPeerRIB(peerAddr.String())
	r.bgpPeers[peerAddr].Insert(fam, makeAttrBytes([4]byte{192, 168, 1, 1}), prefix)
	r.checkBestPathChange(fam, prefix, false, nil)

	unordered, replayed := 0, 0
	bgpredist.RouteChange.Subscribe(bus, func(b *redistevents.RouteChangeBatch) {
		if b.ReplayID == 0 {
			return
		}
		replayed++
		if redistEmitMu.TryRLock() {
			unordered++
			redistEmitMu.RUnlock()
		}
	})

	r.replayRedistribute(&redistevents.ReplayRequest{ReplayID: 3})

	require.Equal(t, 1, replayed)
	require.Zero(t, unordered, "an incremental emit could interleave with the replay")
}
