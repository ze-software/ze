// Design: docs/architecture/plugin/rib-storage-design.md -- selected proof boundaries.
package rib

import (
	"context"
	"errors"
	"math"
	"net/netip"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestSelectedDispatchLostAnswerKeepsUncertainSourceBarrier(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.96")
	second, third := make(chan struct{}), make(chan struct{})
	secondRelease, thirdRelease := make(chan struct{}), make(chan struct{})
	var calls atomic.Uint32
	d, err := newSelectedDispatch(ctx, func(ctx context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		switch calls.Add(1) {
		case 1:
			return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
		case 2:
			close(second)
			select {
			case <-secondRelease:
				return nil, errors.New("a partial selected write lost its final answer")
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		case 3:
			close(third)
			select {
			case <-thirdRelease:
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		default:
			return nil, errors.New("unexpected selected request")
		}
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(96, 77)
	require.NoError(t, d.publish(route))
	require.NoError(t, d.destinationUp(peer, 1006))
	require.NoError(t, d.barrier(ctx, peer))
	route.route.SourceReceipt = 88
	require.NoError(t, d.publish(route))
	selectedDispatchAwait(t, ctx, second)
	route.route.SourceReceipt = 99
	require.NoError(t, d.publish(route))
	close(secondRelease)
	selectedDispatchAwait(t, ctx, third)
	bound, cancel := context.WithCancel(ctx)
	cancel()
	err = d.barrierSource(bound, []uint64{88})
	require.Error(t, err, "source 88 may still own output despite the lost answer and later desired source")
	require.Contains(t, err.Error(), peer.String())
	d.mu.Lock()
	require.Len(t, d.destinations[selectedDestinationID{peer: peer, session: 1006}].acknowledged, 1)
	require.True(t, d.destinations[selectedDestinationID{peer: peer, session: 1006}].acknowledged[route.key].uncertain)
	d.mu.Unlock()
	close(thirdRelease)
	require.NoError(t, d.barrier(ctx, peer))
	require.NoError(t, d.barrierSource(ctx, []uint64{88}))
	d.mu.Lock()
	require.False(t, d.destinations[selectedDestinationID{peer: peer, session: 1006}].acknowledged[route.key].uncertain)
	d.mu.Unlock()
}

func TestSelectedDispatchScopeExhaustionRefusesOldAuthority(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.97")
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(97, 77)
	require.NoError(t, d.publish(route))
	require.NoError(t, d.destinationUp(peer, 1007))
	require.NoError(t, d.barrier(ctx, peer))
	d.mu.Lock()
	d.cut = scopedRevision(math.MaxUint64)
	d.mu.Unlock()
	require.ErrorContains(t, d.publish(route), "revision exhausted")
	query := rpc.CurrentSelectionInput{Destination: peer.String(), DestinationSession: 1007,
		Keys: []rpc.CurrentSelectionKey{{Family: family.IPv4Unicast.String(), KeyNLRI: route.keyNLRI, Revision: 1}}}
	_, err = d.currentSelection(ctx, query)
	require.ErrorContains(t, err, "revision exhausted", "a failed new publication must not authorize the old snapshot as current")
	d.mu.Lock()
	require.Equal(t, scopedRevision(math.MaxUint64), d.cut)
	require.Len(t, d.current, 1)
	require.Equal(t, uint64(1), d.current[route.key].wire.Revision)
	d.mu.Unlock()
}
