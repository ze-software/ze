// Design: docs/architecture/plugin/rib-storage-design.md -- learned policy reoffer.
package rib

import (
	"context"
	"net/netip"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	bgpformat "github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// MUTATION: queue only pending keys without remembering a newer reoffer across
// an in-flight request; its old suppressed answer consumes the request forever.
func TestSelectedPolicyReofferSurvivesHeldSuppression(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.181")
	entered, release := make(chan struct{}), make(chan struct{})
	requests := make(chan rpc.SelectRoutesInput, 4)
	var calls atomic.Int32
	d, err := newSelectedDispatch(ctx, func(ctx context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		requests <- input
		if calls.Add(1) == 1 {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return selectedDispatchAnswer(input, rpc.SelectedSuppressed), nil
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(81, 77)
	require.NoError(t, d.publish(&route))
	require.NoError(t, d.destinationUp(peer, 881))
	selectedDispatchAwait(t, ctx, entered)
	require.NoError(t, d.reofferLearned(peer, 881))
	d.mu.Lock()
	require.False(t, d.settledLocked(d.destinations[selectedDestinationID{peer: peer, session: 881}]))
	d.mu.Unlock()
	close(release)
	require.NoError(t, d.barrier(ctx, peer))
	first := selectedDispatchAwait(t, ctx, requests)
	second := selectedDispatchAwait(t, ctx, requests)
	require.Equal(t, first.Routes[0].Revision, second.Routes[0].Revision, "the same current selection is re-evaluated, not replaced")
	require.Equal(t, int32(2), calls.Load())
	require.Empty(t, requests)
}

// A mixed current selected map contains received and local origins. Policy
// reoffer MUST visit only received routes, despite a useful acknowledged local.
func TestSelectedPolicyReofferPreservesLocalAndExactSession(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.182")
	requests := make(chan rpc.SelectRoutesInput, 4)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		requests <- input
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	learned, local := selectedDispatchRoute(82, 77), selectedDispatchRoute(83, 78)
	require.NoError(t, d.publish(&learned))
	localSelection, err := newSelectedScopedRoute(rpc.SelectedRoute{Kind: rpc.SelectedRouteOrigin,
		Family: local.key.family.String(), KeyNLRI: local.keyNLRI, OriginReceipt: 83,
		StaleLevel: rpc.SelectedFresh, Route: &rpc.StoredRoute{Family: local.key.family.String()}})
	require.NoError(t, err)
	d.mu.Lock()
	err = d.advanceLocked()
	localSelection.wire.Revision = uint64(d.cut)
	d.current[local.key] = localSelection
	d.mu.Unlock()
	require.NoError(t, err)
	require.NoError(t, d.destinationUp(peer, 882))
	require.NoError(t, d.barrier(ctx, peer))
	initial := selectedDispatchAwait(t, ctx, requests)
	require.Len(t, initial.Routes, 2)
	require.NoError(t, d.reofferLearned(peer, 881))
	require.NoError(t, d.barrier(ctx, peer))
	require.Empty(t, requests, "a stale Session cannot reoffer to its successor")
	require.NoError(t, d.reofferLearned(peer, 882))
	require.NoError(t, d.barrier(ctx, peer))
	replayed := selectedDispatchAwait(t, ctx, requests)
	require.Len(t, replayed.Routes, 1)
	require.Equal(t, rpc.SelectedRouteReceived, replayed.Routes[0].Kind)
	require.Equal(t, learned.keyNLRI, replayed.Routes[0].KeyNLRI)
	d.mu.Lock()
	ack := d.destinations[selectedDestinationID{peer: peer, session: 882}].acknowledged[local.key]
	require.Equal(t, scopedRevision(localSelection.wire.Revision), ack.revision)
	require.True(t, ack.possible, "local output retains its acknowledged wire ownership")
	d.mu.Unlock()
}

// The policy reason is not another UP: both event rails retain source/peer
// bookkeeping, enqueue only an eligible captured owner, and arm no ready fence.
func TestSelectedPolicyStateRailsSkipConnectionBookkeeping(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			peer := netip.MustParseAddr("192.0.2.184")
			requests := make(chan rpc.SelectRoutesInput, 4)
			d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				requests <- input
				return selectedDispatchAnswer(input, rpc.SelectedSuppressed), nil
			}, nil, nil)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
			route := selectedDispatchRoute(84, 77)
			require.NoError(t, d.publish(&route))
			require.NoError(t, d.destinationUp(peer, 884))
			require.NoError(t, d.barrier(ctx, peer))
			selectedDispatchAwait(t, ctx, requests)
			const owner uint64 = 184
			r := &RIBManager{selectedDispatch: d, peerUp: map[uint64]bool{owner: false}}
			deliver := func(eligible bool) {
				if typed {
					r.handleStructuredState(&rpc.StructuredEvent{PeerOwner: owner, PeerAddress: peer.String(), State: rpc.SessionStateUp,
						Reason: rpc.ReasonExportPolicy, DestinationSession: 884, SelectedDestination: eligible})
					return
				}
				info := &plugin.PeerInfo{PeerOwner: owner, Address: peer, PeerAS: rpc.RemoteASOf(65001), DestinationSession: 884, SelectedDestination: eligible}
				event := mustParseEvent(t, string(bgpformat.AppendStateChange(nil, info, rpc.SessionStateUp, rpc.ReasonExportPolicy, nil, plugin.EncodingJSON)))
				require.Equal(t, eligible, event.SelectedDestination, "the real wire envelope must preserve projected eligibility")
				r.handleState(event)
			}
			deliver(false)
			require.NoError(t, d.barrier(ctx, peer))
			require.Empty(t, requests, "state observers gain no replay authority")
			deliver(true)
			require.NoError(t, r.settleEvents(ctx))
			replayed := selectedDispatchAwait(t, ctx, requests)
			require.Len(t, replayed.Routes, 1)
			require.False(t, r.peerUp[owner], "policy reconciliation must not report a connection transition")
		})
	}
}
