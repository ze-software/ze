// Design: docs/architecture/update-building.md -- owed keys and the recovery worker
// Related: recovery_owed.go -- recoveryWorker
// Related: relay_recovery.go -- departure owes and the per-key admission
// Related: adj_rib_out.go -- adjOutOwed: excluded sources, epoch, movers
//
// VALIDATES: one owed record per key, merged by every later owe; a departure
// owes only a key its ended session still owns; the admission decides each
// key against the record it read, and leaves or re-owes a key whose question
// changed.
// PREVENTS: an old DOWN overwriting a reconnected source's route, a second owe
// replacing the record a round is deciding, and a replay reinstalling a
// withdrawn route while its replacement is being elected.
package reactor

import (
	"bytes"
	"context"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// owedFenceRecord is a copy of one owed record's fence fields.
type owedFenceRecord struct {
	record   *adjOutOwed
	epoch    uint64
	excluded []netip.Addr
}

// record reads the single owed record under the table mutex.
func (rail *owedRail) record(t *testing.T) owedFenceRecord {
	t.Helper()
	table := &rail.destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	require.Len(t, table.owed, 1)
	for _, record := range table.owed {
		return owedFenceRecord{record: record, epoch: record.epoch,
			excluded: append([]netip.Addr(nil), record.excluded...)}
	}
	return owedFenceRecord{}
}

// departure runs the departure command for the rail's source over prefix, as
// bgp-rs's DOWN handling does, and reports its result on the channel.
func (rail *owedRail) departure(t *testing.T, source netip.Addr, prefix string) chan error {
	t.Helper()
	batch := bgptypes.NLRIBatch{Family: family.IPv4Unicast, RecoverySource: source,
		NLRIs: []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix(prefix), 0, false)}}
	done := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	go func() {
		defer cancel()
		done <- (&reactorAPIAdapter{r: rail.r}).recoverNLRIBatch(ctx, batch,
			[]*Peer{rail.destination}, plugin.ProcessSender("bgp-rs"))
	}()
	return done
}

func (rail *owedRail) revision(t *testing.T) uint64 {
	t.Helper()
	table := &rail.destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	for _, entry := range table.routes {
		return entry.revision
	}
	t.Fatal("the destination holds no route")
	return 0
}

func departureResult(t *testing.T, done chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("the departure never completed")
		return nil
	}
}

// owedSurvivor answers one lookup with a single surviving candidate.
func owedSurvivor(source netip.Addr, p []byte) owedAnswer {
	attributes := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00, 0x40, 0x03, 0x04, 192, 0, 2, 1}
	return func(request ribevents.RecoveryRequest) []ribevents.RecoveryOutcome {
		return recoverySurvivorAnswer(request, p, ribevents.RecoveryRoute{Source: source,
			NLRI: p, Attributes: attributes, NextHop: []byte{192, 0, 2, 1}, MessageID: 7})
	}
}

// TestOwedKeyMergesDepartureAndWithdrawal proves a key keeps one record.
// Method: the owner's withdrawal owes P and the worker's lookup is held. A
// lifecycle removal of P merges with no epoch move; the owner's departure,
// once its session is no longer current, merges its source into the excluded
// set and moves the epoch. The held round's answer is then refused for the
// stale epoch, the next round asks with the source excluded, and its decision
// completes the departure's wait.
func TestOwedKeyMergesDepartureAndWithdrawal(t *testing.T) {
	rail := newOwedRail(t)
	p := []byte{24, 192, 0, 2}
	sourceAddr := rail.source.Settings().Address

	rail.forward(t, false, p)
	revision := rail.revision(t)
	rail.forward(t, true, p)
	first := rail.record(t)
	require.Empty(t, first.excluded)
	request := rail.request(t)
	require.Empty(t, request.Keys[0].Excluded)

	owedRailExpected(t, rail.destination, ownershipRailBody(family.IPv4Unicast, true, p, 0), revision)
	merged := rail.record(t)
	require.Same(t, first.record, merged.record, "a lifecycle removal merges into the record")
	require.Equal(t, first.epoch, merged.epoch, "a lifecycle removal moves no epoch")

	rail.source.forwardGeneration.Add(1) // The owner's session is no longer current.
	done := rail.departure(t, sourceAddr, "192.0.2.0/24")
	lowEventually(t, func() bool { return len(rail.record(t).excluded) == 1 }, "the departure merges")
	departed := rail.record(t)
	require.Same(t, first.record, departed.record, "a departure merges into the record")
	require.Equal(t, []netip.Addr{sourceAddr}, departed.excluded)
	require.NotEqual(t, first.epoch, departed.epoch, "a departure merge moves the epoch")

	rail.release <- nil // The held round answers the question asked before the merge.
	request = rail.request(t)
	require.Equal(t, []netip.Addr{sourceAddr}, request.Keys[0].Excluded,
		"the next round asks with the departed source excluded")
	require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1,
		"an admission for a stale epoch writes nothing")
	rail.release <- nil
	require.NoError(t, departureResult(t, done))
	updates := recoveryWrittenUpdates(t, rail.conn.written())
	require.Len(t, updates, 2)
	require.True(t, bytes.Equal(p, updates[1].WithdrawnRoutes))
	require.Empty(t, rail.owed())
}

// TestAdjOutDepartureOwesOnlyDepartedSession proves a departure owes a key
// only where the departed source owns it through a session that is no longer
// current, or through a binding a targeted down revoked. Method: the source
// owns P at the destination; a departure is run while that session is
// current, after a targeted down on the live session, for another source, for
// a prefix never sent, and after the session ended. Only the revoked and the
// ended cases owe P, and only they wait.
func TestAdjOutDepartureOwesOnlyDepartedSession(t *testing.T) {
	cases := []struct {
		name     string
		other    bool
		prefix   string
		ended    bool
		revoked  bool
		owesTheP bool
	}{
		{name: "current-session-keeps-its-route", prefix: "192.0.2.0/24"},
		{name: "live-source-revoked-owes", prefix: "192.0.2.0/24", revoked: true, owesTheP: true},
		{name: "other-source-owns-nothing", other: true, prefix: "192.0.2.0/24", ended: true},
		{name: "prefix-never-sent", prefix: "198.51.100.0/24", ended: true},
		{name: "earlier-session-owes", prefix: "192.0.2.0/24", ended: true, owesTheP: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rail := newOwedRail(t)
			p := []byte{24, 192, 0, 2}
			rail.forward(t, false, p)
			if tc.ended {
				rail.source.forwardGeneration.Add(1)
			}
			if tc.revoked {
				// A reload rebinds the route server on the kept session: the
				// targeted down ends no session, so only the revocation epoch
				// tells the departure that the live source no longer owns P.
				rail.source.unbindProcesses(nil, []string{"bgp-rs"})
			}
			departed := rail.source.Settings().Address
			if tc.other {
				departed = netip.MustParseAddr("198.51.100.7")
			}
			done := rail.departure(t, departed, tc.prefix)
			if !tc.owesTheP {
				require.NoError(t, departureResult(t, done))
				require.Empty(t, rail.owed(), "nothing is owed")
				require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1, "nothing is written")
				return
			}
			request := rail.request(t)
			require.Equal(t, p, request.Keys[0].NLRI)
			require.Equal(t, []netip.Addr{departed}, request.Keys[0].Excluded)
			rail.release <- nil
			require.NoError(t, departureResult(t, done))
			updates := recoveryWrittenUpdates(t, rail.conn.written())
			require.Len(t, updates, 2)
			require.True(t, bytes.Equal(p, updates[1].WithdrawnRoutes))
		})
	}
	// The ADD-PATH half: the departure owes the prefix, the RIB names every
	// identifier it sent for the source (the reconnect's included), and the
	// writer withdraws only the identifiers the ended session owns.
	t.Run("add-path-reconnect-keeps-newer-identifier", func(t *testing.T) {
		r, destination, conn := recoveryAddPathRail(t)
		ended := [][]byte{{0, 0, 0, 0, 24, 192, 0, 2}, {0, 0, 0, 7, 24, 192, 0, 2}}
		newer := []byte{0, 0, 0, 9, 24, 192, 0, 2}
		var seed [][]byte
		for _, raw := range ended {
			// RFC 4271 Section 4.3 and RFC 7911 Section 3.
			seed = append(seed, ownershipRailBody(family.IPv4Unicast, false, raw, 0))
		}
		failed := recoveryAdvertiseFailed(t, r, destination, seed...)
		failed.setState(PeerStateEstablished) // The source reconnects: a new generation.
		require.NoError(t, ownershipWriterForward(t, destination, failed,
			ownershipRailBody(family.IPv4Unicast, false, newer, 0), false))
		setup := len(conn.written())
		recoveryOwnerStart(t, r, destination)
		owner := ribevents.PublishRecovery(recoveryAddPathAnswer(map[string][][]byte{
			string([]byte{24, 192, 0, 2}): {ended[0], ended[1], newer}}))
		t.Cleanup(owner.Close)
		require.NoError(t, recoveryDepart(t, r, destination, failed.Settings().Address, "192.0.2.0/24"))
		var got [][]byte
		for _, written := range recoveryWrittenUpdates(t, conn.written()[setup:]) {
			require.Empty(t, written.NLRI)
			if len(written.WithdrawnRoutes) != 0 {
				got = append(got, written.WithdrawnRoutes)
			}
		}
		require.ElementsMatch(t, ended, got,
			"only the ended session's identifiers are withdrawn; the reconnect's newer path stays")
		require.Empty(t, owedRecordCount(destination), "the decided key leaves the owed set")
	})
}

// owedRecordCount counts the destination's owed records.
func owedRecordCount(destination *Peer) int {
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	return len(table.owed)
}

// TestOwedKeyFenceInterleavings proves the admission decides each key against
// what happened since its round's snapshot. Method: (a) the chosen candidate
// withdraws after the snapshot, so its own withdrawal is recorded as a mover
// and the admission leaves the key owed and writes nothing; (b) the chosen
// candidate's source does not resolve, so the key is re-owed with that source
// excluded. In both, the next round decides the key.
func TestOwedKeyFenceInterleavings(t *testing.T) {
	t.Run("chosen-candidate-withdrew", func(t *testing.T) {
		rail := newOwedRail(t)
		p := []byte{24, 192, 0, 2}
		rail.forward(t, false, p)
		rail.forward(t, true, p)
		rail.request(t)
		rail.forward(t, true, p) // A withdrawal of the candidate the round elects.
		mover := rail.record(t)
		rail.release <- owedSurvivor(rail.source.Settings().Address, p)
		request := rail.request(t)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1,
			"a candidate that withdrew after the snapshot is not written")
		require.Same(t, mover.record, rail.record(t).record, "the key is left owed")
		require.Empty(t, request.Keys[0].Excluded, "a mover excludes no source")
		rail.release <- nil
		lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the next round decides")
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 2)
	})
	// (j): the chosen survivor's source session was replaced after the
	// round's receipt, the address reused. Its bytes are never written; the
	// key is owed again with that source excluded and a moved epoch.
	t.Run("survivor-session-replaced-reowes", func(t *testing.T) {
		r, source, destination, conn, _ := newSyncOrderRail(t)
		destination.sendingInitialRoutes.Store(0)
		session := destination.currentSession()
		key, record := recoveryOweForTest(t, destination, session, syncOrderPrefixWire)
		epoch := record.epoch
		generation := source.forwardGeneration.Load()
		admission := &recoveryAdmission{peer: destination, session: session, ctx: t.Context(),
			done:    make(chan struct{}),
			sources: map[*Peer]recoverySourceReceipt{source: {generation: generation, cut: source.receiveCut()}},
			keys: []recoveryAdmitKey{{key: key, record: record, epoch: epoch,
				candidate: adjOutPath{source: source}}}}
		admission.items = []fwdItem{{peer: destination, rawBodies: [][]byte{syncOrderAnnounceBody},
			receivedPeer: source, receivedGeneration: generation}}
		admission.mark(0)
		source.forwardGeneration.Add(1) // The source's session is replaced.
		r.fwdPool.safeBatchHandle(fwdKey{peerAddr: destination.Settings().PeerKey()},
			[]fwdItem{{peer: destination, recovery: admission}})
		<-admission.done
		require.Empty(t, conn.written(), "a stale candidate is never written")
		require.Equal(t, recoveryKeyReowed, admission.keys[0].outcome)
		require.Same(t, record, recoveryOwedRecord(destination, key), "the key stays owed")
		table := &destination.adjOut
		table.mu.Lock()
		excluded, moved := slices.Clone(record.excluded), record.epoch != epoch
		table.mu.Unlock()
		require.Equal(t, []netip.Addr{source.Settings().Address}, excluded)
		require.True(t, moved, "the re-owe moves the epoch")
	})
	// (k): the chosen survivor's live source accepted an UPDATE of another
	// prefix between the round's cut and the admission. That traffic is not a
	// fence (AC-15): an UPDATE of the key itself reaches this writer as a
	// mover or as a later write, so the survivor is written, the key is
	// decided by this round, and no source is excluded.
	t.Run("survivor-source-advanced-is-written", func(t *testing.T) {
		r, source, destination, conn, _ := newSyncOrderRail(t)
		destination.sendingInitialRoutes.Store(0)
		session := destination.currentSession()
		key, record := recoveryOweForTest(t, destination, session, syncOrderPrefixWire)
		epoch := record.epoch
		source.acceptedReceive(1).complete(1)
		generation := source.forwardGeneration.Load()
		admission := &recoveryAdmission{peer: destination, session: session, ctx: t.Context(),
			done:    make(chan struct{}),
			sources: map[*Peer]recoverySourceReceipt{source: {generation: generation, cut: source.receiveCut()}},
			keys: []recoveryAdmitKey{{key: key, record: record, epoch: epoch,
				candidate: adjOutPath{source: source}}}}
		admission.items = []fwdItem{{peer: destination, session: session, authority: adjOutRecovery,
			rawBodies: [][]byte{syncOrderAnnounceBody}, receivedPeer: source, receivedGeneration: generation}}
		admission.mark(0)
		source.acceptedReceive(2) // A later UPDATE on the same live session.
		r.fwdPool.safeBatchHandle(fwdKey{peerAddr: destination.Settings().PeerKey()},
			[]fwdItem{{peer: destination, recovery: admission}})
		<-admission.done
		require.Equal(t, recoveryKeyWrite, admission.keys[0].outcome)
		require.NoError(t, admission.err)
		require.NotEmpty(t, conn.written(), "the live survivor is written")
		require.Nil(t, recoveryOwedRecord(destination, key), "the key is decided by this round")
		table := &destination.adjOut
		table.mu.Lock()
		excluded := slices.Clone(record.excluded)
		table.mu.Unlock()
		require.Empty(t, excluded, "a live source is never excluded")
	})
	// (l): the same advance seen by queueRecovery, before any section is
	// staged. The round writes the survivor and asks the RIB nothing more.
	t.Run("survivor-source-advanced-before-staging-writes", func(t *testing.T) {
		rail := newOwedRail(t)
		p := []byte{24, 192, 0, 2}
		rail.source.acceptedReceive(1).complete(1)
		rail.forward(t, false, p)
		rail.forward(t, true, p)
		rail.request(t)
		rail.source.acceptedReceive(2).complete(2) // After the round's cut.
		rail.release <- owedSurvivor(rail.source.Settings().Address, p)
		lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the round decides the key")
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 2, "the survivor is written")
		select {
		case request := <-rail.requests:
			t.Fatalf("the decided key was asked again: %+v", request)
		case <-time.After(100 * time.Millisecond):
		}
	})
	// (q): the chosen candidate re-announces the owed key after the round's
	// snapshot. The writer withholds those bytes for election rather than
	// allowing them to bypass the held decision. The drained round may write
	// its older revision, but the announcement keeps the same key owed with
	// a new epoch. The next round must publish the newer revision.
	t.Run("chosen-candidate-reannounced", func(t *testing.T) {
		rail := newOwedRail(t)
		p := []byte{24, 192, 0, 2}
		rail.forward(t, false, p)
		rail.forward(t, true, p)
		rail.request(t)
		newer := ownershipRailBody(family.IPv4Unicast, false, p, 2)
		require.NoError(t, ownershipWriterForward(t, rail.destination, rail.source, newer, false))
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1,
			"the re-announcement cannot bypass the outstanding election")
		mover := rail.record(t)
		rail.release <- owedSurvivor(rail.source.Settings().Address, p) // The older revision.
		request := rail.request(t)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 2,
			"the drained round may decide before the newer revision is elected")
		after := rail.record(t)
		require.Same(t, mover.record, after.record, "the announcement leaves the same key owed")
		require.NotEqual(t, mover.epoch, after.epoch, "the newer announcement requires another election")
		require.Empty(t, request.Keys[0].Excluded, "a re-announcement excludes no source")
		// RFC 4271 Section 5.1.4: MULTI_EXIT_DISC, optional non-transitive,
		// marks the newer revision the next round answers with.
		med := []byte{0x80, 0x04, 0x04, 0, 0, 0, 2}
		newerAttrs := append([]byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00, 0x40, 0x03, 0x04, 192, 0, 2, 1}, med...)
		source := rail.source.Settings().Address
		rail.release <- func(request ribevents.RecoveryRequest) []ribevents.RecoveryOutcome {
			return recoverySurvivorAnswer(request, p, ribevents.RecoveryRoute{Source: source,
				NLRI: p, Attributes: newerAttrs, NextHop: []byte{192, 0, 2, 1}, MessageID: 8})
		}
		lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the next round decides")
		updates := recoveryWrittenUpdates(t, rail.conn.written())
		require.Len(t, updates, 3)
		require.Equal(t, p, updates[2].NLRI)
		for _, code := range []attribute.AttributeCode{
			attribute.AttrOrigin, attribute.AttrASPath, attribute.AttrNextHop, attribute.AttrMED,
		} {
			_, _, want, present := attribute.AttrFind(newerAttrs, code)
			require.True(t, present)
			_, _, got, found := attribute.AttrFind(updates[2].PathAttributes, code)
			require.True(t, found, "the elected revision retains attribute %d", code)
			require.Equal(t, want, got, "the final advertised attribute %d is from the newer revision", code)
		}
		for _, update := range updates {
			require.Empty(t, update.WithdrawnRoutes, "re-election never transiently withdraws the surviving key")
		}
	})
	t.Run("survivor-gone-reowes", func(t *testing.T) {
		rail := newOwedRail(t)
		p := []byte{24, 192, 0, 2}
		gone := netip.MustParseAddr("203.0.113.99")
		rail.forward(t, false, p)
		rail.forward(t, true, p)
		before := rail.record(t)
		rail.request(t)
		rail.release <- owedSurvivor(gone, p)
		request := rail.request(t)
		require.Equal(t, []netip.Addr{gone}, request.Keys[0].Excluded,
			"a survivor that no longer resolves is excluded from the next round")
		after := rail.record(t)
		require.Same(t, before.record, after.record)
		require.NotEqual(t, before.epoch, after.epoch, "the re-owe moves the epoch")
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1)
		rail.release <- nil
		lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the next round decides")
	})
}

// TestOwedKeyRefreshDuringRoundKeepsRecord proves a bgp-rib replay of an owed
// key is refused and counted. Method: the owner withdraws P and its decision
// is held; a replay of the owner's route (ROUTE-REFRESH, clear bgp rib out)
// reaches the writer. Nothing is written, the record stays, and the
// destination's owed-refusal counter moves by one. The decision then lands.
func TestOwedKeyRefreshDuringRoundKeepsRecord(t *testing.T) {
	rail := newOwedRail(t)
	p := []byte{24, 192, 0, 2}
	rail.forward(t, false, p)
	revision := rail.revision(t)
	rail.forward(t, true, p)
	owed := rail.record(t)
	rail.request(t)
	before := rail.destination.owedRefused.Load()

	owedRailExpected(t, rail.destination, ownershipRailBody(family.IPv4Unicast, false, p, 0), revision)
	require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1, "the replay is refused")
	require.Same(t, owed.record, rail.record(t).record, "the record stays")
	require.Equal(t, before+1, rail.destination.owedRefused.Load(), "the refusal is counted once")

	rail.release <- nil
	lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the decision lands")
	require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 2)
}
