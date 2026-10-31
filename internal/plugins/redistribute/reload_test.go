package redistribute

import (
	"context"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// The tests in this file judge what a reload of the import rules does to the
// routes each consumer already holds: a newly accepted route is dispatched, a
// newly rejected one withdrawn, a route whose chain now sets other values is
// sent again, and a rollback puts back what the apply changed. A reload that
// lands while a batch is being dispatched is judged too, with no sleep: the
// consumer itself runs the apply from inside InjectRoute, which is the exact
// point between the dispatch and the held-set record.

const (
	reloadSource = "fakeredist"
	prefixTag42  = "10.0.0.42/32"
	prefixTag7   = "10.0.0.7/32"
)

// reloadEntries is the producer's current set: one route tagged 42, one
// tagged 7.
func reloadEntries() []redistevents.RouteChangeEntry {
	return []redistevents.RouteChangeEntry{
		{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix(prefixTag42), Tag: 42},
		{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix(prefixTag7), Tag: 7},
	}
}

// reloadRig wires one producer and the orchestrator on a test bus, the way
// runPlugin does: the dispatcher subscribed, a replay coordinator installed,
// and a producer that answers every ReplayRequest with its current set, which
// is what the static and connected plugins' reemitAll does. It returns the
// function that emits the producer's set as an incremental batch.
func reloadRig(t *testing.T) func() {
	t.Helper()
	resetState(t)
	id := redistevents.RegisterProtocol(reloadSource)
	redistevents.RegisterProducer(id)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: reloadSource, Protocol: reloadSource}))

	bus := newTestBus()
	setEventBus(bus)
	setReplayCoordinator(newReplayCoordinator())
	t.Cleanup(func() { setReplayCoordinator(nil) })

	handle := events.Register[*redistevents.RouteChangeBatch](reloadSource, redistevents.EventType)
	emit := func(replayID uint64) {
		b := &redistevents.RouteChangeBatch{Protocol: id, AFI: afiIPv4, SAFI: safiUnicst, Entries: reloadEntries(), ReplayID: replayID}
		_, err := handle.Emit(bus, b)
		require.NoError(t, err)
	}
	redistevents.ReplayRequestEvent.Subscribe(bus, func(r *redistevents.ReplayRequest) { emit(r.ReplayID) })

	ctx, cancel := context.WithCancel(t.Context())
	unsubs := subscribe(ctx, bus, nil)
	t.Cleanup(func() {
		for _, u := range unsubs {
			u()
		}
		cancel()
	})
	return func() { emit(0) }
}

// bgpRules is the rule set fakeredist -> bgp with chain, or no chain when nil.
func bgpRules(chain configredist.ImportRule) []configredist.ImportRule {
	chain.Source, chain.Destination = reloadSource, "bgp"
	return []configredist.ImportRule{chain}
}

// installRules starts the plugin with rules, as configure does.
func installRules(rules []configredist.ImportRule) *ruleState {
	consumerHeld.install(configredist.NewEvaluator(rules))
	return &ruleState{}
}

// reload runs one transaction's verify and apply with rules.
func reload(t *testing.T, state *ruleState, rules []configredist.ImportRule) {
	t.Helper()
	state.mu.Lock()
	state.pending, state.verified = rules, true
	state.mu.Unlock()
	require.NoError(t, state.apply())
}

// isHeld answers whether consumer holds prefix from the reload source.
func isHeld(consumer, prefix string) bool {
	consumerHeld.mu.Lock()
	defer consumerHeld.mu.Unlock()
	_, ok := consumerHeld.set[heldKey{consumer: consumer, source: reloadSource, family: family.IPv4Unicast, prefix: netip.MustParsePrefix(prefix)}]
	return ok
}

// prefixes lists the prefixes of injected entries, in order.
func prefixes(inj []injectedEntry) []string {
	out := make([]string, 0, len(inj))
	for _, e := range inj {
		out = append(out, e.entry.Prefix)
	}
	return out
}

// TestReloadWideningDispatchesNewlyAcceptedRoutes proves a reload that widens
// an import's chain admits, at once, the routes the old chain kept out.
//
// VALIDATES: apply replays every registered consumer under the new rules, and
// the replay records what it sent in the held set.
// PREVENTS: a widened chain that admits nothing until each source next
// announces its routes, which a static route never does.
func TestReloadWideningDispatchesNewlyAcceptedRoutes(t *testing.T) {
	emit := reloadRig(t)
	consumer := registerBGPConsumer(t)
	state := installRules(bgpRules(configredist.ImportRule{Final: "accept", Chain: tagChain(42)}))

	emit()
	require.Equal(t, []string{prefixTag42}, prefixes(consumer.snapshotInjected()), "the narrow chain moves the tag 42 route alone")

	reload(t, state, bgpRules(configredist.ImportRule{}))

	inj := consumer.snapshotInjected()
	assert.Contains(t, prefixes(inj[1:]), prefixTag7, "the widened chain dispatches the route the old chain kept out")
	assert.Empty(t, consumer.snapshotWithdrawn(), "widening withdraws nothing")
	assert.True(t, isHeld("bgp", prefixTag7), "the newly accepted route is held, so a later narrowing can withdraw it")
}

// TestReloadNarrowingWithdrawsAndDoesNotResend proves a narrowed chain
// withdraws the route it now rejects, and that the replay apply runs does not
// send that route back.
//
// VALIDATES: the newly rejected route is withdrawn once and left out of the
// replay.
// PREVENTS: a replay that re-announces the route the withdraw scan retired.
func TestReloadNarrowingWithdrawsAndDoesNotResend(t *testing.T) {
	emit := reloadRig(t)
	consumer := registerBGPConsumer(t)
	state := installRules(bgpRules(configredist.ImportRule{}))

	emit()
	require.Len(t, consumer.snapshotInjected(), 2, "with no chain both routes move")

	reload(t, state, bgpRules(configredist.ImportRule{Final: "accept", Chain: tagChain(42)}))

	withdrawn := consumer.snapshotWithdrawn()
	require.Len(t, withdrawn, 1, "only the route the narrowed chain rejects is withdrawn")
	assert.Equal(t, prefixTag7, withdrawn[0].prefix)
	assert.NotContains(t, prefixes(consumer.snapshotInjected()[2:]), prefixTag7, "the replay does not send the rejected route back")
	assert.False(t, isHeld("bgp", prefixTag7))
	assert.True(t, isHeld("bgp", prefixTag42))
}

// TestReloadChangedSetsAreResent proves a held route whose chain now sets a
// different metric reaches the consumer again with the new value.
//
// VALIDATES: the replay apply runs re-sends an accepted route with the sets
// the new chain applies.
// PREVENTS: a destination left advertising the metric of a chain the operator
// replaced.
func TestReloadChangedSetsAreResent(t *testing.T) {
	emit := reloadRig(t)
	consumer := registerBGPConsumer(t)
	state := installRules(bgpRules(configredist.ImportRule{Final: "accept", Chain: metricChain(50)}))

	emit()
	inj := consumer.snapshotInjected()
	require.Len(t, inj, 2)
	require.Equal(t, uint32(50), inj[0].entry.Metric)

	reload(t, state, bgpRules(configredist.ImportRule{Final: "accept", Chain: metricChain(70)}))

	inj = consumer.snapshotInjected()
	require.Len(t, inj, 4, "both held routes are sent again")
	for _, e := range inj[2:] {
		assert.True(t, e.entry.HasMetric)
		assert.Equal(t, uint32(70), e.entry.Metric, "%s carries the new chain's metric", e.entry.Prefix)
	}
	assert.Empty(t, consumer.snapshotWithdrawn(), "a changed set replaces the route, it does not withdraw it")
}

// TestRollbackRestoresWithdrawnRoutes proves a rolled-back transaction puts
// back the routes its apply withdrew.
//
// VALIDATES: rollback reinstalls the previous evaluator and replays every
// consumer under it.
// PREVENTS: another plugin's failed apply leaving this plugin's destinations
// without the routes the running config still imports.
func TestRollbackRestoresWithdrawnRoutes(t *testing.T) {
	emit := reloadRig(t)
	consumer := registerBGPConsumer(t)
	state := installRules(bgpRules(configredist.ImportRule{}))

	emit()
	reload(t, state, bgpRules(configredist.ImportRule{Final: "accept", Chain: tagChain(42)}))
	require.Len(t, consumer.snapshotWithdrawn(), 1, "the apply withdrew the tag 7 route")
	before := len(consumer.snapshotInjected())

	require.NoError(t, state.rollback())

	assert.Contains(t, prefixes(consumer.snapshotInjected()[before:]), prefixTag7, "the rollback sends the withdrawn route back")
	assert.True(t, isHeld("bgp", prefixTag7))
}

// reloadingConsumer is a stub consumer whose first InjectRoute runs reload
// before it records the route: the apply then lands after the dispatcher judged
// and sent the route, and before it records the route as held.
type reloadingConsumer struct {
	*stubConsumer
	once   sync.Once
	reload func()
}

func (c *reloadingConsumer) InjectRoute(ctx context.Context, fam family.Family, entry configredist.RouteEntry) {
	c.once.Do(c.reload)
	c.stubConsumer.InjectRoute(ctx, fam, entry)
}

// TestReloadDuringDispatchWithdrawsStaleAdd proves an Add judged under the old
// rules and recorded after an apply narrowed them is withdrawn, not held.
//
// VALIDATES: hold re-judges under the installed rules when the evaluator
// generation moved since the batch loaded its evaluator.
// PREVENTS: a route the new chain rejects staying held and advertised because
// it reached the held set after apply's withdraw scan.
func TestReloadDuringDispatchWithdrawsStaleAdd(t *testing.T) {
	resetState(t)
	id := redistevents.RegisterProtocol(reloadSource)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: reloadSource, Protocol: reloadSource}))
	state := installRules(bgpRules(configredist.ImportRule{}))
	consumer := &reloadingConsumer{stubConsumer: &stubConsumer{name: "bgp"}}
	consumer.reload = func() { reload(t, state, bgpRules(configredist.ImportRule{Final: "accept", Chain: tagChain(42)})) }
	require.NoError(t, configredist.RegisterConsumer(consumer))

	batch := &redistevents.RouteChangeBatch{Protocol: id, AFI: afiIPv4, SAFI: safiUnicst, Entries: []redistevents.RouteChangeEntry{
		{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix(prefixTag7), Tag: 7},
	}}
	handleBatch(context.Background(), skipIDs(), batch)

	withdrawn := consumer.snapshotWithdrawn()
	require.Len(t, withdrawn, 1, "the stale Add is withdrawn once the new rules reject it")
	assert.Equal(t, prefixTag7, withdrawn[0].prefix)
	assert.False(t, isHeld("bgp", prefixTag7), "a route the installed rules reject is not held")
}

// TestReloadDuringDispatchSendsStaleReject proves the converse: an Add the old
// rules rejected for one consumer, judged after an apply that now accepts it,
// is sent and held. The reload runs from the first consumer's InjectRoute, so
// the second consumer is judged with the batch's stale evaluator.
//
// VALIDATES: a rejected Add whose generation moved is judged again and sent
// under the installed rules.
// PREVENTS: a route the new chain admits kept out of a destination because the
// batch that carried it judged it under the replaced rules.
func TestReloadDuringDispatchSendsStaleReject(t *testing.T) {
	resetState(t)
	id := redistevents.RegisterProtocol(reloadSource)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: reloadSource, Protocol: reloadSource}))
	narrow := []configredist.ImportRule{
		{Source: reloadSource, Destination: "bgp"},
		{Source: reloadSource, Destination: "isis", Final: "accept", Chain: tagChain(42)},
	}
	wide := []configredist.ImportRule{
		{Source: reloadSource, Destination: "bgp"},
		{Source: reloadSource, Destination: "isis"},
	}
	state := installRules(narrow)
	first := &reloadingConsumer{stubConsumer: &stubConsumer{name: "bgp"}}
	first.reload = func() { reload(t, state, wide) }
	require.NoError(t, configredist.RegisterConsumer(first))
	second := &stubConsumer{name: "isis"}
	require.NoError(t, configredist.RegisterConsumer(second))

	batch := &redistevents.RouteChangeBatch{Protocol: id, AFI: afiIPv4, SAFI: safiUnicst, Entries: []redistevents.RouteChangeEntry{
		{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix(prefixTag7), Tag: 7},
	}}
	handleBatch(context.Background(), skipIDs(), batch)

	assert.Equal(t, []string{prefixTag7}, prefixes(second.snapshotInjected()), "the installed rules admit the route the stale evaluator rejected")
	assert.True(t, isHeld("isis", prefixTag7))
}
