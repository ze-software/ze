// Design: docs/architecture/bgp/structural-forwarding.md -- announcement election at the final writer
// Related: session_ownership.go -- adjOutWrite.allow, oweAnnouncement
// Related: recovery_owed_worker_test.go -- owedRail
package reactor

import (
	"bytes"
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// adjOutTestSend writes body through target's final writer with item's
// authority, as a forward worker does. item.peer, item.session and
// item.rawBodies are filled here.
func adjOutTestSend(t *testing.T, target, source *Peer, item *fwdItem, body []byte) {
	t.Helper()
	s := target.currentSession()
	item.peer, item.session = target, s
	item.rawBodies = [][]byte{body}
	if item.authority == adjOutForwarded {
		original := wireu.NewWireUpdate(body, source.recvCtxID)
		// RFC 7911 Section 2: retain source paths before the final writer.
		require.NoError(t, prepareFwdProvenance(item, original, original, false))
		if item.provenance != nil {
			defer item.provenance.release()
		}
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.sentForward = item
	defer func() { s.sentForward = nil }()
	require.NoError(t, s.writeRawUpdateBody(body))
	require.NoError(t, s.flushWrites())
}

// adjOutTestForward is a forwarded announcement of raw from source, received
// in the message numbered message.
func adjOutTestForward(t *testing.T, target, source *Peer, raw []byte, message uint64) {
	t.Helper()
	item := fwdItem{receivedPeer: source, receivedGeneration: source.forwardGeneration.Load(), sourcePolicyEpoch: source.sourcePolicyEpoch.Load(),
		authority: adjOutForwarded, sourceMessageID: message}
	adjOutTestSend(t, target, source, &item, ownershipRailBody(family.IPv4Unicast, false, raw, 0))
}

// adjOutTestEntry copies the destination's only Adj-RIB-Out entry, or
// reports none.
func adjOutTestEntry(destination *Peer) (adjRIBOutRoute, bool) {
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	for _, entry := range table.routes {
		return *entry, true
	}
	return adjRIBOutRoute{}, false
}

// adjOutTestRecord copies the destination's only owed record.
func adjOutTestRecord(destination *Peer) (adjOutOwed, bool) {
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	for _, record := range table.owed {
		return *record, true
	}
	return adjOutOwed{}, false
}

// adjOutTestOtherSource adds a second established source to r, beside
// source, with the same encoding.
func adjOutTestOtherSource(r *Reactor, source *Peer) *Peer {
	other := NewPeer(&PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.3"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020303,
	})
	other.state.Store(int32(PeerStateEstablished))
	other.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{family.IPv4Unicast: true},
	})
	other.sendCtx.Store(source.sendCtx.Load())
	other.sendCtxID = source.sendCtxID
	other.recvCtxID = source.recvCtxID
	other.refreshForwardFacts()
	r.mu.Lock()
	r.peers[other.Settings().PeerKey()] = other
	r.mu.Unlock()
	return other
}

// TestAdjOutAnnouncementOwedWhenEntryExists proves Data Flow 3a at the final
// writer of a non-ADD-PATH destination (owner, Q-3). Method: real writer
// calls with forwarded and initial-replay authority, the destination's table
// read after each. With a registered recovery owner: the first announcement
// of P is written and installs its owner, generation and received message
// ID; the same uncontested owner's re-announcement is written and updates
// the message ID; another source's announcement is not written, owes P with
// an empty excluded set and contests the entry; the first owner's own
// re-announcement of the now contested P is not written and merges into the
// record as an ANNOUNCE mover with a current-announcer entry. With no
// recovery owner for the source, the contesting announcement is written as
// before Q-3. An initial-replay write installs the received message ID its
// batch carries (initial-source-message), and a forwarded initial replay
// without one is refused.
//
// VALIDATES: Data Flow 3a bullets 1 to 4, the initial-source-message chain.
// PREVENTS: forward-all (newest arrival wins) at an RS client.
func TestAdjOutAnnouncementOwedWhenEntryExists(t *testing.T) {
	p := []byte{24, 192, 0, 2}
	t.Run("recovery-owner", func(t *testing.T) {
		rail := newOwedRail(t)
		other := adjOutTestOtherSource(rail.r, rail.source)
		attachRecoveryOwner(t, rail.r, rail.source, other)

		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1, "the first route of P is written")
		entry, held := adjOutTestEntry(rail.destination)
		require.True(t, held)
		require.Same(t, rail.source, entry.owner.source)
		require.Equal(t, rail.source.forwardGeneration.Load(), entry.owner.generation)
		require.Equal(t, uint64(11), entry.owner.message)
		require.False(t, entry.contested)
		require.Empty(t, rail.owed())

		adjOutTestForward(t, rail.destination, rail.source, p, 12)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 2,
			"the sole source's re-announcement is written")
		entry, _ = adjOutTestEntry(rail.destination)
		require.Equal(t, uint64(12), entry.owner.message)
		require.Empty(t, rail.owed())

		adjOutTestForward(t, rail.destination, other, p, 21)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 2,
			"another source's route for P is a candidate, never written")
		entry, _ = adjOutTestEntry(rail.destination)
		require.Same(t, rail.source, entry.owner.source, "the entry keeps its owner until recovery writes")
		require.True(t, entry.contested)
		request := rail.request(t)
		require.Len(t, request.Keys, 1)
		require.Equal(t, p, request.Keys[0].NLRI)
		require.Empty(t, request.Keys[0].Excluded, "an announcement owes P with an empty excluded set")

		// The round is held: the first owner re-announces the contested P.
		adjOutTestForward(t, rail.destination, rail.source, p, 13)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 2,
			"an owner's re-announcement of a contested key is owed")
		record, owed := adjOutTestRecord(rail.destination)
		require.True(t, owed)
		require.True(t, record.announced, "the re-announcement is an ANNOUNCE mover")
		var current bool
		for _, announcer := range record.announcers {
			if announcer.source == rail.source && announcer.generation == rail.source.forwardGeneration.Load() {
				current = true
			}
		}
		require.True(t, current, "the re-announcing source has a current-announcer entry")
		rail.release <- nil
	})
	t.Run("no-recovery-owner", func(t *testing.T) {
		f := ownershipRailNew(t, nil)
		adjOutTestForward(t, f.destination, f.source, p, 11)
		adjOutTestForward(t, f.destination, f.other, p, 21)
		require.Len(t, recoveryWrittenUpdates(t, f.conn.written()), 2, "forward-all without a recovery owner")
		entry, _ := adjOutTestEntry(f.destination)
		require.Same(t, f.other, entry.owner.source)
		require.True(t, entry.contested, "the bit is kept with no recovery owner (A-5)")
	})
	t.Run("initial-replay-over-foreign-entry-owed", func(t *testing.T) {
		rail := newOwedRail(t)
		other := adjOutTestOtherSource(rail.r, rail.source)
		attachRecoveryOwner(t, rail.r, rail.source, other)
		adjOutTestForward(t, rail.destination, other, p, 21)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1, "the first route of P is written")

		owedRailInitialReplay(t, rail, 81)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1,
			"an initial replay over another source's entry is a candidate, never written")
		entry, _ := adjOutTestEntry(rail.destination)
		require.Same(t, other, entry.owner.source, "the entry keeps its owner until recovery writes")
		require.True(t, entry.contested)
		request := rail.request(t)
		require.Len(t, request.Keys, 1)
		require.Equal(t, p, request.Keys[0].NLRI)
		require.Empty(t, request.Keys[0].Excluded, "an initial replay owes P with an empty excluded set")
		rail.release <- nil
	})
	t.Run("initial-replay-over-owed-record-merges", func(t *testing.T) {
		rail := newOwedRail(t)
		other := adjOutTestOtherSource(rail.r, rail.source)
		attachRecoveryOwner(t, rail.r, rail.source, other)
		adjOutTestForward(t, rail.destination, other, p, 21)
		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1)
		rail.request(t) // The round is held while the replay arrives.

		owedRailInitialReplay(t, rail, 81)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1,
			"an initial replay of an owed key is merged, never written")
		record, owed := adjOutTestRecord(rail.destination)
		require.True(t, owed)
		require.True(t, record.announced, "the replay is an ANNOUNCE mover")
		var current bool
		for _, announcer := range record.announcers {
			if announcer.source == rail.source && announcer.generation == rail.source.forwardGeneration.Load() {
				current = true
			}
		}
		require.True(t, current, "the replaying source has a current-announcer entry")
		rail.release <- nil
	})
	t.Run("initial-replay-own-uncontested-refused", func(t *testing.T) {
		rail := newOwedRail(t)
		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1)

		owedRailInitialReplay(t, rail, 81)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1,
			"the entry's own uncontested owner's replay is refused, as at HEAD")
		entry, _ := adjOutTestEntry(rail.destination)
		require.Same(t, rail.source, entry.owner.source)
		require.Equal(t, uint64(11), entry.owner.message, "the refused replay installs nothing")
		require.False(t, entry.contested)
		require.Empty(t, rail.owed())
	})
	t.Run("originated-announcement-of-owed-key", func(t *testing.T) {
		rail := newOwedRail(t)
		rail.r.config = &Config{LocalAS: 65000}
		other := adjOutTestOtherSource(rail.r, rail.source)
		attachRecoveryOwner(t, rail.r, rail.source, other)
		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		adjOutTestForward(t, rail.destination, other, p, 21)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1)
		rail.request(t)
		require.Len(t, rail.owed(), 1)

		api := &reactorAPIAdapter{r: rail.r}
		sel := selector.ParseDefault(rail.destination.Settings().Address.String())
		require.NoError(t, api.AnnounceNLRIBatch(t.Context(), sel, adjOutBatch(syncOrderPrefix, "10.0.0.1"), plugin.OperatorSender()))
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 2, "an originated route is written over the owed key")
		entry, _ := adjOutTestEntry(rail.destination)
		require.Nil(t, entry.owner.source, "the entry is the local route's")
		require.Empty(t, rail.owed(), "the originated write removes the record")
		rail.release <- nil
	})
	t.Run("initial-replay-over-contested-entry-owed", func(t *testing.T) {
		rail := newOwedRail(t)
		other := adjOutTestOtherSource(rail.r, rail.source)
		attachRecoveryOwner(t, rail.r, rail.source, other)
		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		adjOutTestForward(t, rail.destination, other, p, 21)
		rail.request(t)
		// The RIB elects the revision the destination holds: nothing is
		// written, the record goes, and the entry stays contested.
		rail.release <- owedSurvivorMessage(rail.source.Settings().Address, rail.source.sourcePolicyEpoch.Load(), p, 11)
		lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the round decided P")
		entry, _ := adjOutTestEntry(rail.destination)
		require.Same(t, rail.source, entry.owner.source)
		require.True(t, entry.contested)
		before := len(recoveryWrittenUpdates(t, rail.conn.written()))

		owedRailInitialReplay(t, rail, 81)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), before,
			"the owner's replay of a contested key is a candidate, never written")
		request := rail.request(t)
		require.Equal(t, p, request.Keys[0].NLRI)
		require.Empty(t, request.Keys[0].Excluded)
		rail.release <- nil
	})
	t.Run("initial-replay-over-tombstone-owed", func(t *testing.T) {
		rail := newOwedRail(t)
		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		table := &rail.destination.adjOut
		table.mu.Lock()
		for key := range table.routes {
			delete(table.routes, key)
			table.tombstone(key)
		}
		table.mu.Unlock()
		before := len(recoveryWrittenUpdates(t, rail.conn.written()))

		owedRailInitialReplay(t, rail, 81)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), before,
			"a replay meeting a tombstone and no entry is a candidate, never written")
		_, held := adjOutTestEntry(rail.destination)
		require.False(t, held, "the owed replay installs no entry")
		request := rail.request(t)
		require.Equal(t, p, request.Keys[0].NLRI)
		require.Empty(t, request.Keys[0].Excluded)
		rail.release <- nil
	})
	t.Run("initial-replay-withdrawal-refused", func(t *testing.T) {
		rail := newOwedRail(t)
		session := rail.destination.currentSession()
		session.writeMu.Lock()
		session.initialReplay = 5
		session.writeMu.Unlock()
		rail.destination.initialSyncEOROwed.Store(true)
		item := fwdItem{receivedPeer: rail.source, receivedGeneration: rail.source.forwardGeneration.Load(), sourcePolicyEpoch: rail.source.sourcePolicyEpoch.Load(),
			authority: adjOutInitial, sourceMessageID: 81, initialReplay: 5, initialUpdate: true,
			expectedMessage: 91, initialSourceState: PeerStateEstablished}
		withdrawal := ownershipRailBody(family.IPv4Unicast, true, p, 0)
		adjOutTestSend(t, rail.destination, rail.source, &item, withdrawal)
		require.Empty(t, recoveryWrittenUpdates(t, rail.conn.written()),
			"an initial replay never withdraws a key the destination does not hold")
		require.Empty(t, rail.owed())

		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		before := len(recoveryWrittenUpdates(t, rail.conn.written()))
		item = fwdItem{receivedPeer: rail.source, receivedGeneration: rail.source.forwardGeneration.Load(), sourcePolicyEpoch: rail.source.sourcePolicyEpoch.Load(),
			authority: adjOutInitial, sourceMessageID: 81, initialReplay: 5, initialUpdate: true,
			expectedMessage: 91, initialSourceState: PeerStateEstablished}
		adjOutTestSend(t, rail.destination, rail.source, &item, withdrawal)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), before,
			"an initial replay never withdraws")
		entry, held := adjOutTestEntry(rail.destination)
		require.True(t, held, "the refused withdrawal deletes nothing")
		require.Equal(t, uint64(11), entry.owner.message)
		require.Empty(t, rail.owed(), "the refused withdrawal owes nothing")
	})
	t.Run("add-path-destination-written", func(t *testing.T) {
		rail := newOwedRail(t)
		other := adjOutTestOtherSource(rail.r, rail.source)
		attachRecoveryOwner(t, rail.r, rail.source, other)
		encoding := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
		ctxID, err := bgpctx.Registry.Register(encoding)
		require.NoError(t, err)
		rail.destination.sendCtx.Store(encoding)
		rail.destination.sendCtxID = ctxID
		rail.destination.session.setSendCtxID(ctxID)
		rail.source.recvCtxID, other.recvCtxID = ctxID, ctxID
		send := func(source *Peer, path byte, message uint64) {
			item := fwdItem{receivedPeer: source, receivedGeneration: source.forwardGeneration.Load(), sourcePolicyEpoch: source.sourcePolicyEpoch.Load(),
				authority: adjOutForwarded, sourceMessageID: message}
			raw := []byte{0, 0, 0, path, 24, 192, 0, 2}
			adjOutTestSend(t, rail.destination, source, &item, ownershipRailBody(family.IPv4Unicast, false, raw, 0))
		}
		send(rail.source, 1, 11)
		send(other, 2, 21)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 2,
			"an ADD-PATH destination receives every path, never an election")
		require.Empty(t, rail.owed())
	})
	t.Run("add-path-departure-record-not-a-candidate", func(t *testing.T) {
		// A departure owes an ADD-PATH destination's prefix under identifier
		// zero (oweDeparture), and zero is also a path identifier a
		// forwarded path can be bound to (RFC 7911 Section 2). The path's
		// announcement meets that record and is still written: at an
		// ADD-PATH destination no announcement is an election candidate.
		rail := newOwedRail(t)
		other := adjOutTestOtherSource(rail.r, rail.source)
		attachRecoveryOwner(t, rail.r, rail.source, other)
		encoding := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
		ctxID, err := bgpctx.Registry.Register(encoding)
		require.NoError(t, err)
		rail.destination.sendCtx.Store(encoding)
		rail.destination.sendCtxID = ctxID
		rail.destination.session.setSendCtxID(ctxID)
		rail.source.recvCtxID = ctxID
		// The path's identifier comes from the process-wide table
		// (fwdPathIDs), so the test does not predict it: the first write
		// installs the path, and the record goes at the key it was installed
		// under, wherever the table bound it (zero for the process's first
		// forwarded path). A production departure record follows a write the
		// same way: oweDeparture owes only a key a write installed.
		prefix := []byte{24, 100, 64, 255}
		raw := append([]byte{0, 0, 0, 0}, prefix...)
		first := fwdItem{receivedPeer: rail.source, receivedGeneration: rail.source.forwardGeneration.Load(), sourcePolicyEpoch: rail.source.sourcePolicyEpoch.Load(),
			authority: adjOutForwarded, sourceMessageID: 10}
		adjOutTestSend(t, rail.destination, rail.source, &first, ownershipRailBody(family.IPv4Unicast, false, raw, 0))
		table := &rail.destination.adjOut
		var key adjOutKey
		table.mu.Lock()
		for installed := range table.routes {
			if installed.addPath {
				key = installed
			}
		}
		table.mu.Unlock()
		require.True(t, key.addPath, "the first write installed the path at an ADD-PATH key")
		table.mu.Lock()
		table.owe(key, rail.destination.currentSession(), family.IPv4Unicast, prefix).exclude(adjOutExclusion{source: other.Settings().Address,
			generation: adjOutEverySession, departed: adjOutEverySession})
		table.mu.Unlock()

		// The path changes (another ORIGIN): its announcement meets the
		// record at its own key.
		item := fwdItem{receivedPeer: rail.source, receivedGeneration: rail.source.forwardGeneration.Load(), sourcePolicyEpoch: rail.source.sourcePolicyEpoch.Load(),
			authority: adjOutForwarded, sourceMessageID: 11}
		adjOutTestSend(t, rail.destination, rail.source, &item, ownershipRailBody(family.IPv4Unicast, false, raw, 1))
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 2,
			"the changed path is written over the departure's record")
		table.mu.Lock()
		_, held := table.routes[key]
		table.mu.Unlock()
		require.True(t, held, "the path is still installed at the record's key")
	})
	t.Run("staged-once-over-two-filter-passes", func(t *testing.T) {
		rail := newOwedRail(t)
		other := adjOutTestOtherSource(rail.r, rail.source)
		attachRecoveryOwner(t, rail.r, rail.source, other)
		held := []byte{24, 198, 51, 100}
		q := []byte{24, 203, 0, 113}
		// A round is held on another key, so no snapshot clears the marks.
		adjOutTestForward(t, rail.destination, rail.source, held, 1)
		adjOutTestForward(t, rail.destination, other, held, 2)
		rail.request(t)
		adjOutTestForward(t, rail.destination, other, p, 21)
		before := len(recoveryWrittenUpdates(t, rail.conn.written()))

		// One body: P is owed and removed, Q is written, so the writer
		// filters the body a second time into a scratch buffer.
		adjOutTestForward(t, rail.destination, rail.source, append(slices.Clone(p), q...), 11)
		written := recoveryWrittenUpdates(t, rail.conn.written())[before:]
		require.Len(t, written, 1)
		require.Equal(t, q, written[0].NLRI, "only Q reaches the wire")
		var record *adjOutOwed
		for _, owed := range rail.owed() {
			if bytes.Equal(owed.nlri, p) {
				record = owed
			}
		}
		require.NotNil(t, record, "P is owed")
		require.False(t, record.announced, "the announcement that created the record is no mover of it")
		require.Len(t, record.announcers, 1, "one current-announcer entry")
		rail.release <- nil
	})
	t.Run("initial-replay-installs-source-message", func(t *testing.T) {
		f := ownershipRailNew(t, nil)
		f.r.config = &Config{LocalAS: 65000}
		session := f.destination.currentSession()
		session.writeMu.Lock()
		session.initialReplay = 5
		session.writeMu.Unlock()
		f.destination.initialSyncEOROwed.Store(true)
		api := &reactorAPIAdapter{r: f.r}
		sel := selector.ParseDefault(f.destination.Settings().Address.String())
		batch := adjOutBatch(syncOrderPrefix, "10.0.0.1")
		batch.Replay = true
		batch.SentOwnerMessage = 91
		batch.InitialReplay = 5
		batch.InitialSourcePeer = f.source.Settings().Address.String()
		batch.InitialSourceID = uint32(f.source.SourceID())
		batch.InitialSourceOwner = f.source.sourceOwner
		batch.SourcePolicyEpoch = f.source.sourcePolicyEpoch.Load()

		missing := batch
		require.ErrorIs(t, api.AnnounceNLRIBatch(t.Context(), sel, missing, plugin.OperatorSender()), errAdjOutProvenance,
			"a forwarded initial replay without its source message is refused")
		require.Empty(t, recoveryWrittenUpdates(t, f.conn.written()))

		batch.InitialSourceMessage = 81
		require.NoError(t, api.AnnounceNLRIBatch(t.Context(), sel, batch, plugin.OperatorSender()))
		require.Len(t, recoveryWrittenUpdates(t, f.conn.written()), 1)
		entry, held := adjOutTestEntry(f.destination)
		require.True(t, held)
		require.Same(t, f.source, entry.owner.source)
		require.Equal(t, uint64(81), entry.owner.message, "the entry names the replayed revision")
	})
}

// adjOutTestTombstones counts the destination's tombstoned keys.
func adjOutTestTombstones(destination *Peer) int {
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	return len(table.tombstones)
}

// TestAdjOutContestedBitAndTombstone proves the contested bit and the
// tombstone at the final writer (round 11, I-3 and I-4; round 12, I-1 and
// I-3). Method: real writer calls on a non-ADD-PATH destination with a
// registered recovery owner and a gated RIB double, the table read after
// each. A recovery write sets the bit, so the new owner's own re-announcement
// is owed. An originated withdrawal of a source-owned, not-owed key leaves a
// tombstone, and the next forwarded announcement meets it and is owed; the
// deletion of an originated-owned entry leaves none. A recovery WITHDRAW
// decision leaves no tombstone, and a table reset clears bits and
// tombstones. A recovery announcement with no source is refused with
// errAdjOutProvenance and changes nothing.
//
// VALIDATES: Data Flow 3a, the bit and the tombstone; A-14's two exits.
// PREVENTS: a candidate written with no election after the entry it
// contested was deleted, and an unattributed recovery write installing an
// originated-looking owner.
func TestAdjOutContestedBitAndTombstone(t *testing.T) {
	p := syncOrderPrefixWire
	sel := func(rail *owedRail) *selector.Selector {
		return selector.ParseDefault(rail.destination.Settings().Address.String())
	}
	t.Run("recovery-write-sets-bit", func(t *testing.T) {
		rail := newOwedRail(t)
		other := adjOutTestOtherSource(rail.r, rail.source)
		attachRecoveryOwner(t, rail.r, rail.source, other)
		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		rail.forward(t, true, p) // The owner withdraws P: owed.
		rail.request(t)
		rail.release <- owedSurvivorMessage(other.Settings().Address, other.sourcePolicyEpoch.Load(), p, 21)
		lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the round decided P")
		entry, held := adjOutTestEntry(rail.destination)
		require.True(t, held)
		require.Same(t, other, entry.owner.source, "the recovery write installs the elected owner")
		require.True(t, entry.contested, "a recovery write sets the bit")
		before := len(recoveryWrittenUpdates(t, rail.conn.written()))

		adjOutTestForward(t, rail.destination, other, p, 22)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), before,
			"the elected owner's own re-announcement of a contested key is owed")
		rail.request(t)
		rail.release <- nil
	})
	t.Run("originated-withdrawal-leaves-tombstone", func(t *testing.T) {
		rail := newOwedRail(t)
		rail.r.config = &Config{LocalAS: 65000}
		other := adjOutTestOtherSource(rail.r, rail.source)
		attachRecoveryOwner(t, rail.r, rail.source, other)
		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		api := &reactorAPIAdapter{r: rail.r}
		require.NoError(t, api.WithdrawNLRIBatch(t.Context(), sel(rail), adjOutBatch(syncOrderPrefix, "10.0.0.1"), plugin.OperatorSender()))
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 2, "the originated withdrawal is written")
		_, held := adjOutTestEntry(rail.destination)
		require.False(t, held)
		require.Equal(t, 1, adjOutTestTombstones(rail.destination), "a source-owned deletion leaves a tombstone")

		adjOutTestForward(t, rail.destination, other, p, 21)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 2,
			"an announcement meeting a tombstone is owed, never written")
		request := rail.request(t)
		require.Equal(t, p, request.Keys[0].NLRI)
		rail.release <- nil
	})
	t.Run("originated-entry-leaves-no-tombstone", func(t *testing.T) {
		rail := newOwedRail(t)
		rail.r.config = &Config{LocalAS: 65000}
		api := &reactorAPIAdapter{r: rail.r}
		require.NoError(t, api.AnnounceNLRIBatch(t.Context(), sel(rail), adjOutBatch(syncOrderPrefix, "10.0.0.1"), plugin.OperatorSender()))
		require.NoError(t, api.WithdrawNLRIBatch(t.Context(), sel(rail), adjOutBatch(syncOrderPrefix, "10.0.0.1"), plugin.OperatorSender()))
		require.Zero(t, adjOutTestTombstones(rail.destination), "an originated entry's deletion leaves none")
		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 3, "the first forwarded route is written")
		require.Empty(t, rail.owed())
	})
	t.Run("withdraw-decision-leaves-no-tombstone", func(t *testing.T) {
		rail := newOwedRail(t)
		other := adjOutTestOtherSource(rail.r, rail.source)
		attachRecoveryOwner(t, rail.r, rail.source, other)
		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		rail.forward(t, true, p)
		rail.request(t)
		rail.release <- nil // WITHDRAW.
		lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the round decided P")
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 2, "the WITHDRAW answer is written")
		require.Zero(t, adjOutTestTombstones(rail.destination), "a WITHDRAW decision leaves no tombstone")

		adjOutTestForward(t, rail.destination, other, p, 21)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 3,
			"after a WITHDRAW decision the next route is a first arrival")
		require.Empty(t, rail.owed())
	})
	t.Run("table-reset-clears-bits-and-tombstones", func(t *testing.T) {
		rail := newOwedRail(t)
		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		table := &rail.destination.adjOut
		table.mu.Lock()
		for key := range table.routes {
			table.tombstone(key)
		}
		table.clearRoutes()
		table.mu.Unlock()
		require.Zero(t, adjOutTestTombstones(rail.destination))
		_, held := adjOutTestEntry(rail.destination)
		require.False(t, held)
	})
	t.Run("recovery-announcement-without-source-refused", func(t *testing.T) {
		rail := newOwedRail(t)
		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		table := &rail.destination.adjOut
		table.mu.Lock()
		for key := range table.routes {
			table.tombstone(key)
		}
		table.mu.Unlock()
		before := len(recoveryWrittenUpdates(t, rail.conn.written()))
		s := rail.destination.currentSession()
		item := fwdItem{peer: rail.destination, session: s, authority: adjOutRecovery}
		body := ownershipRailBody(family.IPv4Unicast, false, p, 0)
		item.rawBodies = [][]byte{body}
		s.writeMu.Lock()
		s.sentForward = &item
		err := s.writeRawUpdateBody(body)
		s.sentForward = nil
		s.writeMu.Unlock()
		require.ErrorIs(t, err, errAdjOutProvenance)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), before)
		entry, held := adjOutTestEntry(rail.destination)
		require.True(t, held)
		require.Same(t, rail.source, entry.owner.source, "no originated-looking owner is installed")
		require.False(t, entry.contested, "no bit is set")
		require.Equal(t, 1, adjOutTestTombstones(rail.destination), "the tombstone is untouched")
	})
}
