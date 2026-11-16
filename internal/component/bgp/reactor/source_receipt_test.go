// Design: docs/architecture/api/process-protocol.md -- retained selected-source authority.
// Related: source_receipt.go -- immutable session facts and bounded ownership.
package reactor

import (
	"context"
	"math"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// sourceReceiptFixture owns one established peer and its original session facts.
// It opens no transport; each test explicitly advances or retires its source.
func sourceReceiptFixture(t *testing.T) (*Peer, *sourceReceiptFacts) {
	t.Helper()
	address := netip.MustParseAddr("192.0.2.79")
	peer := NewPeer(&PeerSettings{Address: address, LocalAS: 65000, PeerAS: 65001})
	peer.state.Store(int32(PeerStateEstablished))
	peer.advanceForwardGeneration()
	facts, err := newSourceReceiptFacts(relaySource{addr: address, strAdr: "192.0.2.79",
		info: forwardSourceInfo{resolved: true, peer: peer, remoteRouterID: 71, globalLocalAS: 65000,
			filterInfo: filterapi.PeerFilterInfo{Address: address, PeerAS: 65001, LocalAS: 65000,
				Name: "original", GroupName: "sources", LLGRAdvertised: true}},
		ctxID: bgpctx.ContextID(73), srcID: peer.SourceID(), generation: peer.forwardGeneration.Load(), ok: true})
	require.NoError(t, err)
	return peer, facts
}

func sourceReceiptOwner(t *testing.T) (plugin.SelectedOwner, chan struct{}) {
	t.Helper()
	done := make(chan struct{})
	owner, err := plugin.NewSelectedOwner(plugin.ProcessSender("rib"), done, nil)
	require.NoError(t, err)
	return owner, done
}

// TestSourceReceiptFactsExcludeOperationAuthority proves a session snapshot
// cannot retain a particular forward's sender, replay flag or recovery state.
func TestSourceReceiptFactsExcludeOperationAuthority(t *testing.T) {
	_, facts := sourceReceiptFixture(t)
	operation := facts.source
	operation.info.sender = plugin.OperatorSender()
	operation.info.trackDelivery = true
	operation.info.initialUpdate = true
	operation.info.flowSpecRedirectRefused = true
	operation.info.recovery = &recoveryAdmission{}
	captured, err := newSourceReceiptFacts(operation)
	require.NoError(t, err)
	require.False(t, captured.source.info.sender.IsSet())
	require.False(t, captured.source.info.trackDelivery)
	require.False(t, captured.source.info.initialUpdate)
	require.False(t, captured.source.info.flowSpecRedirectRefused)
	require.Nil(t, captured.source.info.recovery)
	require.Equal(t, facts.source.info.filterInfo, captured.source.info.filterInfo)
}

// TestSourceReceiptRetainedFactsSurviveReconnect proves a retained receipt keeps
// its original forwarding facts when the same peer establishes a newer session.
func TestSourceReceiptRetainedFactsSurviveReconnect(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	owner, _ := sourceReceiptOwner(t)
	lease, err := receipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	require.NoError(t, lease.retain())
	peer.state.Store(int32(PeerStateStopped))
	live.close()
	peer.advanceForwardGeneration()
	peer.state.Store(int32(PeerStateEstablished))
	newFacts, err := newSourceReceiptFacts(relaySource{addr: facts.source.addr, strAdr: "192.0.2.79",
		info: forwardSourceInfo{resolved: true, peer: peer, remoteRouterID: 72, globalLocalAS: 65000,
			filterInfo: filterapi.PeerFilterInfo{Address: facts.source.addr, PeerAS: 65002, LocalAS: 65000,
				Name: "replacement", GroupName: "new-sources"}},
		ctxID: bgpctx.ContextID(74), srcID: peer.SourceID(), generation: peer.forwardGeneration.Load(), ok: true})
	require.NoError(t, err)
	replacement, err := receipts.issue(newFacts)
	require.NoError(t, err)
	require.NotEqual(t, live.ID(), replacement.ID())
	write, err := lease.begin(peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	require.NoError(t, write.current())
	original := write.source()
	require.Equal(t, uint32(71), original.info.remoteRouterID)
	require.Equal(t, uint32(65001), original.info.filterInfo.PeerAS)
	require.Equal(t, "original", original.info.filterInfo.Name)
	require.Equal(t, "sources", original.info.filterInfo.GroupName)
	require.True(t, original.info.filterInfo.LLGRAdvertised)
	require.Equal(t, bgpctx.ContextID(73), original.ctxID)
	require.NotEqual(t, peer.forwardGeneration.Load(), original.generation)
	write.complete()
	replacement.close()
	require.NoError(t, lease.revoke(context.Background()))
}

// TestSourceReceiptDownNeedsExistingRetention distinguishes valid plugin-owned
// retention from a receipt that merely belonged to a once-established source.
func TestSourceReceiptDownNeedsExistingRetention(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	owner, _ := sourceReceiptOwner(t)
	lease, err := receipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	peer.state.Store(int32(PeerStateStopped))
	live.close()
	_, err = lease.begin(peer.sourcePolicyEpoch.Load())
	require.Error(t, err, "a non-retained source cannot authorize late output")
	require.NoError(t, lease.retain(), "the already-owned lease can receive the plugin's GR decision")
	write, err := lease.begin(peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	write.complete()
	other, _ := sourceReceiptOwner(t)
	_, err = receipts.acquire(live.ID(), other)
	require.Error(t, err, "a same-named replacement process cannot acquire the old down source")
	require.NoError(t, lease.revoke(context.Background()))
	_, err = receipts.acquire(live.ID(), owner)
	require.Error(t, err, "released down authority cannot be reacquired")
}

// TestSourceReceiptQueuedEventAcquiresBeforeDownApplied proves a physical
// disconnect does not erase an earlier event's right to acquire its receipt.
// The session's delivery reference remains open until the ordered DOWN callback
// has applied; it grants storage ownership, never live forwarding authority.
func TestSourceReceiptQueuedEventAcquiresBeforeDownApplied(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	peer.state.Store(int32(PeerStateStopped))
	peer.advanceForwardGeneration()
	owner, _ := sourceReceiptOwner(t)
	lease, err := receipts.acquire(live.ID(), owner)
	require.NoError(t, err, "the received event was admitted before physical source departure")
	_, err = lease.begin(peer.sourcePolicyEpoch.Load())
	require.Error(t, err, "an event reference is not permission to forward a down source")
	require.NoError(t, lease.retain())
	live.close() // The DOWN callback has now completed.
	write, err := lease.begin(peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	write.complete()
	require.NoError(t, lease.revoke(t.Context()))
	require.Empty(t, receipts.entries)
}

// TestSourceReceiptSourceRemovalRefusesLateAcquire distinguishes permanent
// configured-source removal from the earlier queued physical-DOWN survivor.
func TestSourceReceiptSourceRemovalRefusesLateAcquire(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	owner, _ := sourceReceiptOwner(t)
	peer.stopping.Store(true)
	_, err = receipts.acquire(live.ID(), owner)
	require.Error(t, err, "a removed source cannot create late retained state")
	live.close()
	require.Empty(t, receipts.entries)
}

// TestSourceReceiptStoppedOwnerCannotAcquire proves process liveness remains
// part of lease admission even while the source delivery reference is open.
func TestSourceReceiptStoppedOwnerCannotAcquire(t *testing.T) {
	_, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	owner, stopped := sourceReceiptOwner(t)
	close(stopped)
	_, err = receipts.acquire(live.ID(), owner)
	require.Error(t, err)
	live.close()
	require.Empty(t, receipts.entries)
}

// TestSourceReceiptRevocationJoinsWriter proves revocation refuses new work
// immediately and cannot acknowledge while an admitted final writer still owns it.
func TestSourceReceiptRevocationJoinsWriter(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	owner, _ := sourceReceiptOwner(t)
	lease, err := receipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	write, err := lease.begin(peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	finished := make(chan error, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	go func() { finished <- lease.revoke(ctx) }()
	select {
	case <-write.done():
	case <-ctx.Done():
		t.Fatal("revocation did not publish its cancellation")
	}
	require.Error(t, write.current(), "queued old work is refused before revocation acknowledgment")
	_, err = lease.begin(peer.sourcePolicyEpoch.Load())
	require.Error(t, err)
	select {
	case err := <-finished:
		t.Fatalf("revocation acknowledged before final completion: %v", err)
	default:
	}
	write.complete()
	write.complete() // Writer cleanup MUST be safe across error unwinding.
	select {
	case err := <-finished:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("revocation did not join the completed writer")
	}
	live.close()
	require.Empty(t, receipts.entries)
}

// TestSourceReceiptPolicyAndProcessRevocation proves a retained lease never
// refreshes old policy admission and never inherits a replacement process's life.
func TestSourceReceiptPolicyAndProcessRevocation(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	owner, stopped := sourceReceiptOwner(t)
	lease, err := receipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	require.NoError(t, lease.retain())
	epoch := peer.sourcePolicyEpoch.Load()
	write, err := lease.begin(epoch)
	require.NoError(t, err)
	peer.advanceSourcePolicyEpoch()
	require.Error(t, write.current(), "old admission is never restamped")
	write.complete()
	_, err = lease.begin(epoch)
	require.Error(t, err)
	close(stopped)
	_, err = lease.begin(peer.sourcePolicyEpoch.Load())
	require.Error(t, err)
	require.NoError(t, receipts.revokeOwner(t.Context(), owner))
	live.close()
	require.Empty(t, receipts.entries)
}

// TestSourceReceiptSourceRemovalRefusesRetainedWrite proves the configured
// source's stopping fence also binds valid retained-session authority.
func TestSourceReceiptSourceRemovalRefusesRetainedWrite(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	owner, _ := sourceReceiptOwner(t)
	lease, err := receipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	require.NoError(t, lease.retain())
	peer.stopping.Store(true)
	_, err = lease.begin(peer.sourcePolicyEpoch.Load())
	require.Error(t, err)
	require.NoError(t, lease.revoke(t.Context()))
	live.close()
	require.Empty(t, receipts.entries)
}

// TestSourceReceiptCanceledRevokeStaysRevoked proves a timed-out barrier never
// restores authority, and its last admitted write owns the remaining memory.
func TestSourceReceiptCanceledRevokeStaysRevoked(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	owner, _ := sourceReceiptOwner(t)
	lease, err := receipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	write, err := lease.begin(peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, lease.revoke(ctx), context.Canceled)
	require.Error(t, write.current())
	live.close()
	require.Len(t, receipts.entries, 1, "the unresolved final writer still owns its record")
	write.complete()
	require.Empty(t, receipts.entries)
}

// TestSourceReceiptStorageTracksCurrentOwners churns source incarnations and
// proves completed live/lease/write obligations leave no historical entries.
func TestSourceReceiptStorageTracksCurrentOwners(t *testing.T) {
	_, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	owner, _ := sourceReceiptOwner(t)
	var previous uint64
	for range 256 {
		live, err := receipts.issue(facts)
		require.NoError(t, err)
		require.Greater(t, live.ID(), previous)
		previous = live.ID()
		lease, err := receipts.acquire(live.ID(), owner)
		require.NoError(t, err)
		live.close()
		require.NoError(t, lease.revoke(t.Context()))
		require.Empty(t, receipts.entries)
	}
	receipts.next = math.MaxUint64
	_, err := receipts.issue(facts)
	require.Error(t, err, "receipt identity never wraps to a previously issued value")
	require.Empty(t, receipts.entries)
}
