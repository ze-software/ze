// Design: docs/architecture/plugin/rib-storage-design.md -- learned-only policy reoffer over scoped origins.
package rib

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A's P elects a scoped local origin; B keeps common learned P. Reoffering
// learned output at A must send only Q, preserving local P and B's output
// without a withdrawal or resend. A genuine local withdrawal then restores P.
// MUTATION: choose a reoffer key's kind from the common winner rather than its scoped election.
func TestRIBOriginPolicyReofferPreservesLocalAndOtherRecipient(t *testing.T) {
	ctx := selectedDispatchContext(t)
	a := netip.MustParseAddr("192.0.2.234")
	b := netip.MustParseAddr("192.0.2.235")
	writes := make(chan rpc.SelectRoutesInput, 16)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		writes <- input
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, d.stop(cleanup))
	})
	p := selectedDispatchRoute(234, 8234)
	q := selectedDispatchRoute(235, 8235)
	require.NoError(t, d.publish(&p))
	require.NoError(t, d.publish(&q))
	require.NoError(t, d.destinationOwnerUp(a, 1234, 2234))
	require.NoError(t, d.destinationOwnerUp(b, 1235, 2235))
	require.NoError(t, d.barrier(ctx))
	initial := make(map[string][]rpc.SelectedRoute)
	for len(writes) != 0 {
		batch := <-writes
		initial[batch.Destination] = append(initial[batch.Destination], batch.Routes...)
	}
	require.Len(t, initial, 2)
	for _, address := range []string{a.String(), b.String()} {
		require.Len(t, initial[address], 2)
		for _, route := range initial[address] {
			require.Equal(t, rpc.SelectedRouteReceived, route.Kind)
		}
		require.ElementsMatch(t, [][]byte{p.keyNLRI, q.keyNLRI}, [][]byte{initial[address][0].KeyNLRI, initial[address][1].KeyNLRI})
	}
	local := originIntakeFixture(a, 1234, 9234, 234)
	accepted, err := d.originIntake(ctx, local)
	require.NoError(t, err)
	require.Equal(t, uint32(1), accepted.Announced)
	announced := selectedDispatchAwait(t, ctx, writes)
	require.Equal(t, a.String(), announced.Destination)
	require.Len(t, announced.Routes, 1)
	require.Equal(t, rpc.SelectedRouteOrigin, announced.Routes[0].Kind)
	require.Equal(t, uint64(9234), announced.Routes[0].OriginReceipt)
	require.NoError(t, d.barrier(ctx))
	require.Empty(t, writes)
	d.mu.Lock()
	beforeLocal := d.destinations[selectedDestinationID{peer: a, session: 2234}].acknowledged[p.key].revision
	beforeOther := d.destinations[selectedDestinationID{peer: b, session: 2235}].acknowledged[p.key].revision
	d.mu.Unlock()

	require.NoError(t, d.reofferLearned(a, 2234))
	require.NoError(t, d.barrier(ctx))
	reoffered := selectedDispatchAwait(t, ctx, writes)
	require.Equal(t, a.String(), reoffered.Destination)
	require.Len(t, reoffered.Routes, 1, "a learned policy change must neither withdraw nor resend the scoped local P")
	require.Equal(t, rpc.SelectedRouteReceived, reoffered.Routes[0].Kind)
	require.Equal(t, q.keyNLRI, reoffered.Routes[0].KeyNLRI)
	require.Empty(t, writes, "the unrelated recipient and local origin receive no second output")
	d.mu.Lock()
	localNow := d.currentScopeLocked(originScope{peer: a, owner: 1234}, p.key).wire
	otherNow := d.currentScopeLocked(originScope{peer: b, owner: 1235}, p.key).wire
	afterLocal := d.destinations[selectedDestinationID{peer: a, session: 2234}].acknowledged[p.key].revision
	afterOther := d.destinations[selectedDestinationID{peer: b, session: 2235}].acknowledged[p.key].revision
	d.mu.Unlock()
	require.Equal(t, rpc.SelectedRouteOrigin, localNow.Kind)
	require.Equal(t, uint64(9234), localNow.OriginReceipt)
	require.Equal(t, beforeLocal, afterLocal)
	require.Equal(t, rpc.SelectedRouteReceived, otherNow.Kind)
	require.Equal(t, p.route.SourceReceipt, otherNow.SourceReceipt)
	require.Equal(t, beforeOther, afterOther)

	local.Operation = rpc.OriginIntakeWithdraw
	_, err = d.originIntake(ctx, local)
	require.NoError(t, err)
	restored := selectedDispatchAwait(t, ctx, writes)
	require.Equal(t, a.String(), restored.Destination)
	require.Len(t, restored.Routes, 1)
	require.Equal(t, rpc.SelectedRouteReceived, restored.Routes[0].Kind)
	require.Equal(t, p.keyNLRI, restored.Routes[0].KeyNLRI)
	require.Equal(t, p.route.SourceReceipt, restored.Routes[0].SourceReceipt)
	require.Empty(t, writes, "B keeps its useful original learned route through A's withdrawal")
}
