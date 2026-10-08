package redistributeegress

import (
	"context"
	"net/netip"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// taggedAddBatch builds a one-entry add batch carrying tag.
func taggedAddBatch(p redistevents.ProtocolID, prefix string, tag uint32) *redistevents.RouteChangeBatch {
	return &redistevents.RouteChangeBatch{
		Protocol: p,
		AFI:      afiIPv4,
		SAFI:     safiUnicst,
		Entries: []redistevents.RouteChangeEntry{{
			Action: redistevents.ActionAdd,
			Prefix: netip.MustParsePrefix(prefix),
			Tag:    tag,
		}},
	}
}

// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-2 -- the orchestrator
// copies RouteChangeEntry.Tag into the RouteEntry each consumer receives.
// PREVENTS: the tag reaching the bus and being dropped one hop before a consumer,
// which looks exactly like a producer that never set it.
func TestHandleBatchCarriesEntryTag(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("tagsource")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "tagsource", Protocol: "tagsource"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "tagsource", Destination: "bgp"},
	}))

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), taggedAddBatch(id, "10.0.0.0/8", 7788))

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1)
	assert.Equal(t, uint32(7788), inj[0].entry.Tag)
}

// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-8 -- an import rule naming a
// tag filters ENTRY BY ENTRY, because the tag belongs to the route and not to the batch.
// PREVENTS: a whole batch being accepted or rejected on the tag of no particular route,
// which is what a batch-level decision does when the entries disagree.
func TestHandleBatchFiltersEntriesByTag(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("tagsource")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "tagsource", Protocol: "tagsource"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "tagsource", Destination: "bgp", MatchTag: true, Tag: 42},
	}))

	batch := &redistevents.RouteChangeBatch{
		Protocol: id,
		AFI:      afiIPv4,
		SAFI:     safiUnicst,
		Entries: []redistevents.RouteChangeEntry{
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.1.0.0/16"), Tag: 42},
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.2.0.0/16"), Tag: 43},
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.3.0.0/16")},
		},
	}

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), batch)

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1, "only the route carrying tag 42 is imported")
	assert.Equal(t, "10.1.0.0/16", inj[0].entry.Prefix)
}

// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-2 -- an entry with no tag
// reaches the consumer as zero, which every consumer reads as "no tag".
// PREVENTS: an uninitialised or leaked pooled value arriving as a route tag.
func TestHandleBatchUntaggedEntryCarriesZero(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("tagsource")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "tagsource", Protocol: "tagsource"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "tagsource", Destination: "bgp"},
	}))

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), addBatch(id, afiIPv4, "10.0.0.0/8", ""))

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1)
	assert.Equal(t, uint32(0), inj[0].entry.Tag)
	assert.Equal(t, family.IPv4Unicast, inj[0].fam)
}

// replayTaggedBatch arms a peer-up replay for one peer and returns the batch
// stamped with the replayID the coordinator recorded, so handleBatch routes it
// down the replay path (handleReplayBatch) rather than the incremental one.
func replayTaggedBatch(t *testing.T, batch *redistevents.RouteChangeBatch) *redistevents.RouteChangeBatch {
	t.Helper()
	coord := newReplayCoordinator()
	setReplayCoordinator(coord)
	t.Cleanup(func() { setReplayCoordinator(nil) })

	id, fired := coord.onPeerUp(newTestBus(), "10.0.0.1")
	require.True(t, fired, "a bgp import must arm the peer-up replay")
	batch.ReplayID = id
	return batch
}

// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-2 -- on the REPLAY path
// the orchestrator copies RouteChangeEntry.Tag into the RouteEntry the consumer
// receives, as the incremental path does.
// PREVENTS: a peer that establishes after the producer emitted receiving its
// routes with the tag stripped, while peers present at emit time see it.
func TestHandleReplayBatchCarriesEntryTag(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("tagsource")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "tagsource", Protocol: "tagsource"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "tagsource", Destination: "bgp"},
	}))

	consumer := registerBGPConsumer(t)
	batch := replayTaggedBatch(t, taggedAddBatch(id, "10.0.0.0/8", 7788))
	handleBatch(context.Background(), skipIDs(bgpID), batch)

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1)
	assert.Equal(t, uint32(7788), inj[0].entry.Tag)
	assert.Equal(t, "10.0.0.1", inj[0].entry.Peer, "a peer-up replay reaches only the peer that established")
}

// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-6/AC-8 -- on the REPLAY
// path an import rule naming a tag filters entry by entry.
// PREVENTS: a late peer receiving routes the tag rule excludes, because the
// replay judged the batch without the tag every entry carries.
func TestHandleReplayBatchFiltersEntriesByTag(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("tagsource")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "tagsource", Protocol: "tagsource"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "tagsource", Destination: "bgp", MatchTag: true, Tag: 42},
	}))

	batch := replayTaggedBatch(t, &redistevents.RouteChangeBatch{
		Protocol: id,
		AFI:      afiIPv4,
		SAFI:     safiUnicst,
		Entries: []redistevents.RouteChangeEntry{
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.1.0.0/16"), Tag: 42},
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.2.0.0/16"), Tag: 43},
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.3.0.0/16")},
		},
	})

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), batch)

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1, "only the route carrying tag 42 is replayed")
	assert.Equal(t, "10.1.0.0/16", inj[0].entry.Prefix)
	assert.Equal(t, uint32(42), inj[0].entry.Tag)
}

// retagBatches feeds prefix through handleBatch twice from one source, first
// carrying tag from, then tag to, which is what a static route re-tagged in place
// emits: one Add per announcement and no Remove between them.
func retagBatches(t *testing.T, id redistevents.ProtocolID, prefix string, from, to uint32) {
	t.Helper()
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	handleBatch(context.Background(), skipIDs(bgpID), taggedAddBatch(id, prefix, from))
	handleBatch(context.Background(), skipIDs(bgpID), taggedAddBatch(id, prefix, to))
}

// tagRules installs one import rule into bgp for each tag in tags.
func tagRules(t *testing.T, tags ...uint32) redistevents.ProtocolID {
	t.Helper()
	id := redistevents.RegisterProtocol("tagsource")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "tagsource", Protocol: "tagsource"}))
	rules := make([]configredist.ImportRule, 0, len(tags))
	for _, tag := range tags {
		rules = append(rules, configredist.ImportRule{Source: "tagsource", Destination: "bgp", MatchTag: true, Tag: tag})
	}
	configredist.SetGlobal(configredist.NewEvaluator(rules))
	return id
}

// VALIDATES: a re-tagged route a consumer accepts under both tags is replaced in
// place: the second Add overwrites the first and nothing is withdrawn.
// PREVENTS: a tag change flapping the route at every consumer, which a withdraw
// sent before the replacement does to a destination that keeps the route.
// Method: rules for tags 42 and 43, Add tagged 42 then Add tagged 43.
func TestHandleBatchRetagAcceptedReplacesWithoutWithdraw(t *testing.T) {
	resetState(t)
	id := tagRules(t, 42, 43)
	consumer := registerBGPConsumer(t)

	retagBatches(t, id, "10.0.0.0/8", 42, 43)

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 2)
	assert.Equal(t, uint32(42), inj[0].entry.Tag)
	assert.Equal(t, uint32(43), inj[1].entry.Tag)
	assert.Empty(t, consumer.snapshotWithdrawn(), "a replacement the consumer accepts MUST NOT withdraw first")
}

// VALIDATES: a rejected replacement Add for a prefix the consumer holds removes
// the route it holds, which is BGP import-policy semantics: the update replaces
// the route, and the policy then rejects the replacement.
// PREVENTS: a route re-tagged out of an import rule's set staying in BGP, OSPF or
// IS-IS forever, because the source announced it once under the new tag and
// never withdrew the old one.
// Method: a rule for tag 42 alone, Add tagged 42, Add tagged 43, then a second
// Add tagged 43 that must find nothing left to remove.
func TestHandleBatchRetagRejectedRemovesHeldRoute(t *testing.T) {
	resetState(t)
	id := tagRules(t, 42)
	consumer := registerBGPConsumer(t)

	retagBatches(t, id, "10.0.0.0/8", 42, 43)

	require.Len(t, consumer.snapshotInjected(), 1, "only the 42-tagged announcement passes the rule")
	wd := consumer.snapshotWithdrawn()
	require.Len(t, wd, 1, "the rejected replacement MUST remove the held route")
	assert.Equal(t, "10.0.0.0/8", wd[0].prefix)

	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	handleBatch(context.Background(), skipIDs(bgpID), taggedAddBatch(id, "10.0.0.0/8", 43))
	assert.Len(t, consumer.snapshotWithdrawn(), 1, "a route already removed is not withdrawn twice")
}

// VALIDATES: a rejected Add for a prefix the consumer never held reaches the
// consumer as nothing at all.
// PREVENTS: a withdraw for every rejected Add, which sends a BGP peer a withdraw
// for a prefix Ze never announced to it.
// Method: a rule for tag 42, one Add tagged 43 and no earlier announcement.
func TestHandleBatchRejectedAddNeverHeldDoesNothing(t *testing.T) {
	resetState(t)
	id := tagRules(t, 42)
	consumer := registerBGPConsumer(t)

	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	handleBatch(context.Background(), skipIDs(bgpID), taggedAddBatch(id, "10.0.0.0/8", 43))

	assert.Empty(t, consumer.snapshotInjected())
	assert.Empty(t, consumer.snapshotWithdrawn(), "a consumer that never held the prefix has nothing to remove")
}

// VALIDATES: a consumer that registers again holds nothing the orchestrator
// dispatched to the instance it replaced.
// PREVENTS: a restarted OSPF or IS-IS instance being sent a withdraw for a route
// only its predecessor held.
// Method: hold a 42-tagged route, re-register the consumer under the observer,
// then send a rejected 43-tagged Add.
func TestHandleBatchReregisteredConsumerHoldsNothing(t *testing.T) {
	resetState(t)
	id := tagRules(t, 42)
	registerBGPConsumer(t)
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	handleBatch(context.Background(), skipIDs(bgpID), taggedAddBatch(id, "10.0.0.0/8", 42))

	coord := newReplayCoordinator()
	setReplayCoordinator(coord)
	t.Cleanup(func() { setReplayCoordinator(nil) })
	defer watchConsumers(newTestBus(), coord)()

	successor := &stubConsumer{name: "bgp"}
	require.True(t, configredist.ReregisterConsumer(successor))
	handleBatch(context.Background(), skipIDs(bgpID), taggedAddBatch(id, "10.0.0.0/8", 43))

	assert.Empty(t, successor.snapshotWithdrawn(), "the successor never held the route its predecessor held")
}

// replayIntoISIS registers an isis consumer AFTER a producer announced one
// untagged route, under a rule importing tag 0 alone, so the consumer holds that
// route only through its registration replay. It returns the producer's id and
// the consumer.
func replayIntoISIS(t *testing.T, prefix string) (redistevents.ProtocolID, *stubConsumer) {
	t.Helper()
	fakeID := redistevents.RegisterProtocol("fakeredist")
	redistevents.RegisterProducer(fakeID)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Destination: "isis", MatchTag: true, Tag: 0},
	}))

	bus := newTestBus()
	setEventBus(bus)
	emit := replayingProducer(t, bus, fakeID, "fakeredist", prefix)

	coord := newReplayCoordinator()
	setReplayCoordinator(coord)
	t.Cleanup(func() { setReplayCoordinator(nil) })

	ctx, cancel := context.WithCancel(t.Context())
	unsubs := subscribe(ctx, bus, nil)
	stopWatching := watchConsumers(bus, coord)
	t.Cleanup(func() {
		stopWatching()
		for _, u := range unsubs {
			u()
		}
		cancel()
	})

	emit(0)
	isis := &stubConsumer{name: "isis"}
	require.NoError(t, configredist.RegisterConsumer(isis))
	require.Len(t, isis.snapshotInjected(), 1, "the replay delivers the untagged route")
	return fakeID, isis
}

// VALIDATES: a route a consumer received through its registration replay is a
// route it holds, so a later rejected replacement removes it like any other.
// PREVENTS: a consumer that registered after the source announced keeping a
// route re-tagged out of its import rule, because only the incremental path
// recorded what each consumer holds.
// Method: the consumer is replayed an untagged route under a rule for tag 0,
// then the producer re-announces it tagged 43.
func TestHandleBatchReplayedRouteIsHeld(t *testing.T) {
	resetState(t)
	fakeID, isis := replayIntoISIS(t, "10.99.0.0/24")

	handleBatch(t.Context(), nil, taggedAddBatch(fakeID, "10.99.0.0/24", 43))

	wd := isis.snapshotWithdrawn()
	require.Len(t, wd, 1, "the replayed route is held, so the rejected replacement removes it")
	assert.Equal(t, "10.99.0.0/24", wd[0].prefix)
}

// taggedRemoveBatch builds a one-entry remove batch carrying tag.
func taggedRemoveBatch(p redistevents.ProtocolID, prefix string, tag uint32) *redistevents.RouteChangeBatch {
	b := taggedAddBatch(p, prefix, tag)
	b.Entries[0].Action = redistevents.ActionRemove
	return b
}

// VALIDATES: a Remove for a route the consumer holds reaches the consumer even
// when the import filter rejects the Remove itself.
// PREVENTS: a consumer keeping a route its source withdrew, because the Remove
// carried a tag (or arrived under rules) the filter no longer accepts.
// Method: a rule for tag 42, Add tagged 42, then a Remove tagged 43, then the
// same Remove again, which must find nothing left to withdraw.
func TestHandleBatchRejectedRemoveOfHeldRouteWithdraws(t *testing.T) {
	resetState(t)
	id := tagRules(t, 42)
	consumer := registerBGPConsumer(t)
	bgpID, _ := redistevents.ProtocolIDOf("bgp")

	handleBatch(context.Background(), skipIDs(bgpID), taggedAddBatch(id, "10.0.0.0/8", 42))
	handleBatch(context.Background(), skipIDs(bgpID), taggedRemoveBatch(id, "10.0.0.0/8", 43))

	wd := consumer.snapshotWithdrawn()
	require.Len(t, wd, 1, "the consumer holds the route, so the source's Remove MUST reach it")
	assert.Equal(t, "10.0.0.0/8", wd[0].prefix)

	handleBatch(context.Background(), skipIDs(bgpID), taggedRemoveBatch(id, "10.0.0.0/8", 43))
	assert.Len(t, consumer.snapshotWithdrawn(), 1, "a route already withdrawn is not withdrawn twice")
}

// VALIDATES: a rejected Remove for a route the consumer never held reaches the
// consumer as nothing, as before the held set existed.
// PREVENTS: the held-route rule turning every rejected Remove into a withdraw
// for a prefix the consumer was never sent.
func TestHandleBatchRejectedRemoveNeverHeldDoesNothing(t *testing.T) {
	resetState(t)
	id := tagRules(t, 42)
	consumer := registerBGPConsumer(t)
	bgpID, _ := redistevents.ProtocolIDOf("bgp")

	handleBatch(context.Background(), skipIDs(bgpID), taggedRemoveBatch(id, "10.0.0.0/8", 0))

	assert.Empty(t, consumer.snapshotWithdrawn())
}

// VALIDATES: a Remove for a route the consumer holds only through its
// registration replay reaches the consumer when the filter rejects the Remove.
// A replay batch carries Adds alone (handleReplayBatch skips any other action),
// so the Remove arrives on the incremental path and the replay's part is the
// held record it left.
// PREVENTS: a late-registered consumer keeping a route its source withdrew.
func TestHandleBatchRejectedRemoveOfReplayedRouteWithdraws(t *testing.T) {
	resetState(t)
	fakeID, isis := replayIntoISIS(t, "10.99.0.0/24")

	handleBatch(t.Context(), nil, taggedRemoveBatch(fakeID, "10.99.0.0/24", 43))

	wd := isis.snapshotWithdrawn()
	require.Len(t, wd, 1, "the replayed route is held, so the source's Remove MUST reach it")
	assert.Equal(t, "10.99.0.0/24", wd[0].prefix)
}

// VALIDATES: the withdraw a rejected entry sends for a held route is counted in
// ze_bgp_redistribute_withdrawals, like the withdraw an accepted Remove sends.
// PREVENTS: the counter reading lower than the withdrawals consumers received.
// Method: a rule for tag 42, Add tagged 42, a rejected Add tagged 43 (one
// withdraw), Add tagged 42 again, a rejected Remove tagged 43 (a second).
func TestHandleBatchHeldRouteWithdrawCounted(t *testing.T) {
	resetState(t)
	metricsPtr.Store(nil)
	t.Cleanup(func() { metricsPtr.Store(nil) })
	rec := newRecordingRegistry()
	setMetricsRegistry(rec)

	id := tagRules(t, 42)
	consumer := registerBGPConsumer(t)
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	ctx := context.Background()

	handleBatch(ctx, skipIDs(bgpID), taggedAddBatch(id, "10.0.0.0/8", 42))
	handleBatch(ctx, skipIDs(bgpID), taggedAddBatch(id, "10.0.0.0/8", 43))
	assert.Equal(t, int64(1), rec.value("ze_bgp_redistribute_withdrawals"), "the rejected replacement's withdraw")

	handleBatch(ctx, skipIDs(bgpID), taggedAddBatch(id, "10.0.0.0/8", 42))
	handleBatch(ctx, skipIDs(bgpID), taggedRemoveBatch(id, "10.0.0.0/8", 43))
	assert.Equal(t, int64(2), rec.value("ze_bgp_redistribute_withdrawals"), "the rejected Remove's withdraw")
	assert.Len(t, consumer.snapshotWithdrawn(), 2)
}

// VALIDATES: the held set records a route for each consumer and each source
// apart, so a rejected entry withdraws only where that consumer holds that
// source's route.
// PREVENTS: a rejection at one consumer withdrawing the route another consumer
// holds, and a rejected Add from a source that never reached the consumer
// withdrawing the prefix another source announced there.
// Method: bgp imports tag 42 from tagsource and othersource, isis imports tag 43
// from tagsource. tagsource announces 10.0.0.0/8 tagged 42 (bgp holds it, isis
// rejects it); othersource then announces the same prefix tagged 43 (bgp
// rejects it, and never held it from othersource).
func TestHandleBatchHeldRouteIsPerConsumerAndSource(t *testing.T) {
	resetState(t)
	tagID := redistevents.RegisterProtocol("tagsource")
	otherID := redistevents.RegisterProtocol("othersource")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "tagsource", Protocol: "tagsource"}))
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "othersource", Protocol: "othersource"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "tagsource", Destination: "bgp", MatchTag: true, Tag: 42},
		{Source: "othersource", Destination: "bgp", MatchTag: true, Tag: 42},
		{Source: "tagsource", Destination: "isis", MatchTag: true, Tag: 43},
	}))
	bgp := registerBGPConsumer(t)
	isis := &stubConsumer{name: "isis"}
	require.NoError(t, configredist.RegisterConsumer(isis))
	ctx := context.Background()

	handleBatch(ctx, nil, taggedAddBatch(tagID, "10.0.0.0/8", 42))
	require.Len(t, bgp.snapshotInjected(), 1, "bgp imports tag 42")
	assert.Empty(t, isis.snapshotWithdrawn(), "isis rejected the Add and holds nothing, whatever bgp holds")

	handleBatch(ctx, nil, taggedAddBatch(otherID, "10.0.0.0/8", 43))
	assert.Empty(t, bgp.snapshotWithdrawn(), "othersource never reached bgp, so its rejected Add withdraws nothing there")

	handleBatch(ctx, nil, taggedAddBatch(tagID, "10.0.0.0/8", 43))
	require.Len(t, bgp.snapshotWithdrawn(), 1, "tagsource's rejected replacement still removes what bgp holds from it")
	assert.Empty(t, isis.snapshotWithdrawn(), "isis accepts tag 43: an announcement, never a withdraw")
	assert.Len(t, isis.snapshotInjected(), 1)
}
