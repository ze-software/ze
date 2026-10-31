package redistribute

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// The tests in this file judge the order between a replay and the incremental
// batches a producer emits around it, and between a reload's withdraw scan and
// the batches in flight while it runs. Each one drives the interleaving
// directly, by calling handleBatch in the order under test or by running the
// reload from inside a consumer's InjectRoute, so none waits on a clock.

const orderPrefix = "10.9.9.9/32"

// orderRig registers the reload source with an accept-all import into bgp, the
// BGP stub consumer and a coordinator. It answers the producer's protocol id.
func orderRig(t *testing.T) (redistevents.ProtocolID, *stubConsumer, *replayCoordinator) {
	t.Helper()
	resetState(t)
	id := redistevents.RegisterProtocol(reloadSource)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: reloadSource, Protocol: reloadSource}))
	installRules(bgpRules(configredist.ImportRule{}))
	consumer := registerBGPConsumer(t)
	coord := newReplayCoordinator()
	setReplayCoordinator(coord)
	t.Cleanup(func() { setReplayCoordinator(nil) })
	return id, consumer, coord
}

// orderBatch is one single-entry batch of the reload source.
func orderBatch(id redistevents.ProtocolID, action redistevents.RouteAction, replayID uint64) *redistevents.RouteChangeBatch {
	return &redistevents.RouteChangeBatch{Protocol: id, AFI: afiIPv4, SAFI: safiUnicst, ReplayID: replayID, Entries: []redistevents.RouteChangeEntry{
		{Action: action, Prefix: netip.MustParsePrefix(orderPrefix)},
	}}
}

// TestReplayDoesNotReviveRemovedRoute proves a replay Add from a snapshot the
// producer took before it removed the route is dropped when the Remove reached
// the orchestrator first.
//
// VALIDATES: handleReplayBatch drops an Add whose route's most recent
// incremental action, handled after the replay fired, was a Remove.
// PREVENTS: a reload that changes static routes and redistribute rules together
// leaving the consumer holding a route its source removed.
func TestReplayDoesNotReviveRemovedRoute(t *testing.T) {
	id, consumer, coord := orderRig(t)
	handleBatch(context.Background(), skipIDs(), orderBatch(id, redistevents.ActionAdd, 0))
	require.Len(t, consumer.snapshotInjected(), 1)

	replayID, fired := coord.onConsumerRegistered(newTestBus(), "bgp")
	require.True(t, fired)
	handleBatch(context.Background(), skipIDs(), orderBatch(id, redistevents.ActionRemove, 0))
	handleBatch(context.Background(), skipIDs(), orderBatch(id, redistevents.ActionAdd, replayID))

	assert.Len(t, consumer.snapshotInjected(), 1, "the stale replay Add is not dispatched")
	assert.Len(t, consumer.snapshotWithdrawn(), 1, "the Remove withdrew the route")
	assert.False(t, isHeld("bgp", orderPrefix), "the consumer holds nothing the source removed")
}

// TestReplayDispatchesRouteReaddedOrRemovedBefore proves the drop is no wider
// than the stale snapshot: a route removed then announced again, and a route
// removed before the replay fired, are both replayed normally.
//
// VALIDATES: an incremental Add clears the record, and a Remove handled before
// the request does not count against it.
// PREVENTS: a replay that loses a live route because it was once removed.
func TestReplayDispatchesRouteReaddedOrRemovedBefore(t *testing.T) {
	id, consumer, coord := orderRig(t)
	bus := newTestBus()

	first, _ := coord.onConsumerRegistered(bus, "bgp")
	handleBatch(context.Background(), skipIDs(), orderBatch(id, redistevents.ActionRemove, 0))
	handleBatch(context.Background(), skipIDs(), orderBatch(id, redistevents.ActionAdd, 0))
	before := len(consumer.snapshotInjected())
	consumerHeld.reset()
	handleBatch(context.Background(), skipIDs(), orderBatch(id, redistevents.ActionAdd, first))
	assert.Len(t, consumer.snapshotInjected(), before+1, "a route announced again after its Remove replays")

	handleBatch(context.Background(), skipIDs(), orderBatch(id, redistevents.ActionRemove, 0))
	second, _ := coord.onConsumerRegistered(bus, "bgp")
	handleBatch(context.Background(), skipIDs(), orderBatch(id, redistevents.ActionAdd, second))
	assert.Len(t, consumer.snapshotInjected(), before+2, "a Remove before the request does not drop the replay")
}

// TestWithdrawScanSparesNewerAcceptedVersion proves the reload's withdraw scan
// re-checks a listed route under the lock: a batch that settled a version the
// installed rules accept, after the scan listed the old one, keeps it.
//
// VALIDATES: withdrawListed releases only a route still rejected when it comes
// to it.
// PREVENTS: a check-then-act gap withdrawing a route the new rules accept.
func TestWithdrawScanSparesNewerAcceptedVersion(t *testing.T) {
	resetState(t)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: reloadSource, Protocol: reloadSource}))
	consumer := registerBGPConsumer(t)
	installRules(bgpRules(configredist.ImportRule{Final: "accept", Chain: tagChain(42)}))
	ev, gen := consumerHeld.evaluator()

	k := heldKey{consumer: "bgp", source: reloadSource, family: family.IPv4Unicast, prefix: netip.MustParsePrefix(prefixTag7)}
	view := func(tag uint32) *configredist.RedistRoute {
		r := redistRoute(reloadSource, 0, family.IPv4Unicast, &redistevents.RouteChangeEntry{Prefix: k.prefix, Tag: tag})
		return &r
	}
	sent := sentRoute{}
	consumerHeld.mu.Lock()
	consumerHeld.set[k] = heldRoute{view: *view(7)}
	consumerHeld.mu.Unlock()
	listed := consumerHeld.rejected(ev)
	require.Equal(t, []heldKey{k}, listed)

	// The batch carrying the tag 42 version settles between the list and the withdraw.
	outcome, _ := consumerHeld.settleAdd(k, view(42), &sent, false, true, gen)
	require.Equal(t, settlementSettled, outcome)

	withdrawListed(context.Background(), ev, listed)
	assert.Empty(t, consumer.snapshotWithdrawn(), "the accepted version is not withdrawn")
	assert.True(t, isHeld("bgp", prefixTag7))
}

// TestReloadDuringDispatchWithdrawsHeldRouteOnce proves a held route whose
// replacement Add is in flight when a reload rejects it is withdrawn once: the
// scan leaves it to the Add's settlement.
//
// VALIDATES: releaseRejected skips an in-flight key, and settleAdd withdraws it.
// PREVENTS: the scan and the settlement each sending the same withdraw.
func TestReloadDuringDispatchWithdrawsHeldRouteOnce(t *testing.T) {
	resetState(t)
	id := redistevents.RegisterProtocol(reloadSource)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: reloadSource, Protocol: reloadSource}))
	state := installRules(bgpRules(configredist.ImportRule{}))
	consumer := &reloadingConsumer{stubConsumer: &stubConsumer{name: "bgp"}}
	consumer.reload = func() { reload(t, state, bgpRules(configredist.ImportRule{Final: "accept", Chain: tagChain(42)})) }
	require.NoError(t, configredist.RegisterConsumer(consumer))

	k := heldKey{consumer: "bgp", source: reloadSource, family: family.IPv4Unicast, prefix: netip.MustParsePrefix(prefixTag7)}
	entry := redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: k.prefix, Tag: 7}
	held := redistRoute(reloadSource, id, family.IPv4Unicast, &entry)
	_, gen := consumerHeld.evaluator()
	sent := sentRoute{}
	consumerHeld.settleAdd(k, &held, &sent, false, true, gen)

	handleBatch(context.Background(), skipIDs(), &redistevents.RouteChangeBatch{Protocol: id, AFI: afiIPv4, SAFI: safiUnicst, Entries: []redistevents.RouteChangeEntry{entry}})

	assert.Len(t, consumer.snapshotWithdrawn(), 1, "the rejected route is withdrawn exactly once")
	assert.False(t, isHeld("bgp", prefixTag7))
}

// TestReloadChangingNothingSendsNothing proves a reload whose rules decide the
// same for every held route sends the consumer nothing.
//
// VALIDATES: a consumer replay dispatches only a route the consumer does not
// already hold as it would be sent.
// PREVENTS: every commit re-sending each redistributed route as an identical
// UPDATE to every BGP peer.
func TestReloadChangingNothingSendsNothing(t *testing.T) {
	emit := reloadRig(t)
	consumer := registerBGPConsumer(t)
	state := installRules(bgpRules(configredist.ImportRule{}))
	emit()
	require.Len(t, consumer.snapshotInjected(), 2)

	reload(t, state, bgpRules(configredist.ImportRule{}))

	assert.Len(t, consumer.snapshotInjected(), 2, "an unchanged route is not sent again")
	assert.Empty(t, consumer.snapshotWithdrawn())
	assert.True(t, isHeld("bgp", prefixTag42))
}
