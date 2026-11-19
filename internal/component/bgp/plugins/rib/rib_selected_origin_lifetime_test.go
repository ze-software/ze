// Design: docs/architecture/plugin/rib-storage-design.md -- bounded scoped origin admission.
package rib

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedDispatchWithdrawnOriginsLeaveNoDisconnectedScopeRows churns
// origins in a scope with no Session while another owner of the same address
// receives the shared learned winners. The late recipient must get those
// winners without needing a historical scope row for any withdrawn origin.
func TestSelectedDispatchWithdrawnOriginsLeaveNoDisconnectedScopeRows(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.106")
	const disconnectedOwner = 410
	const liveOwner = 411
	liveID := selectedDestinationID{peer: peer, session: 1008}
	lateID := selectedDestinationID{peer: peer, session: 1009}
	disconnected := originScope{peer: peer, owner: disconnectedOwner}
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })

	const keyCount = 8
	for i := range keyCount {
		learned := selectedDispatchRoute(byte(110+i), uint64(2000+i))
		require.NoError(t, d.publish(&learned))
	}
	require.NoError(t, d.destinationOwnerUp(peer, liveOwner, liveID.session))
	require.NoError(t, d.barrier(ctx))
	for i := range keyCount {
		input := originIntakeFixture(peer, disconnectedOwner, uint64(3000+i), byte(110+i))
		announced, err := d.originIntake(ctx, input)
		require.NoError(t, err)
		require.Zero(t, announced.Announced)
		require.Len(t, announced.Admissions, 1)
		require.Equal(t, rpc.RouteAdmissionSelected, announced.Admissions[0].Disposition)
		input.Operation = rpc.OriginIntakeWithdraw
		withdrawn, err := d.originIntake(ctx, input)
		require.NoError(t, err)
		require.Equal(t, uint32(1), withdrawn.Withdrawn)
	}

	// A real source replacement should keep the same shared key useful.
	replacement := selectedDispatchRoute(110, 4000)
	require.NoError(t, d.publish(&replacement))
	require.NoError(t, d.barrier(ctx))
	d.mu.Lock()
	historicalRows := len(d.scoped[disconnected])
	remainingOrigins := len(d.origins)
	sharedLearned := len(d.current)
	live := d.destinations[liveID]
	liveOutput := live.acknowledged[replacement.key]
	d.mu.Unlock()
	require.Zero(t, remainingOrigins)
	require.Equal(t, keyCount, sharedLearned)
	require.True(t, liveOutput.possible)
	require.Equal(t, replacement.route.SourceReceipt, liveOutput.source)

	require.NoError(t, d.destinationOwnerUp(peer, disconnectedOwner, lateID.session))
	require.NoError(t, d.barrier(ctx))
	d.mu.Lock()
	late := d.destinations[lateID]
	lateLearned := len(late.acknowledged)
	lateOutput := late.acknowledged[replacement.key]
	d.mu.Unlock()
	require.Equal(t, keyCount, lateLearned)
	require.True(t, lateOutput.possible)
	require.Equal(t, replacement.route.SourceReceipt, lateOutput.source)
	require.Zero(t, historicalRows, "a disconnected scope has no admitted origin or output that can retain each learned key")

	input := originIntakeFixture(peer, disconnectedOwner, 5000, 110)
	announced, err := d.originIntake(ctx, input)
	require.NoError(t, err)
	require.Equal(t, uint32(1), announced.Announced)
	input.Operation = rpc.OriginIntakeWithdraw
	withdrawn, err := d.originIntake(ctx, input)
	require.NoError(t, err)
	require.Equal(t, uint32(1), withdrawn.Withdrawn)
	d.mu.Lock()
	fallback := d.scoped[disconnected][replacement.key]
	activeOutput := late.acknowledged[replacement.key]
	d.mu.Unlock()
	require.NotNil(t, fallback.current)
	require.Greater(t, activeOutput.revision, lateOutput.revision, "the live recipient must retain the newer fallback revision")
	require.Equal(t, replacement.route.SourceReceipt, activeOutput.source)

	require.NoError(t, d.destinationDown(ctx, peer, lateID.session))
	d.mu.Lock()
	rowsAfterDown := len(d.scoped[disconnected])
	otherOutput := d.destinations[liveID].acknowledged[replacement.key]
	d.mu.Unlock()
	require.Zero(t, rowsAfterDown, "the joined last recipient releases its obsolete scoped fallback")
	require.Equal(t, liveOutput, otherOutput, "the same-address independent owner keeps its output")
}
