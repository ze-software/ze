// Design: docs/architecture/plugin/rib-storage-design.md -- bounded replay source uncertainty.
package rib

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A post-cut replacement really reaches the writer, then its final absence
// settles. Its source still owes the held End without retaining source history.
func TestSelectedRefreshReplacementSourceWaitsForEnd(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.210")
	healthy := netip.MustParseAddr("192.0.2.211")
	firstEntered, firstRelease := make(chan struct{}), make(chan struct{})
	nextEntered, nextRelease := make(chan struct{}), make(chan struct{})
	endEntered := make(chan struct{}, 1)
	var firstOnce, nextOnce sync.Once
	var writes atomic.Int32
	var refuseEnd atomic.Bool
	refuseEnd.Store(true)
	healthyWrites := make(chan rpc.SelectRoutesInput, 2)
	d, err := newSelectedDispatch(ctx, func(work context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		if input.Destination == healthy.String() {
			healthyWrites <- input
			return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
		}
		switch writes.Add(1) {
		case 2:
			close(firstEntered)
			select {
			case <-firstRelease:
			case <-work.Done():
				return nil, work.Err()
			}
		case 3:
			close(nextEntered)
			select {
			case <-nextRelease:
			case <-work.Done():
				return nil, work.Err()
			}
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		firstOnce.Do(func() { close(firstRelease) })
		nextOnce.Do(func() { close(nextRelease) })
		require.NoError(t, d.stop(ctx))
	})
	require.NoError(t, d.setRefreshSender(func(_ context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
		require.Equal(t, uint64(1210), input.PeerOwner)
		require.Equal(t, uint64(210), input.DestinationSession)
		if input.Action == rpc.SelectedRefreshEnd && refuseEnd.Load() {
			select {
			case endEntered <- struct{}{}:
			default:
			}
			return nil, errors.New("replacement replay End is held")
		}
		return selectedRefreshReceipt(input), nil
	}))
	route := selectedDispatchRoute(110, 0x1100)
	replacement := route
	replacement.route.SourceReceipt = 0x1101
	require.NoError(t, d.publish(&route))
	require.NoError(t, d.destinationOwnerUp(peer, 1210, 210))
	require.NoError(t, d.barrier(ctx, peer))
	require.NoError(t, d.requestRefresh(peer, 1210, 210, route.key.family))
	selectedDispatchAwait(t, ctx, firstEntered)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.NoError(t, d.barrierSource(canceled, []uint64{0xdead}), "an untouched source is independent before replacement uncertainty")
	require.NoError(t, d.publish(&replacement))
	firstOnce.Do(func() { close(firstRelease) })
	selectedDispatchAwait(t, ctx, nextEntered)
	require.NoError(t, d.remove(replacement.key))
	nextOnce.Do(func() { close(nextRelease) })
	selectedDispatchAwait(t, ctx, endEntered)
	d.mu.Lock()
	destination := d.destinations[selectedDestinationID{peer: peer, session: 210}]
	current, acknowledged := len(d.current), len(destination.acknowledged)
	pending, inFlight := len(destination.pending), len(destination.inFlight)
	frozenSources := len(destination.refreshActive.sources)
	_, replacementFrozen := destination.refreshActive.sources[replacement.route.SourceReceipt]
	d.mu.Unlock()
	require.Zero(t, current)
	require.Zero(t, acknowledged)
	require.Zero(t, pending)
	require.Zero(t, inFlight)
	require.Equal(t, 1, frozenSources, "replacement history must not grow the frozen source set")
	require.False(t, replacementFrozen)
	require.Equal(t, int32(4), writes.Load(), "initial A, replay A, replacement B and B absence must each settle")
	require.ErrorIs(t, d.barrierSource(canceled, []uint64{replacement.route.SourceReceipt}), context.Canceled)
	require.ErrorIs(t, d.barrierSource(canceled, []uint64{0xdead}), context.Canceled, "uncertain source settlement conservatively joins this destination End")
	// Exercise the production phase wrapper, including a receipt removed by
	// its phase after the wrapper's original affected-source snapshot.
	manager := &RIBManager{selectedDispatch: d}
	manager.sourceLeases = newRIBSourceLeases(manager)
	manager.sourceLeases.leases[replacement.route.SourceReceipt] = &ribSourceLease{peer: peer, peerOwner: 9210}
	input := rpc.SourcePolicyInput{}
	input.Source.PeerOwner = 9210
	complete := manager.settleSourcePolicyPhase(canceled, func(_ netip.Addr, _ rpc.SourcePolicyInput) error {
		manager.sourceLeases.mu.Lock()
		delete(manager.sourceLeases.leases, replacement.route.SourceReceipt)
		manager.sourceLeases.mu.Unlock()
		return nil
	})
	require.ErrorIs(t, complete(peer, input), context.Canceled, "source-policy completion must retain its before-phase affected receipt")
	require.NoError(t, d.destinationUp(healthy, 211))
	unrelated := selectedDispatchRoute(111, 0x1110)
	require.NoError(t, d.publish(&unrelated))
	require.NoError(t, d.barrier(ctx, healthy))
	require.Equal(t, unrelated.keyNLRI, selectedDispatchAwait(t, ctx, healthyWrites).Routes[0].KeyNLRI)
	refuseEnd.Store(false)
	d.mu.Lock()
	d.signalLocked()
	d.mu.Unlock()
	require.NoError(t, d.barrier(ctx, peer))
	require.NoError(t, d.barrierSource(ctx, []uint64{replacement.route.SourceReceipt}))
}

// The actual SDK request preserves unequal owner and Session tokens. A wrong
// nonzero owner cannot replace an already active useful captured refresh.
func TestOrdinaryRefreshUnequalOwnerAndSession(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			peers := [2]netip.Addr{netip.MustParseAddr("192.0.2.212"), netip.MustParseAddr("192.0.2.213")}
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			t.Cleanup(func() { once.Do(func() { close(release) }) })
			var markers atomic.Int32
			var writes atomic.Int32
			send := func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				require.Equal(t, uint64(212), input.DestinationSession)
				writes.Add(1)
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			}
			marker := func(work context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
				require.Equal(t, uint64(1212), input.PeerOwner)
				require.Equal(t, uint64(212), input.DestinationSession)
				markers.Add(1)
				if input.Action == rpc.SelectedRefreshBegin {
					close(entered)
					select {
					case <-release:
					case <-work.Done():
						return nil, work.Err()
					}
				}
				return selectedRefreshReceipt(input), nil
			}
			manager, r := startQuiesceRIB(t, ctx, native, peers, send, marker)
			route := selectedDispatchRoute(112, 0x1120)
			require.NoError(t, r.selectedDispatch.publish(&route))
			selectedRefreshState(t, r, native, peers[0], 1212, 212)
			require.NoError(t, manager.DrainEventDelivery(ctx))
			ordinaryRefresh(t, r, native, peers[0], 1212, 212, true, route.key.family)
			selectedDispatchAwait(t, ctx, entered)
			ordinaryRefresh(t, r, native, peers[0], 9999, 212, true, route.key.family)
			r.selectedDispatch.mu.Lock()
			pending := len(r.selectedDispatch.destinations[selectedDestinationID{peer: peers[0], session: 212}].refreshPending)
			r.selectedDispatch.mu.Unlock()
			require.Zero(t, pending)
			once.Do(func() { close(release) })
			require.NoError(t, manager.DrainEventDelivery(ctx))
			require.Equal(t, int32(2), markers.Load())
			require.Equal(t, int32(2), writes.Load())
		})
	}
}

// Registering a family after UP must not grow that existing Session's queue.
// The real registration is rolled back after the destination worker joins.
func TestSelectedRefreshFamilyRegisteredAfterUPIsRefused(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.214")
	late := family.Family{AFI: family.AFI(65534), SAFI: family.SAFIUnicast}
	_, exists := family.LookupFamily(late.String())
	require.False(t, exists, "the task-owned late family must initially be absent")
	var markers atomic.Int32
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	var added []family.FamilyRegistration
	t.Cleanup(func() { family.UnregisterFamilyBatch(added) })
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	require.NoError(t, d.setRefreshSender(func(_ context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
		markers.Add(1)
		return selectedRefreshReceipt(input), nil
	}))
	require.NoError(t, d.destinationOwnerUp(peer, 1214, 214))
	require.NoError(t, d.barrier(ctx, peer))
	added, err = family.RegisterFamilyBatch([]family.FamilyRegistration{{AFI: late.AFI, SAFI: late.SAFI,
		AFIName: "refresh-late-family", SAFIName: "unicast"}})
	require.NoError(t, err)
	require.Len(t, added, 1)
	require.Error(t, d.requestRefresh(peer, 1214, 214, late))
	d.mu.Lock()
	pending := d.destinations[selectedDestinationID{peer: peer, session: 214}].refreshPending
	d.mu.Unlock()
	require.Nil(t, pending)
	require.NoError(t, d.requestRefresh(peer, 1214, 214, family.IPv4Unicast))
	require.NoError(t, d.barrier(ctx, peer))
	require.Equal(t, int32(2), markers.Load(), "the original admitted family still completes Begin and End")
}
