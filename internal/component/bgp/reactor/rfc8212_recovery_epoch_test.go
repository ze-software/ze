// Design: docs/architecture/bgp/structural-forwarding.md -- the source-policy epoch at recovery
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on; rows RFC8212-3-1/3-2 are {gap}, which refuse a proof tag until the spec closes them (AC-11).
// Related: relay_recovery.go -- queueRecoveryOutcome, admit, settle
// Related: session_paths_limit_elect.go -- queuePathsLimitElection

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// rfc8212RecoveryAttributes are the ORIGIN, AS_PATH and NEXT_HOP of every
// survivor these tests elect.
var rfc8212RecoveryAttributes = []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00, 0x40, 0x03, 0x04, 192, 0, 2, 1}

// rfc8212SupersedeEpoch records an import admission for peer, then supersedes
// it, as a policy change does, and returns the superseded epoch.
func rfc8212SupersedeEpoch(peer *Peer) uint64 {
	admitted := peer.advanceSourcePolicyEpoch()
	peer.advanceSourcePolicyEpoch()
	return admitted
}

// rfc8212RecoverySurvivor is source's route for p admitted at epoch.
func rfc8212RecoverySurvivor(source *Peer, epoch uint64, p []byte, message uint64) ribevents.RecoveryRoute {
	return ribevents.RecoveryRoute{Source: source.Settings().Address, SourcePolicyEpoch: epoch,
		NLRI: p, Attributes: rfc8212RecoveryAttributes, NextHop: []byte{192, 0, 2, 1}, MessageID: message}
}

// rfc8212RecoverySource adds an established source at address to r, with
// source's encoding, as adjOutTestOtherSource does for its fixed address.
func rfc8212RecoverySource(r *Reactor, source *Peer, address string, routerID uint32) *Peer {
	peer := NewPeer(&PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr(address),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   routerID,
	})
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{family.IPv4Unicast: true},
	})
	peer.sendCtx.Store(source.sendCtx.Load())
	peer.sendCtxID = source.sendCtxID
	peer.recvCtxID = source.recvCtxID
	peer.refreshForwardFacts()
	r.mu.Lock()
	r.peers[peer.Settings().PeerKey()] = peer
	r.mu.Unlock()
	return peer
}

// TestRFC8212RecoveryStaleOnlySurvivorWithdraws proves a departed source's
// route is withdrawn when the only survivor the RIB names was admitted under a
// superseded import policy. Goal: RFC 4271 Section 9.2 still holds under the
// RFC 8212 epoch fence; the route never stays advertised and the key never
// stays owed. Method: A's route for P is installed at C, A departs, and the
// RIB answers with B's route at B's superseded epoch. The departure must
// complete, C must receive exactly the withdrawal of P, and nothing stays owed.
//
// VALIDATES: queueRecoveryOutcome skips a policy-stale survivor.
// PREVENTS: a stale survivor left owed forever while the departed route stays.
func TestRFC8212RecoveryStaleOnlySurvivorWithdraws(t *testing.T) {
	rail := newOwedRail(t)
	stale := adjOutTestOtherSource(rail.r, rail.source)
	attachRecoveryOwner(t, rail.r, rail.source, stale)
	p := []byte{24, 192, 0, 2}
	address := rail.source.Settings().Address
	adjOutTestForward(t, rail.destination, rail.source, p, 11)
	before := len(recoveryWrittenUpdates(t, rail.conn.written()))
	staleEpoch := rfc8212SupersedeEpoch(stale)

	rail.source.advanceForwardGeneration() // A's session ended.
	done := rail.departure(t, address, "192.0.2.0/24")
	require.Equal(t, []netip.Addr{address}, rail.request(t).Keys[0].Excluded,
		"the departed source is excluded")
	rail.release <- func(request ribevents.RecoveryRequest) []ribevents.RecoveryOutcome {
		return recoverySurvivorAnswer(request, p, rfc8212RecoverySurvivor(stale, staleEpoch, p, 21))
	}

	require.NoError(t, departureResult(t, done))
	written := recoveryWrittenUpdates(t, rail.conn.written())[before:]
	require.Len(t, written, 1, "exactly one UPDATE: the withdrawal")
	require.Empty(t, written[0].NLRI, "the stale survivor is never announced")
	require.Equal(t, p, written[0].WithdrawnRoutes, "the departed route is withdrawn")
	require.Empty(t, rail.owed(), "the key is decided")
	_, held := adjOutTestEntry(rail.destination)
	require.False(t, held, "the destination holds no route for P")
}

// TestRFC8212RecoveryStaleSurvivorYieldsToCurrent proves a policy-stale
// survivor ranked first gives way to the next eligible one. Goal: the
// replacement for a departed route is the best survivor whose import
// admission is current. Method: A's route for P is installed at C, A
// departs, and the RIB answers B (superseded epoch) then D (current epoch).
// C must receive D's announcement of P, and hold D's route.
//
// VALIDATES: queueRecoveryOutcome tries the next candidate after a stale one.
// PREVENTS: the stale first survivor blocking the eligible replacement.
func TestRFC8212RecoveryStaleSurvivorYieldsToCurrent(t *testing.T) {
	rail := newOwedRail(t)
	stale := adjOutTestOtherSource(rail.r, rail.source)
	current := rfc8212RecoverySource(rail.r, rail.source, "10.0.0.4", 0x01020304)
	attachRecoveryOwner(t, rail.r, rail.source, stale, current)
	p := []byte{24, 192, 0, 2}
	address := rail.source.Settings().Address
	adjOutTestForward(t, rail.destination, rail.source, p, 11)
	before := len(recoveryWrittenUpdates(t, rail.conn.written()))
	staleEpoch := rfc8212SupersedeEpoch(stale)
	currentEpoch := current.advanceSourcePolicyEpoch()

	rail.source.advanceForwardGeneration() // A's session ended.
	done := rail.departure(t, address, "192.0.2.0/24")
	rail.request(t)
	rail.release <- func(request ribevents.RecoveryRequest) []ribevents.RecoveryOutcome {
		outcomes := recoverySurvivorAnswer(request, p, rfc8212RecoverySurvivor(stale, staleEpoch, p, 21))
		for i := range outcomes {
			outcomes[i].Survivors = append(outcomes[i].Survivors, rfc8212RecoverySurvivor(current, currentEpoch, p, 31))
		}
		return outcomes
	}

	require.NoError(t, departureResult(t, done))
	written := recoveryWrittenUpdates(t, rail.conn.written())[before:]
	require.Len(t, written, 1, "exactly one UPDATE: the replacement")
	require.Equal(t, p, written[0].NLRI, "the current survivor replaces the departed route")
	entry, held := adjOutTestEntry(rail.destination)
	require.True(t, held)
	require.Same(t, current, entry.owner.source, "the destination holds the current survivor's route")
	require.Equal(t, uint64(31), entry.owner.message)
	require.Empty(t, rail.owed(), "the key is decided")
}

// TestRFC8212PathsLimitStaleSurvivorKeepsDepartureWithdrawals proves a
// PATHS-LIMIT election whose only survivor is policy-stale still withdraws the
// departed source's identifiers. Goal: the RFC 8212 fence holds back only the
// stale announcement, never the RFC 4271 Section 9.2 withdrawals of the same
// key. Method: a destination under a limit of 2 holds two identifiers from a
// source that departs; the RIB names one live source's path at a superseded
// epoch. Both identifiers must be withdrawn, and nothing announced.
//
// VALIDATES: queuePathsLimitElection elects without a policy-stale survivor.
// PREVENTS: the per-section fence dropping a key's departure withdrawals.
func TestRFC8212PathsLimitStaleSurvivorKeepsDepartureWithdrawals(t *testing.T) {
	r, destination, _, conn := recoveryLimitedRail(t)
	departed := [][]byte{{0, 0, 0, 0, 24, 192, 0, 2}, {0, 0, 0, 7, 24, 192, 0, 2}}
	var seed [][]byte
	for _, raw := range departed {
		seed = append(seed, ownershipRailBody(family.IPv4Unicast, false, raw, 0))
	}
	failed := recoveryAdvertiseFailed(t, r, destination, seed...)
	setup := conn.written()
	require.Len(t, recoveryWrittenUpdates(t, setup), len(departed), "both paths fit the limit")
	recoveryOwnerStart(t, r, destination)
	stale, _ := newAnnouncePeer(t, "192.0.2.11")
	r.peers[stale.Settings().PeerKey()] = stale
	attachRecoveryOwner(t, r, stale)
	staleEpoch := rfc8212SupersedeEpoch(stale)
	survivor := rfc8212RecoverySurvivor(stale, staleEpoch, []byte{24, 192, 0, 2}, 41)
	owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		outcomes := make([]ribevents.RecoveryOutcome, len(request.Keys))
		for i := range request.Keys {
			outcomes[i] = ribevents.RecoveryOutcome{NLRI: request.Keys[i].NLRI,
				Survivors: []ribevents.RecoveryRoute{survivor}}
		}
		return outcomes, nil
	})
	t.Cleanup(owner.Close)

	require.NoError(t, recoveryDepart(t, r, destination, failed.Settings().Address, "192.0.2.0/24"))

	var got [][]byte
	for _, written := range recoveryWrittenUpdates(t, conn.written()[len(setup):]) {
		require.Empty(t, written.NLRI, "the stale survivor is never announced")
		got = append(got, written.WithdrawnRoutes)
	}
	require.ElementsMatch(t, departed, got, "both departed identifiers are withdrawn")
}

// TestRFC8212RecoveryAdmitHoldsStaleSectionOnly proves the admission fence for
// a policy change that lands after the producer staged a survivor. Goal: the
// stale announcement is never written, the key's withdrawal still is, and
// the key is re-owed rather than left owed with nothing to retry it. Method:
// stage one key with a withdrawal section and a source's announcement
// section, supersede the source's epoch, then admit and settle.
//
// VALIDATES: admit's per-section hold, writes, settle's policy re-owe.
// PREVENTS: a stale section dropping the key's withdrawals, or a key left
// owed with no round to decide it.
func TestRFC8212RecoveryAdmitHoldsStaleSectionOnly(t *testing.T) {
	r, source, destination, _, _ := newSyncOrderRail(t)
	session := destination.currentSession()
	key, record := recoveryOweForTest(t, destination, session, []byte{24, 192, 0, 2})
	epoch := record.epoch
	admission := (&reactorAPIAdapter{r: r}).newRecoveryAdmission(t.Context(), destination, session,
		[]recoveryAdmitKey{{key: key, record: record, epoch: epoch}})
	staleEpoch := rfc8212SupersedeEpoch(source)
	admission.items = []fwdItem{
		{peer: destination, session: session, authority: adjOutRecovery, pathsLimitDemotion: true},
		{peer: destination, session: session, authority: adjOutRecovery, receivedPeer: source,
			receivedGeneration: source.forwardGeneration.Load(), sourcePolicyEpoch: staleEpoch},
	}
	admission.mark(0)

	require.True(t, admission.admit(session), "the admission belongs to this session")
	require.Equal(t, recoveryKeyPolicyStale, admission.keys[0].outcome)
	require.True(t, admission.writes(0), "the key's withdrawal is still written")
	require.False(t, admission.writes(1), "the stale announcement is never written")

	admission.settle()
	require.Same(t, record, recoveryOwedRecord(destination, key), "the key stays owed")
	require.Equal(t, epoch+1, record.epoch, "the key is re-owed for the next round")
}
