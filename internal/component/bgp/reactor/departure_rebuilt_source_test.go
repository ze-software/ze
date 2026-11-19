// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
// Related: peer.go -- forwardGenerationClock, advanceForwardGeneration
// Related: relay_recovery.go -- departedOwner, unsendableSurvivor, recoverySources, queueRecoveryOutcome
// Related: adj_rib_out.go -- announcerLifts

package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// rebuiltSource is a new Peer for old's address, as a config remove and
// re-add builds it, or as a dynamic peer is created again after its cleanup.
func rebuiltSource(t *testing.T, old *Peer) *Peer {
	t.Helper()
	return makeForwardSourcePeer(t, old.sendCtx.Load(), old.sendCtxID)
}

// endSession ends source's running session the way graceful restart leaves
// it: no longer Established, its routes retained by the RIB.
func endSession(source *Peer) {
	source.state.Store(int32(PeerStateConnecting))
}

// oldPeerSessions runs three sessions of source through the generation
// producer, writes prefix to destination on the last unless prefix is nil, and
// ends it. It retains that final Session’s issued source facts before ending it.
func oldPeerSessions(t *testing.T, rail *owedRail, prefix []byte) recoveryFixtureOrigin {
	t.Helper()
	for range 3 {
		recoveryFixtureSession(t, rail.source, rail.source.sendCtxID)
		rail.source.setState(PeerStateEstablished)
	}
	if prefix != nil {
		adjOutTestForward(t, rail.destination, rail.source, prefix, 1)
	}
	original := captureRecoveryFixtureOrigin(t, rail.source, rail.source.currentSession(), rail.source.sendCtxID)
	endSession(rail.source)
	return original
}

// PREVENTS: a departure queued by a removed Peer owing the route a Peer built
// again for the same address wrote, after that new Peer's session ended under
// graceful restart, which withdraws a route RFC 4724 Section 4.2 says the
// speaker MUST retain. Generations span the process and exclusions identify
// stable Peer owners, so a count that
// restarts with each Peer makes the new Peer's session look older than the
// departure. Method: the old Peer ends its third session and is removed; a
// new Peer for the address establishes, writes the key and ends; the old
// Peer's departure, run only now, must owe nothing. (A key the old Peer still
// owned would make the new Peer's announcement contested and owed, not
// written, so the old Peer writes nothing here.)
func TestRebuiltPeerRouteSurvivesOldPeerDeparture(t *testing.T) {
	rail := newOwedRail(t)
	source := rail.source.Settings().Address
	prefix := []byte{24, 192, 0, 2}
	original := oldPeerSessions(t, rail, nil)

	rebuilt := rebuiltSource(t, rail.source)
	rebuilt.setState(PeerStateEstablished)
	adjOutTestForward(t, rail.destination, rebuilt, prefix, 2)
	endSession(rebuilt)

	entry, held := adjOutTestEntry(rail.destination)
	require.True(t, held)
	require.Same(t, rebuilt, entry.owner.source, "the rebuilt Peer owns the key")
	require.Greater(t, entry.owner.generation, original.generation,
		"a rebuilt Peer's session is newer than every session its predecessor ended")

	stale := departureGenerationBatch(source, rail.source.sourceOwner, original.generation)
	owed, err := oweDeparture(rail.destination, rail.destination.currentSession(), &stale)
	require.NoError(t, err)
	require.Empty(t, owed, "the old Peer's departure owes the rebuilt Peer's retained route")
}

// PREVENTS: an exclusion of a removed Peer applying to a distinct stable owner
// built again at its address after that Peer announced the key and lost that
// session under graceful restart, so the election skipped its retained route
// (RFC 4724 Section 4.2). Method: the old Peer's departure owes the key; the
// rebuilt Peer announces it while it is owed and ends; the round must ask the
// RIB with the source electable. A record holding both Peers' announcers, the
// old one first, must not exclude the distinct rebuilt owner. The old owner
// may remain excluded; no address alias transfers that exclusion.
func TestRebuiltPeerAnnouncementLiftsOldPeerExclusion(t *testing.T) {
	rail := newOwedRail(t)
	source := rail.source.Settings().Address
	prefix := []byte{24, 192, 0, 2}
	original := oldPeerSessions(t, rail, prefix)

	departure := departureGenerationBatch(source, rail.source.sourceOwner, original.generation)
	owed, err := oweDeparture(rail.destination, rail.destination.currentSession(), &departure)
	require.NoError(t, err)
	require.Len(t, owed, 1, "the old Peer's departure owes its route")
	record := owed[0]

	rebuilt := rebuiltSource(t, rail.source)
	rebuilt.setState(PeerStateEstablished)
	adjOutTestForward(t, rail.destination, rebuilt, prefix, 2)
	endSession(rebuilt)

	table := &rail.destination.adjOut
	table.mu.Lock()
	excluded := record.electableExcluded()
	table.mu.Unlock()
	require.NotContains(t, excluded, rebuilt.sourceOwner,
		"the rebuilt Peer's retained route is skipped by the election")

	both := adjOutOwed{
		excluded: []adjOutExclusion{{source: rail.source.sourceOwner, generation: original.generation, departed: original.generation}},
		announcers: []adjOutAnnouncer{
			{source: rail.source, generation: original.generation},
			{source: rebuilt, generation: rebuilt.forwardGeneration.Load()},
		},
	}
	require.Contains(t, both.electableExcluded(), rail.source.sourceOwner, "the old owner remains independently excluded")
	require.NotContains(t, both.electableExcluded(), rebuilt.sourceOwner,
		"the old Peer's announcer hides the rebuilt Peer's newer one")
}

// PREVENTS: a survivor that could not be sent excluding its source from every
// later session too, so a source that reconnects, announces the key and ends
// under graceful restart again before the next round stays excluded and its
// retained route is replaced or withdrawn (RFC 4724 Section 4.2). Method: the
// source's departure owes the key; the round's survivor is a path of the
// source the destination does not hold, so the key is re-owed with the source
// excluded, as the round's settlement merges it; the source then establishes
// a new session, announces the key and ends; the next round must ask the RIB
// with the source electable.
func TestUnsendableSurvivorExclusionLiftedByLaterSession(t *testing.T) {
	rail := newOwedRail(t)
	source := rail.source.Settings().Address
	prefix := []byte{24, 192, 0, 2}
	original := oldPeerSessions(t, rail, prefix)

	departure := departureGenerationBatch(source, rail.source.sourceOwner, original.generation)
	owed, err := oweDeparture(rail.destination, rail.destination.currentSession(), &departure)
	require.NoError(t, err)
	require.Len(t, owed, 1, "the departure owes the source's route")
	record := owed[0]

	key := departureGenerationKey(t, rail.destination)
	entry, held := adjOutTestEntry(rail.destination)
	require.True(t, held)
	unheld := ribevents.RecoveryRoute{Source: original.address, SourceOwner: original.owner, SourceReceipt: original.receipt, SourcePolicyEpoch: rail.source.sourcePolicyEpoch.Load(), PathID: entry.owner.received + 1,
		AddPath: entry.owner.addPath, MessageID: 3}
	decided := recoveryAdmitKey{key: key, record: record}
	admission := (&reactorAPIAdapter{r: rail.r}).newRecoveryAdmission(t.Context(), rail.destination, nil, nil)
	admission.unsendableSurvivor(&decided, &unheld)
	require.Equal(t, recoveryKeyReowed, decided.outcome,
		"a path the destination does not hold cannot be sent from a sessionless source")
	table := &rail.destination.adjOut
	table.mu.Lock()
	record.exclude(decided.gone)
	beforeLift := record.electableExcluded()
	table.mu.Unlock()
	require.Equal(t, original.owner, decided.gone.source, "the actual original owner is excluded")
	require.Contains(t, beforeLift, original.owner, "exclusion must exist before a later announcement lifts it")

	recoveryFixtureSession(t, rail.source, rail.source.sendCtxID)
	rail.source.setState(PeerStateEstablished)
	adjOutTestForward(t, rail.destination, rail.source, prefix, 4)
	endSession(rail.source)

	table.mu.Lock()
	excluded := record.electableExcluded()
	table.mu.Unlock()
	require.NotContains(t, excluded, rail.source.sourceOwner,
		"a session started after the decision stays excluded by the unsendable survivor")
}

// PREVENTS: a survivor judged sessionless against the round's source snapshot
// being excluded up to the clock at the decision, which covers a session the
// source started after the snapshot, during the RIB lookup. That session
// announces the key and ends under graceful restart before the next round,
// and the exclusion then skips its retained route (RFC 4724 Section 4.2).
// Method: the source's departure owes the key; the round's snapshot is taken
// through recoverySources while the source is sessionless; the source then
// starts a session; the survivor goes through queueRecoveryOutcome, whose
// queueRecovery refuses it because the session is not the snapshot's
// (errRelayNoSource), so the key is re-owed with the source excluded; that
// session announces the key and ends; the next round must ask the RIB with
// the source electable.
func TestUnsendableSurvivorBoundIsTheRoundSnapshot(t *testing.T) {
	rail := newOwedRail(t)
	source := rail.source.Settings().Address
	prefix := []byte{24, 192, 0, 2}
	original := oldPeerSessions(t, rail, prefix)

	departure := departureGenerationBatch(source, rail.source.sourceOwner, original.generation)
	session := rail.destination.currentSession()
	owed, err := oweDeparture(rail.destination, session, &departure)
	require.NoError(t, err)
	require.Len(t, owed, 1, "the departure owes the source's route")
	record := owed[0]
	key := departureGenerationKey(t, rail.destination)
	entry, held := adjOutTestEntry(rail.destination)
	require.True(t, held)

	adapter := &reactorAPIAdapter{r: rail.r}
	admission := adapter.newRecoveryAdmission(t.Context(), rail.destination, session,
		[]recoveryAdmitKey{{key: key, record: record, epoch: record.epoch}})
	issued := admission.sources.issued
	later := recoveryFixtureSession(t, rail.source, rail.source.sendCtxID)
	rail.source.setState(PeerStateEstablished) // Started during the RIB lookup.
	newOriginal := captureRecoveryFixtureOrigin(t, rail.source, later, rail.source.sendCtxID)
	require.Greater(t, rail.source.forwardGeneration.Load(), issued)

	outcome := ribevents.RecoveryOutcome{Survivors: []ribevents.RecoveryRoute{{Source: newOriginal.address, SourceOwner: newOriginal.owner, SourceReceipt: newOriginal.receipt, SourcePolicyEpoch: rail.source.sourcePolicyEpoch.Load(),
		PathID: entry.owner.received + 1, AddPath: entry.owner.addPath, MessageID: 3}}}
	require.NoError(t, adapter.queueRecoveryOutcome(&outcome, 0, family.IPv4Unicast, admission, plugin.OperatorSender()))
	decided := &admission.keys[0]
	require.Equal(t, recoveryKeyReowed, decided.outcome,
		"a survivor whose session is not the snapshot's is re-owed with its source excluded")
	require.Equal(t, issued, decided.gone.generation,
		"the exclusion is bounded by the round's snapshot, not the clock at the decision")
	table := &rail.destination.adjOut
	table.mu.Lock()
	record.exclude(decided.gone)
	beforeLift := record.electableExcluded()
	table.mu.Unlock()
	require.Equal(t, original.owner, decided.gone.source, "the actual original owner is excluded")
	require.Contains(t, beforeLift, original.owner, "exclusion must exist before a later announcement lifts it")

	adjOutTestForward(t, rail.destination, rail.source, prefix, 4)
	endSession(rail.source)

	table.mu.Lock()
	excluded := record.electableExcluded()
	table.mu.Unlock()
	require.NotContains(t, excluded, rail.source.sourceOwner,
		"a session started during the lookup stays excluded by the unsendable survivor")
}

// VALIDATES: a recovery round's source snapshot is one cut of the generation
// clock: every receipt it holds is at or below its issued generation, and a
// session the source advances to while the snapshot is read is above it.
// PREVENTS: a session reaching Established after the snapshot read the clock
// and before it read the source's receipt, so the receipt names a session
// above the bound; were that session the survivor and to end under graceful
// restart before the decision, its exclusion would be lifted at once
// (adjOutOwed.announcerLifts) and a round wasted re-electing it.
// Method: one goroutine advances the source as fast as it can while the test
// takes snapshots; each snapshot must hold the source's receipt at or below
// issued.
func TestRecoverySourcesIsOneGenerationCut(t *testing.T) {
	rail := newOwedRail(t)
	adapter := &reactorAPIAdapter{r: rail.r}
	stop := make(chan struct{})
	advanced := make(chan struct{})
	go func() {
		defer close(advanced)
		for {
			select {
			case <-stop:
				return
			default:
				rail.source.advanceForwardGeneration()
			}
		}
	}()
	for range 2000 {
		sources := adapter.recoverySources()
		receipt, present := sources.receipts[rail.source]
		if !present {
			close(stop)
			<-advanced
			t.Fatal("the snapshot holds a receipt for every configured Peer")
		}
		if receipt.generation > sources.issued {
			close(stop)
			<-advanced
			t.Fatalf("receipt names generation %d above the snapshot's issued %d: "+
				"a session started after the clock read sits in the snapshot", receipt.generation, sources.issued)
		}
	}
	close(stop)
	<-advanced
}

// VALIDATES: a recovery admission holds the round's source snapshot, taken
// when the admission is built, as one cut of the generation clock: its issued
// generation is the clock read during the build, it holds the source's
// receipt at the source's current, nonzero generation, at or below issued,
// and a session the source starts afterward is above it.
// PREVENTS: an admission judged against a snapshot other than the one its
// round drained, a stale one, or none, whose zero bound every session of the
// source lifts, so an unsendable survivor's exclusion would never hold.
// Method: advance the source once so its receipt is nonzero, read the clock
// immediately before and after building the admission through
// newRecoveryAdmission, the only constructor, and bracket issued between the
// two reads; then advance the source.
func TestRecoveryAdmissionHoldsRoundSnapshot(t *testing.T) {
	rail := newOwedRail(t)
	rail.source.advanceForwardGeneration()
	adapter := &reactorAPIAdapter{r: rail.r}
	before := forwardGenerationClock.Load()
	admission := adapter.newRecoveryAdmission(t.Context(), rail.destination, nil, nil)
	after := forwardGenerationClock.Load()
	issued := admission.sources.issued
	require.LessOrEqual(t, before, issued, "issued is the clock read while the admission is built, not a stale value")
	require.LessOrEqual(t, issued, after, "issued is the clock read while the admission is built")
	receipt, present := admission.sources.receipts[rail.source]
	require.True(t, present, "the admission's snapshot holds a receipt for every configured Peer")
	require.Positive(t, receipt.generation, "the advanced source's receipt names its session")
	require.Equal(t, rail.source.forwardGeneration.Load(), receipt.generation,
		"the snapshot is taken when the admission is built")
	require.LessOrEqual(t, receipt.generation, issued,
		"every session a receipt names is at or below the snapshot's issued generation")
	rail.source.advanceForwardGeneration()
	require.Greater(t, rail.source.forwardGeneration.Load(), issued,
		"a session started after the admission is above its snapshot's bound")
}
