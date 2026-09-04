package redistributeegress

import (
	"context"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/events"
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

	target, ok := coord.lookupTarget(id)
	assert.True(t, ok)
	assert.Equal(t, replayKindPeer, target.kind)
	assert.Equal(t, "10.0.0.1", target.name)

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

// replayingProducer subscribes a fake producer to ReplayRequest and answers
// each one with its current set, which is what the static and connected
// plugins' reemitAll does. Returns the handle its incremental emits use.
func replayingProducer(t *testing.T, bus *testBus, id redistevents.ProtocolID, name, prefix string) func(uint64) {
	t.Helper()
	handle := events.Register[*redistevents.RouteChangeBatch](name, redistevents.EventType)
	emit := func(replayID uint64) {
		b := addBatch(id, afiIPv4, prefix, "")
		b.ReplayID = replayID
		_, err := handle.Emit(bus, b)
		require.NoError(t, err)
	}
	redistevents.ReplayRequestEvent.Subscribe(bus, func(r *redistevents.ReplayRequest) { emit(r.ReplayID) })
	return emit
}

// TestLateConsumerReceivesProducerSet is AC-3 at the orchestrator: a consumer
// that registers after a producer emitted holds that producer's current set.
//
// VALIDATES: AC-3 -- the static plugin and the IS-IS plugin start in the same
// tier, nothing orders them, and the IS-IS LSP has to carry the static prefix
// either way.
// PREVENTS: the dispatch that reads ConsumerNames() live at event time and
// therefore fans a batch out to whoever happened to be registered at that
// instant, losing it for everybody else with no line in any log.
func TestLateConsumerReceivesProducerSet(t *testing.T) {
	resetState(t)
	fakeID := redistevents.RegisterProtocol("fakeredist")
	redistevents.RegisterProducer(fakeID)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Destination: "isis"},
	}))

	bus := newTestBus()
	setEventBus(bus)
	emit := replayingProducer(t, bus, fakeID, "fakeredist", "10.99.0.0/24")

	coord := newReplayCoordinator()
	setReplayCoordinator(coord)
	t.Cleanup(func() { setReplayCoordinator(nil) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	unsubs := subscribe(ctx, bus, nil)
	defer func() {
		for _, u := range unsubs {
			u()
		}
	}()
	defer watchConsumers(bus, coord)()

	// The producer emits its route before any consumer exists. This is the
	// batch that is lost today.
	emit(0)

	isis := &stubConsumer{name: "isis"}
	require.NoError(t, configredist.RegisterConsumer(isis))

	inj := isis.snapshotInjected()
	require.Len(t, inj, 1, "the consumer must hold the producer's set the moment it registers")
	assert.Equal(t, "10.99.0.0/24", inj[0].entry.Prefix)
	assert.Empty(t, inj[0].entry.Peer, "a consumer replay takes the all-peers fan-out, not the single-peer selector")
	assert.Equal(t, "fakeredist", inj[0].entry.Source)
}

// TestConsumerAlreadyRegisteredIsSweptAtStartup is the other startup order. The
// consumer registered before the dispatcher subscribed, so no observer can have
// seen it, and the producer's batch reached no dispatcher either.
//
// VALIDATES: AC-3 for the order the observer alone does not cover.
// PREVENTS: fixing one arrival order and leaving the other, which would make
// the outcome depend on which plugin tier ran first.
func TestConsumerAlreadyRegisteredIsSweptAtStartup(t *testing.T) {
	resetState(t)
	fakeID := redistevents.RegisterProtocol("fakeredist")
	redistevents.RegisterProducer(fakeID)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Destination: "isis"},
	}))

	bus := newTestBus()
	setEventBus(bus)
	emit := replayingProducer(t, bus, fakeID, "fakeredist", "10.99.0.0/24")

	isis := &stubConsumer{name: "isis"}
	require.NoError(t, configredist.RegisterConsumer(isis))
	emit(0)
	require.Empty(t, isis.snapshotInjected(), "nothing dispatches before the orchestrator subscribes")

	coord := newReplayCoordinator()
	setReplayCoordinator(coord)
	t.Cleanup(func() { setReplayCoordinator(nil) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	unsubs := subscribe(ctx, bus, nil)
	defer func() {
		for _, u := range unsubs {
			u()
		}
	}()
	defer watchConsumers(bus, coord)()

	inj := isis.snapshotInjected()
	require.Len(t, inj, 1, "the startup sweep must replay for a consumer that registered first")
	assert.Equal(t, "10.99.0.0/24", inj[0].entry.Prefix)
}

// TestReplayFiresOncePerConsumer holds the trigger to R-2: one registration
// emits one ReplayRequest, and a consumer no rule imports into emits none.
//
// VALIDATES: R-2 -- the boundary row "1 request per registration", whose
// invalid-above case is a duplicate request per producer.
// PREVENTS: a replay storm at startup, where every consumer asks every producer
// separately and a deployment with N consumers and M producers pays N*M.
func TestReplayFiresOncePerConsumer(t *testing.T) {
	resetState(t)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Destination: "isis"},
	}))

	bus := newTestBus()
	setEventBus(bus)

	// Two producers answer the broadcast. One request still has to be ONE
	// request. The count below is of requests, and two producers answering it
	// is the mechanism rather than a second request.
	var requests []uint64
	redistevents.ReplayRequestEvent.Subscribe(bus, func(r *redistevents.ReplayRequest) {
		requests = append(requests, r.ReplayID)
	})
	redistevents.ReplayRequestEvent.Subscribe(bus, func(_ *redistevents.ReplayRequest) {})

	coord := newReplayCoordinator()
	setReplayCoordinator(coord)
	t.Cleanup(func() { setReplayCoordinator(nil) })
	defer watchConsumers(bus, coord)()

	isis := &stubConsumer{name: "isis"}
	require.NoError(t, configredist.RegisterConsumer(isis))
	require.Len(t, requests, 1, "one registration fires exactly one request")

	// An engine instance recreated by an SDK reconnect re-registers, and that
	// instance holds nothing. It is owed one request of its own: one request,
	// never one per producer.
	configredist.ReregisterConsumer(&stubConsumer{name: "isis"})
	require.Len(t, requests, 2)
	assert.NotEqual(t, requests[0], requests[1], "each request carries its own replayID")

	// No rule imports into ospf, so a replay would ask every producer for a set
	// this consumer will reject anyway.
	require.NoError(t, configredist.RegisterConsumer(&stubConsumer{name: "ospf"}))
	assert.Len(t, requests, 2, "a consumer no rule feeds must not fire a request")
}

// TestConsumerReplayRespectsLoopPrevention keeps the second replay target under
// the same invariant as the first: a source protocol's batch is never
// redistributed into that same protocol's consumer.
//
// VALIDATES: the "behavior to preserve" row on loop prevention.
// PREVENTS: an IS-IS batch replayed into the IS-IS consumer, which the config
// in test/interop/scenarios/isis-redist-frr deliberately does not ask for.
func TestConsumerReplayRespectsLoopPrevention(t *testing.T) {
	resetState(t)
	isisID := redistevents.RegisterProtocol("isis")
	redistevents.RegisterProducer(isisID)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "isis", Protocol: "isis"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "isis", Destination: "isis"},
	}))

	bus := newTestBus()
	setEventBus(bus)
	emit := replayingProducer(t, bus, isisID, "isis", "10.99.0.0/24")

	coord := newReplayCoordinator()
	setReplayCoordinator(coord)
	t.Cleanup(func() { setReplayCoordinator(nil) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	unsubs := subscribe(ctx, bus, nil)
	defer func() {
		for _, u := range unsubs {
			u()
		}
	}()
	defer watchConsumers(bus, coord)()

	emit(0)
	isis := &stubConsumer{name: "isis"}
	require.NoError(t, configredist.RegisterConsumer(isis))
	assert.Empty(t, isis.snapshotInjected(), "an isis-sourced batch must never reach the isis consumer")
}
