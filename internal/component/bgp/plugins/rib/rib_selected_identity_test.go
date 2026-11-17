// Design: docs/architecture/plugin/rib-storage-design.md -- exact destination Sessions.
package rib

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestSelectedDispatchSameAddressSessionsRetireIndependently(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.94")
	requests := make(chan rpc.SelectRoutesInput, 8)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		requests <- input
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(94, 77)
	require.NoError(t, d.publish(route))
	require.NoError(t, d.destinationUp(peer, 1001))
	require.NoError(t, d.destinationUp(peer, 1002))
	require.NoError(t, d.barrier(ctx))
	first := selectedDispatchAwait(t, ctx, requests)
	second := selectedDispatchAwait(t, ctx, requests)
	require.NotEqual(t, first.DestinationSession, second.DestinationSession)
	require.ElementsMatch(t, []uint64{1001, 1002}, []uint64{first.DestinationSession, second.DestinationSession})
	require.ErrorContains(t, d.destinationDown(ctx, peer, 0), "captured Session")
	d.mu.Lock()
	require.Len(t, d.destinations, 2, "a receipt-free DOWN must never retire either exact Session")
	d.mu.Unlock()
	require.NoError(t, d.destinationDown(ctx, peer, 1001))
	route.route.MessageID++
	require.NoError(t, d.publish(route))
	require.NoError(t, d.barrier(ctx))
	useful := selectedDispatchAwait(t, ctx, requests)
	require.Equal(t, uint64(1002), useful.DestinationSession)
	require.NoError(t, d.destinationDown(ctx, peer, 1001), "late duplicate DOWN cannot retire the other Session")
	query := rpc.CurrentSelectionInput{Destination: peer.String(), DestinationSession: 1002,
		Keys: []rpc.CurrentSelectionKey{{Family: family.IPv4Unicast.String(), KeyNLRI: route.keyNLRI, Revision: useful.Routes[0].Revision}}}
	answer, err := d.currentSelection(ctx, query)
	require.NoError(t, err)
	require.Equal(t, rpc.CurrentSelectionPresent, answer.Results[0].Presence)
	query.DestinationSession = 1001
	_, err = d.currentSelection(ctx, query)
	require.Error(t, err)
}
