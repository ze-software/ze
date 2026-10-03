package rib

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// purgeTestPeer registers an up eBGP peer with an empty Adj-RIB-In.
func purgeTestPeer(r *RIBManager, addr netip.Addr, asn uint32) {
	r.peerUp[addr] = true
	r.peerMeta[addr] = &peerMetadata{PeerASN: asn, LocalASN: 65000}
	r.bgpPeers[addr] = storage.NewPeerRIB(addr.String())
}

// purgedBatches returns the best-change batches the bus recorded.
func purgedBatches(bus *testEventBus) []*bestChangeBatch {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	var batches []*bestChangeBatch
	for _, evt := range bus.events {
		if batch, isBatch := evt.Payload.(*bestChangeBatch); isBatch {
			batches = append(batches, batch)
		}
	}
	return batches
}

// TestPurgeWithdrawKeepsABestElectedConcurrently lands an UPDATE's election
// at the instant a peer-down purge withdraws the same prefix.
//
// VALIDATES: the purge decides "no best is recorded, so the route goes" and
// removes the route from the Loc-RIB as one step under the bestPrev shard
// lock, so an election that records a new best for the prefix either lands
// before that step, and the purge keeps the route, or after it, and its own
// mirror reinstalls the route.
// PREVENTS: the purge reading "no record", a concurrent election recording
// and mirroring a new best, and the purge then deleting that best from the
// Loc-RIB the kernel FIB reads while the bgp-rib still records it.
//
// Method: purgeRemoveHook runs the arriving peer's election on its own
// goroutine and gives it 200ms to finish before the purge continues. Without
// the shared lock it finishes; with it, it waits for the purge.
func TestPurgeWithdrawKeepsABestElectedConcurrently(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)

	fam := family.Family{AFI: 1, SAFI: 1}
	leaving := netip.MustParseAddr("192.0.2.40")
	arriving := netip.MustParseAddr("192.0.2.41")
	purgeTestPeer(r, leaving, 65001)
	purgeTestPeer(r, arriving, 65002)

	prefix := ipv4Prefix(24, 10, 40, 0) // 10.40.0.0/24
	pfx := netip.MustParsePrefix("10.40.0.0/24")
	r.bgpPeers[leaving].Insert(fam, makeAttrBytes([4]byte{192, 168, 40, 40}), prefix)
	_, ok := r.checkBestPathChange(fam, prefix, false, nil)
	require.True(t, ok, "the leaving peer's path is the first best")

	elected := make(chan struct{})
	r.purgeRemoveHook = func(hookFam family.Family, hookPfx netip.Prefix) {
		r.purgeRemoveHook = nil
		go func() {
			defer close(elected)
			r.peerMu.RLock()
			arrivingRIB := r.bgpPeers[arriving]
			r.peerMu.RUnlock()
			arrivingRIB.Insert(hookFam, makeAttrBytes([4]byte{192, 168, 41, 41}), prefix)
			r.checkBestPathChange(hookFam, prefix, false, nil)
		}()
		select {
		case <-elected:
		case <-time.After(200 * time.Millisecond):
		}
	}

	r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: leaving.String(), State: rpc.SessionStateDown})
	select {
	case <-elected:
	case <-time.After(10 * time.Second):
		t.Fatal("the concurrent election never finished")
	}
	require.Nil(t, r.purgeRemoveHook, "the purge reached the Loc-RIB removal")

	depth := 0
	for _, d := range r.bestPrev.shardDepth(fam) {
		depth += d
	}
	require.Equal(t, 1, depth, "the arriving peer's path is recorded as the best")
	group, held := loc.Lookup(fam, pfx)
	require.True(t, held, "the Loc-RIB holds the best the bgp-rib records")
	require.Len(t, group.Paths, 1, "the Loc-RIB holds one BGP path for the prefix")
	assert.Equal(t, netip.MustParseAddr("192.168.41.41"), group.Paths[0].NextHop, "the Loc-RIB holds the arriving peer's path")
}

// TestPurgePublishesEachRouteAsItIsElected purges a peer that held the best
// for two prefixes, one another peer still reaches and one nobody does.
//
// VALIDATES: emitPurgedWithdraws publishes each purged route as soon as its
// election answers, one batch per route: the survivor as one Update, the
// orphan as one Withdraw.
// PREVENTS: every route of a family held back until the last election ran, so
// a consumer sees the first route's change only after the whole table was
// re-elected, and a later UPDATE's change for that route can overtake it.
func TestPurgePublishesEachRouteAsItIsElected(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	fam := family.Family{AFI: 1, SAFI: 1}
	leaving := netip.MustParseAddr("192.0.2.50")
	surviving := netip.MustParseAddr("192.0.2.51")
	purgeTestPeer(r, leaving, 65001)
	purgeTestPeer(r, surviving, 65002)

	shared := ipv4Prefix(24, 10, 50, 0) // 10.50.0.0/24, both peers
	orphan := ipv4Prefix(24, 10, 51, 0) // 10.51.0.0/24, the leaving peer only
	for _, nlri := range [][]byte{shared, orphan} {
		r.bgpPeers[leaving].Insert(fam, makeAttrBytes([4]byte{192, 168, 50, 50}), nlri)
		_, ok := r.checkBestPathChange(fam, nlri, false, nil)
		require.True(t, ok, "the leaving peer's path is the first best")
	}
	// It ties on every attribute step and loses on the higher peer address.
	r.bgpPeers[surviving].Insert(fam, makeAttrBytes([4]byte{192, 168, 51, 51}), shared)
	_, ok := r.checkBestPathChange(fam, shared, false, nil)
	require.False(t, ok, "the surviving peer's path does not displace the best")

	bus.mu.Lock()
	bus.events = nil
	bus.mu.Unlock()
	r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: leaving.String(), State: rpc.SessionStateDown})

	batches := purgedBatches(bus)
	require.Len(t, batches, 2, "one batch for each purged route")
	actions := map[netip.Prefix]routeaction.Action{}
	for _, batch := range batches {
		require.Len(t, batch.Changes, 1, "a batch carries the one route its election answered")
		actions[batch.Changes[0].Prefix] = batch.Changes[0].Action
	}
	assert.Equal(t, ribevents.BestChangeUpdate, actions[netip.MustParsePrefix("10.50.0.0/24")], "the survivor replaces the departed best")
	assert.Equal(t, ribevents.BestChangeWithdraw, actions[netip.MustParsePrefix("10.51.0.0/24")], "the orphan is withdrawn")
}
