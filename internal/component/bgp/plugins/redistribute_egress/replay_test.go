package redistributeegress

import (
	"context"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: TestOrchestratorFiresReplayOnPeerUp -- a BGP peer's down->up edge
// allocates a replayID, records replayID->peer, and emits ReplayRequest for a
// bgp destination only. An already-up peer (no edge) does NOT re-fire, and a
// deployment with no bgp import does not fire at all.
// PREVENTS: re-sending to already-up peers (AC-2) and pointless replay storms.
func TestOrchestratorFiresReplayOnPeerUp(t *testing.T) {
	resetState(t)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Destination: "bgp", Families: []family.Family{family.IPv4Unicast}},
	}))

	bus := newTestBus()
	var gotIDs []uint64
	redistevents.ReplayRequestEvent.Subscribe(bus, func(r *redistevents.ReplayRequest) {
		gotIDs = append(gotIDs, r.ReplayID)
	})

	coord := newReplayCoordinator()

	// down->up edge fires a replay with a nonzero ID recorded against the peer.
	id, fired := coord.onPeerUp(bus, "10.0.0.1")
	require.True(t, fired)
	assert.NotZero(t, id)
	require.Len(t, gotIDs, 1)
	assert.Equal(t, id, gotIDs[0], "emitted ReplayRequest carries the allocated ID")

	peer, ok := coord.lookupTarget(id)
	assert.True(t, ok)
	assert.Equal(t, "10.0.0.1", peer)

	// Same peer already up: not a down->up edge, no re-fire.
	_, fired2 := coord.onPeerUp(bus, "10.0.0.1")
	assert.False(t, fired2)
	assert.Len(t, gotIDs, 1, "already-up peer must not fire a second replay")

	// After a down, the next up is a fresh edge again.
	coord.onPeerDown("10.0.0.1")
	_, fired3 := coord.onPeerUp(bus, "10.0.0.1")
	assert.True(t, fired3)
	assert.Len(t, gotIDs, 2)

	// No import feeds bgp -> no replay fired.
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Destination: "ospf"},
	}))
	_, fired4 := coord.onPeerUp(bus, "10.0.0.2")
	assert.False(t, fired4, "no bgp destination -> no replay")
	assert.Len(t, gotIDs, 2)
}

// VALIDATES: TestOrchestratorTargetsNewPeerOnly -- a ReplayID-tagged batch is
// injected to the mapped peer (single-peer selector, never "*"); distinct
// replayIDs per peer-up route to distinct peers with no cross-delivery; an
// unknown replayID is dropped.
// PREVENTS: R-1 (wrong-peer / thundering herd) and R-2 (concurrent cross-delivery).
func TestOrchestratorTargetsNewPeerOnly(t *testing.T) {
	resetState(t)
	fakeID := redistevents.RegisterProtocol("fakeredist")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Destination: "bgp", Families: []family.Family{family.IPv4Unicast}},
	}))
	consumer := registerBGPConsumer(t)

	coord := newReplayCoordinator()
	setReplayCoordinator(coord)
	t.Cleanup(func() { setReplayCoordinator(nil) })

	bus := newTestBus()
	idA, firedA := coord.onPeerUp(bus, "10.0.0.1")
	idB, firedB := coord.onPeerUp(bus, "10.0.0.2")
	require.True(t, firedA)
	require.True(t, firedB)
	require.NotEqual(t, idA, idB, "distinct replayID per peer-up")

	// Producer re-emit for peer A: targeted to peer A only.
	batchA := addBatch(fakeID, afiIPv4, "10.0.0.1/32", "")
	batchA.ReplayID = idA
	handleBatch(context.Background(), skipIDs(bgpID), batchA)

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1)
	assert.Equal(t, "10.0.0.1", inj[0].entry.Peer, "replay batch must target peer A, not '*'")
	assert.Equal(t, "10.0.0.1/32", inj[0].entry.Prefix)

	// Producer re-emit for peer B: targeted to peer B only, no cross-delivery.
	batchB := addBatch(fakeID, afiIPv4, "10.0.0.2/32", "")
	batchB.ReplayID = idB
	handleBatch(context.Background(), skipIDs(bgpID), batchB)

	inj = consumer.snapshotInjected()
	require.Len(t, inj, 2)
	assert.Equal(t, "10.0.0.2", inj[1].entry.Peer, "replay batch must target peer B")

	// Unknown/expired replayID is dropped, never mis-delivered.
	batchUnknown := addBatch(fakeID, afiIPv4, "10.0.0.9/32", "")
	batchUnknown.ReplayID = 999999
	handleBatch(context.Background(), skipIDs(bgpID), batchUnknown)
	assert.Len(t, consumer.snapshotInjected(), 2, "unknown replayID must be dropped")
}

// VALIDATES: a replay batch reflects the CURRENT live set (adds only); a remove
// entry in a replay batch is skipped rather than fanned out as a withdraw.
// PREVENTS: a stray withdraw on the replay path (AC-4 liveness semantics).
func TestOrchestratorReplaySkipsRemoveEntries(t *testing.T) {
	resetState(t)
	fakeID := redistevents.RegisterProtocol("fakeredist")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Destination: "bgp", Families: []family.Family{family.IPv4Unicast}},
	}))
	consumer := registerBGPConsumer(t)

	coord := newReplayCoordinator()
	setReplayCoordinator(coord)
	t.Cleanup(func() { setReplayCoordinator(nil) })

	bus := newTestBus()
	id, _ := coord.onPeerUp(bus, "10.0.0.1")

	batch := removeBatch(fakeID, afiIPv4, "10.0.0.1/32")
	batch.ReplayID = id
	handleBatch(context.Background(), skipIDs(bgpID), batch)

	assert.Empty(t, consumer.snapshotInjected(), "replay must not inject a remove")
	assert.Empty(t, consumer.snapshotWithdrawn(), "replay must not fan out a withdraw")
}

// VALIDATES: an incremental (ReplayID==0) batch keeps the existing all-peers
// fan-out with an empty Peer selector (unchanged behavior, AC-8).
// PREVENTS: the replay branch accidentally scoping the normal incremental path.
func TestOrchestratorIncrementalUnchangedByReplayID(t *testing.T) {
	resetState(t)
	fakeID := redistevents.RegisterProtocol("fakeredist")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Destination: "bgp", Families: []family.Family{family.IPv4Unicast}},
	}))
	consumer := registerBGPConsumer(t)

	// No coordinator set: the incremental path must not depend on it.
	handleBatch(context.Background(), skipIDs(bgpID), addBatch(fakeID, afiIPv4, "10.0.0.1/32", ""))

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1)
	assert.Equal(t, "", inj[0].entry.Peer, "incremental inject keeps the all-peers selector")
}
