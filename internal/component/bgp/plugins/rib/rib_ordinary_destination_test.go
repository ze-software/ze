// Design: docs/architecture/plugin/rib-storage-design.md -- ordinary destination admission.
package rib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ordinaryDestinationState enters the actual RIB event router on each rail;
// the fixture's real SDK loop owns every selected write and readiness request.
func ordinaryDestinationState(t *testing.T, r *RIBManager, native bool, peer netip.Addr, up, eligible bool, owner, session uint64, reason string) {
	t.Helper()
	state := rpc.SessionStateDown
	textState := "down"
	if up {
		state, textState = rpc.SessionStateUp, "up"
	}
	initial := uint64(0)
	if up && (eligible || reason != rpc.ReasonAttached) {
		initial = session
	}
	if native {
		r.dispatchStructured(&rpc.StructuredEvent{EventType: rpc.EventKindState, PeerAddress: peer.String(),
			State: state, PeerOwner: owner, SelectedDestination: eligible, DestinationSession: session, InitialReplay: initial, Reason: reason})
		return
	}
	payload, err := json.Marshal(Event{Type: "state", Peer: json.RawMessage(fmt.Sprintf(`{"remote":{"address":%q}}`, peer.String())),
		State: textState, PeerOwner: owner, SelectedDestination: eligible, DestinationSession: session, InitialReplay: initial, Reason: reason})
	require.NoError(t, err)
	r.dispatchText(string(payload))
}

func TestOrdinaryDestinationStateStartsSelectedOutput(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			peers := [2]netip.Addr{netip.MustParseAddr("192.0.2.191"), netip.MustParseAddr("192.0.2.192")}
			written := make(chan rpc.SelectRoutesInput, 8)
			send := func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				written <- input
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			}
			manager, r := startQuiesceRIB(t, ctx, native, peers, send)
			route := selectedDispatchRoute(91, 0x9191)
			require.NoError(t, r.selectedDispatch.publish(&route))

			ordinaryDestinationState(t, r, native, peers[1], true, false, 1192, 192, rpc.ReasonAttached)
			r.selectedDispatch.mu.Lock()
			sourceOnly := len(r.selectedDispatch.destinations)
			r.selectedDispatch.mu.Unlock()
			require.Zero(t, sourceOnly, "a source-only/explicit-role observer must not acquire ordinary output")

			ordinaryDestinationState(t, r, native, peers[0], true, true, 1191, 191, rpc.ReasonAttached)
			r.selectedDispatch.mu.Lock()
			destination := r.selectedDispatch.destinations[selectedDestinationID{peer: peers[0], session: 191}]
			r.selectedDispatch.mu.Unlock()
			require.NotNil(t, destination, "authenticated attached-UP must start its captured ordinary destination")
			require.NoError(t, manager.DrainEventDelivery(ctx))
			initial := selectedDispatchAwait(t, ctx, written)
			require.Equal(t, uint64(191), initial.DestinationSession)
			require.Equal(t, route.keyNLRI, initial.Routes[0].KeyNLRI)
			require.True(t, initial.Routes[0].Route.InitialUpdate)

			route.route.SourceReceipt++
			require.NoError(t, r.selectedDispatch.publish(&route))
			require.NoError(t, manager.DrainEventDelivery(ctx))
			live := selectedDispatchAwait(t, ctx, written)
			require.Equal(t, route.route.SourceReceipt, live.Routes[0].SourceReceipt)
			require.False(t, live.Routes[0].Route.InitialUpdate)

			// A replacement captured connection must survive a delayed DOWN
			// for the predecessor, even though both have the same address.
			ordinaryDestinationState(t, r, native, peers[0], true, true, 1191, 193, rpc.ReasonAttached)
			require.NoError(t, manager.DrainEventDelivery(ctx))
			selectedDispatchAwait(t, ctx, written)
			ordinaryDestinationState(t, r, native, peers[0], false, false, 1191, 191, "")
			r.selectedDispatch.mu.Lock()
			ended := r.selectedDispatch.destinations[selectedDestinationID{peer: peers[0], session: 191}]
			replacement := r.selectedDispatch.destinations[selectedDestinationID{peer: peers[0], session: 193}]
			r.selectedDispatch.mu.Unlock()
			require.Nil(t, ended, "DOWN must join exactly the captured predecessor")
			require.NotNil(t, replacement, "an old DOWN must not cancel its replacement")
			route.route.SourceReceipt++
			require.NoError(t, r.selectedDispatch.publish(&route))
			require.NoError(t, manager.DrainEventDelivery(ctx))
			afterDown := selectedDispatchAwait(t, ctx, written)
			require.Equal(t, uint64(193), afterDown.DestinationSession)
			require.Equal(t, route.route.SourceReceipt, afterDown.Routes[0].SourceReceipt)
		})
	}
}

func TestOrdinaryDestinationEmptyStateReportsReady(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, reason := range []string{"", rpc.ReasonAttached} {
			t.Run(fmt.Sprintf("native=%t/reason=%s", native, reason), func(t *testing.T) {
				ctx := selectedDispatchContext(t)
				peers := [2]netip.Addr{netip.MustParseAddr("192.0.2.194"), netip.MustParseAddr("192.0.2.195")}
				var writes atomic.Int64
				var ready atomic.Int64
				receipts := make(chan rpc.SelectRoutesInput, 2)
				send := func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
					writes.Add(1)
					return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
				}
				manager, r := startQuiesceRIB(t, ctx, native, peers, send)
				r.selectedDispatch.ready = func(ctx context.Context, peer string, session uint64) error {
					select {
					case receipts <- rpc.SelectRoutesInput{Destination: peer, DestinationSession: session}:
					case <-ctx.Done():
						return ctx.Err()
					}
					ready.Add(1)
					return r.signalSelectedReady(ctx, peer, session)
				}
				ordinaryDestinationState(t, r, native, peers[0], true, true, 1194, 194, reason)
				require.NoError(t, manager.DrainEventDelivery(ctx))
				require.Equal(t, int64(1), ready.Load(), "an empty current selection still owes the captured readiness receipt")
				receipt := selectedDispatchAwait(t, ctx, receipts)
				require.Equal(t, peers[0].String(), receipt.Destination)
				require.Equal(t, uint64(194), receipt.DestinationSession)
				require.Zero(t, writes.Load(), "empty selection must not invent a route")
			})
		}
	}
}
