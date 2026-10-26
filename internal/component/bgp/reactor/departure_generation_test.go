// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
// Related: relay_recovery.go -- oweDeparture, departedOwner, unsendableSurvivor
// Related: adj_rib_out.go -- adjOutExclusion, electableExcluded, spares
// Related: session_ownership.go -- recoverySpares

package reactor

import (
	"context"
	"math"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// adjOutEverySession is a departure bound above every generation a test
// makes: a departure that answers whichever session of its source wrote a key.
// The engine never issues it; a plugin's departure names one ended session.
const adjOutEverySession = math.MaxUint64

// departureGenerationBatch is a departure of source over 192.0.2.0/24 that
// answers the end of the session at generation.
func departureGenerationBatch(source netip.Addr, generation uint64) bgptypes.NLRIBatch {
	return bgptypes.NLRIBatch{Family: family.IPv4Unicast, RecoverySource: source,
		RecoveryGeneration: generation,
		NLRIs: []nlri.NLRI{nlri.NewINET(family.IPv4Unicast,
			netip.MustParsePrefix("192.0.2.0/24"), 0, false)}}
}

// departureGenerationKey returns the destination's only Adj-RIB-Out key.
func departureGenerationKey(t *testing.T, destination *Peer) adjOutKey {
	t.Helper()
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	require.Len(t, table.routes, 1)
	for key := range table.routes {
		return key
	}
	return adjOutKey{}
}

// PREVENTS: a departure queued behind a reconnect owing the newer session's
// route after that session ended under graceful restart, which withdrew a
// route RFC 4724 Section 4.2 says the speaker MUST retain. Method: the source
// writes the key on session 1, re-writes it on session 2, and session 2 ends;
// the departure of session 1, run only now, owes nothing, while the departure
// of session 2 owes the key, and a departure that names no session is refused.
func TestStaleDepartureSparesNewerSessionRoute(t *testing.T) {
	rail := newOwedRail(t)
	source := rail.source.Settings().Address
	prefix := []byte{24, 192, 0, 2}
	rail.source.advanceForwardGeneration() // Session 1 reaches Established.
	adjOutTestForward(t, rail.destination, rail.source, prefix, 1)
	sessionOne := rail.source.forwardGeneration.Load()
	rail.source.advanceForwardGeneration() // Session 2 reaches Established.
	adjOutTestForward(t, rail.destination, rail.source, prefix, 2)
	rail.source.advanceForwardGeneration() // Session 2 ends; GR retains its routes.

	entry, held := adjOutTestEntry(rail.destination)
	require.True(t, held)
	require.Greater(t, entry.owner.generation, sessionOne, "session 2 owns the key")
	session := rail.destination.currentSession()

	stale := departureGenerationBatch(source, sessionOne)
	owed, err := oweDeparture(rail.destination, session, &stale)
	require.NoError(t, err)
	require.Empty(t, owed, "session 1's departure owes session 2's retained route")
	rail.destination.adjOut.mu.Lock()
	owedKeys := len(rail.destination.adjOut.owed)
	rail.destination.adjOut.mu.Unlock()
	require.Zero(t, owedKeys, "session 1's departure left a record for session 2's route")

	unnamed := departureGenerationBatch(source, 0)
	_, err = oweDeparture(rail.destination, session, &unnamed)
	require.Error(t, err, "a departure naming no session must be refused, not defaulted")

	current := departureGenerationBatch(source, entry.owner.generation)
	owed, err = oweDeparture(rail.destination, session, &current)
	require.NoError(t, err)
	require.Len(t, owed, 1, "session 2's own departure owes its route")
}

// PREVENTS: a source that re-announced an owed key on a new session, then
// lost that session under graceful restart before the record was decided,
// staying excluded from the election and its retained route withdrawn or
// replaced (RFC 4724 Section 4.2). Method: session 1's departure owes the
// key; session 2 re-announces it (owed, not written) and ends. The round
// still asks the RIB with the source electable, and the source's retained
// survivor, which cannot be sent without a session, is held on the source's
// path the destination already has rather than re-owed with the source
// excluded; a path the destination does not hold is re-owed.
func TestReannouncedThenRestartedSourceStaysElectable(t *testing.T) {
	rail := newOwedRail(t)
	source := rail.source.Settings().Address
	prefix := []byte{24, 192, 0, 2}
	rail.source.advanceForwardGeneration() // Session 1 reaches Established.
	adjOutTestForward(t, rail.destination, rail.source, prefix, 1)
	rail.source.advanceForwardGeneration() // Session 1 ends.
	sessionOne := rail.source.forwardGeneration.Load()
	session := rail.destination.currentSession()

	departure := departureGenerationBatch(source, sessionOne)
	owed, err := oweDeparture(rail.destination, session, &departure)
	require.NoError(t, err)
	require.Len(t, owed, 1, "session 1's departure owes its route")
	record := owed[0]

	rail.source.advanceForwardGeneration() // Session 2 reaches Established.
	adjOutTestForward(t, rail.destination, rail.source, prefix, 2)
	rail.source.advanceForwardGeneration() // Session 2 ends; GR retains its routes.

	table := &rail.destination.adjOut
	table.mu.Lock()
	excluded := record.electableExcluded()
	table.mu.Unlock()
	require.NotContains(t, excluded, source,
		"the source's retained session-2 route is skipped by the election")

	key := departureGenerationKey(t, rail.destination)
	entry, held := adjOutTestEntry(rail.destination)
	require.True(t, held)
	require.Equal(t, uint64(1), entry.owner.message, "the owed key keeps session 1's path until decided")
	sources := (&reactorAPIAdapter{r: rail.r}).recoverySources()
	admission := newRecoveryAdmission(t.Context(), rail.destination, nil, sources, nil)

	retained := ribevents.RecoveryRoute{Source: source, PathID: entry.owner.received,
		AddPath: entry.owner.addPath, MessageID: 2}
	decided := recoveryAdmitKey{key: key, record: record}
	admission.unsendableSurvivor(&decided, &retained)
	require.NotEqual(t, recoveryKeyReowed, decided.outcome,
		"the held retained route is re-owed with its source excluded")
	require.True(t, adjOutSameOwner(decided.candidate, entry.owner), "the held route is the candidate")

	other := retained
	other.PathID = entry.owner.received + 1
	elsewhere := recoveryAdmitKey{key: key, record: record}
	admission.unsendableSurvivor(&elsewhere, &other)
	require.Equal(t, recoveryKeyReowed, elsewhere.outcome,
		"a path the destination does not hold cannot be sent from a sessionless source")
	// The bound is the newest generation issued when the round's source
	// snapshot was taken, the snapshot the survivor was judged against.
	require.Equal(t, source, elsewhere.gone.source)
	require.Zero(t, elsewhere.gone.departed, "an unsendable survivor is no departure")
	require.GreaterOrEqual(t, elsewhere.gone.generation, rail.source.forwardGeneration.Load(),
		"the exclusion covers every session the snapshot knew")
	require.Equal(t, sources.issued, elsewhere.gone.generation,
		"the exclusion is bounded by the round's snapshot, not the clock at the decision")
}

// PREVENTS: the ADD-PATH writer withdrawing an identifier a newer session of
// the departed source wrote, after that session also ended under graceful
// restart, which drops a route RFC 4724 Section 4.2 says the speaker MUST
// retain. The ownership check alone cannot spare it: no session of the source
// is current. Method: the source writes identifiers 0 and 7 on session 1,
// session 2 writes identifier 9 and ends, and the departure of session 1 runs
// only now through the departure command. The RIB names all three
// identifiers; the writer withdraws 0 and 7 and keeps 9, which the record's
// departure bound does not cover.
func TestStaleAddPathDepartureSparesNewerSessionIdentifier(t *testing.T) {
	r, destination, conn := recoveryAddPathRail(t)
	ended := [][]byte{{0, 0, 0, 0, 24, 192, 0, 2}, {0, 0, 0, 7, 24, 192, 0, 2}}
	newer := []byte{0, 0, 0, 9, 24, 192, 0, 2}
	failed, _ := newAnnouncePeer(t, "192.0.2.10")
	failed.recvCtxID = destination.sendContextID()
	r.peers[failed.Settings().PeerKey()] = failed
	failed.advanceForwardGeneration() // Session 1 reaches Established.
	sessionOne := failed.forwardGeneration.Load()
	for _, raw := range ended {
		// RFC 4271 Section 4.3 and RFC 7911 Section 3.
		require.NoError(t, ownershipWriterForward(t, destination, failed,
			ownershipRailBody(family.IPv4Unicast, false, raw, 0), false))
	}
	failed.setState(PeerStateStopped)     // Session 1 ends.
	failed.setState(PeerStateEstablished) // Session 2 reaches Established.
	require.NoError(t, ownershipWriterForward(t, destination, failed,
		ownershipRailBody(family.IPv4Unicast, false, newer, 0), false))
	failed.setState(PeerStateStopped) // Session 2 ends; GR retains its routes.
	require.Greater(t, failed.forwardGeneration.Load(), sessionOne)
	setup := len(conn.written())
	recoveryOwnerStart(t, r, destination)
	owner := ribevents.PublishRecovery(recoveryAddPathAnswer(map[string][][]byte{
		string([]byte{24, 192, 0, 2}): {ended[0], ended[1], newer}}))
	t.Cleanup(owner.Close)

	departure := recoveryDepartureBatch(failed.Settings().Address, "192.0.2.0/24")
	departure.RecoveryGeneration = sessionOne
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, (&reactorAPIAdapter{r: r}).recoverNLRIBatch(ctx, departure,
		[]*Peer{destination}, plugin.ProcessSender("bgp-rs")))

	var got [][]byte
	for _, written := range recoveryWrittenUpdates(t, conn.written()[setup:]) {
		require.Empty(t, written.NLRI)
		if len(written.WithdrawnRoutes) != 0 {
			got = append(got, written.WithdrawnRoutes)
		}
	}
	require.ElementsMatch(t, ended, got,
		"session 1's departure withdraws session 1's identifiers and keeps session 2's retained one")
	require.Empty(t, owedRecordCount(destination), "the decided key leaves the owed set")
}
