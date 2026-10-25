// Design: docs/architecture/bgp/structural-forwarding.md -- announcement election at the final writer
// Related: session_ownership.go -- adjOutWrite.allow, oweAnnouncement
// Related: recovery_owed_worker_test.go -- owedRail
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
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
	item := fwdItem{receivedPeer: source, receivedGeneration: source.forwardGeneration.Load(),
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
