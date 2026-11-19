// Design: docs/architecture/plugin/rib-storage-design.md -- bounded ordinary output.
package rib

import (
	"context"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedDispatchEndedWorkerReleasesAfterCanceledDown holds one admitted
// sender after DOWN cancels it. The failure receipt must leave the live worker
// registered. Final sender completion must release it without a second DOWN.
func TestSelectedDispatchEndedWorkerReleasesAfterCanceledDown(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.105")
	oldID := selectedDestinationID{peer: peer, session: 1006}
	siblingID := selectedDestinationID{peer: peer, session: 1007}
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	blocked := selectedDispatchRoute(105, 77)
	marker := selectedDispatchRoute(106, 78)
	d, err := newSelectedDispatch(ctx, func(work context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		if input.DestinationSession == oldID.session {
			close(entered)
			<-work.Done()
			close(canceled)
			<-release
			return nil, work.Err()
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		require.NoError(t, d.stop(ctx))
	})
	require.NoError(t, d.publish(&blocked))
	require.NoError(t, d.destinationUp(peer, oldID.session))
	selectedDispatchAwait(t, ctx, entered)
	require.NoError(t, d.destinationUp(peer, siblingID.session))
	d.mu.Lock()
	old := d.destinations[oldID]
	sibling := d.destinations[siblingID]
	d.mu.Unlock()
	require.NotNil(t, old)
	require.NotNil(t, sibling)
	require.NoError(t, d.barrierDestinations(ctx, []*selectedDestination{sibling}))

	down, cancelDown := context.WithCancel(ctx)
	defer cancelDown()
	result := make(chan error, 1)
	go func() { result <- d.destinationDown(down, peer, oldID.session) }()
	selectedDispatchAwait(t, ctx, canceled)
	cancelDown()
	refused := selectedDispatchAwait(t, ctx, result)
	require.ErrorIs(t, refused, context.Canceled)
	require.ErrorContains(t, refused, "192.0.2.105 Session 1006 did not retire")
	d.mu.Lock()
	stillRegistered := d.destinations[oldID]
	stillInFlight := len(old.inFlight)
	d.mu.Unlock()
	require.Same(t, old, stillRegistered, "the unjoined sender must keep its retirement obligation")
	require.Equal(t, 1, stillInFlight)

	require.NoError(t, d.publish(&marker))
	require.NoError(t, d.barrierDestinations(ctx, []*selectedDestination{sibling}))
	releaseOnce.Do(func() { close(release) })
	selectedDispatchAwait(t, ctx, old.done)
	d.mu.Lock()
	_, oldRegistered := d.destinations[oldID]
	currentSibling := d.destinations[siblingID]
	originalOutput := sibling.acknowledged[blocked.key]
	markerOutput := sibling.acknowledged[marker.key]
	d.mu.Unlock()
	require.Same(t, sibling, currentSibling)
	require.True(t, originalOutput.possible, "the same-address sibling keeps its original output")
	require.True(t, markerOutput.possible, "the sibling remains useful while the old sender is held")
	require.False(t, oldRegistered, "a finished sender must release its exact Session after a refused join")
	require.NoError(t, d.barrier(ctx), "the ended Session must no longer prevent settlement")
}
