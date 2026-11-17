// Design: docs/architecture/plugin/rib-storage-design.md -- selected refresh settlement.
package rib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ordinaryRefresh enters the real RIB router with already authenticated engine
// facts. The producer-capture tests separately prove where those facts originate.
func ordinaryRefresh(t *testing.T, r *RIBManager, native bool, peer netip.Addr, session uint64, eligible bool, fam family.Family) {
	t.Helper()
	if native {
		message := &bgptypes.RawMessage{RawBytes: []byte{byte(fam.AFI >> 8), byte(fam.AFI), 0, byte(fam.SAFI)}}
		r.dispatchStructured(&rpc.StructuredEvent{EventType: rpc.EventKindRefresh, PeerAddress: peer.String(),
			DestinationSession: session, SelectedDestination: eligible, RawMessage: message})
		return
	}
	payload, err := json.Marshal(Event{Type: "refresh", Peer: json.RawMessage(fmt.Sprintf(`{"remote":{"address":%q}}`, peer.String())),
		DestinationSession: session, SelectedDestination: eligible, AFI: fam.AFI, SAFI: fam.SAFI})
	require.NoError(t, err)
	r.dispatchText(string(payload))
}

// TestOrdinaryRefreshReplaysCurrentSelection checks both public event rails:
// an unchanged current winner is sent again before EoRR, without SENT history.
// This fixture proves SDK receipts and marker ordering, not a BGP socket write.
func TestOrdinaryRefreshReplaysCurrentSelection(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			peers := [2]netip.Addr{netip.MustParseAddr("192.0.2.196"), netip.MustParseAddr("192.0.2.197")}
			written := make(chan rpc.SelectRoutesInput, 8)
			order := make(chan string, 8)
			send := func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				written <- input
				order <- "selected final receipt"
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			}
			manager, r := startQuiesceRIB(t, ctx, native, peers, send)
			route := selectedDispatchRoute(96, 0x9696)
			require.NoError(t, r.selectedDispatch.publish(&route))
			ordinaryDestinationState(t, r, native, peers[0], true, true, 196, rpc.ReasonAttached)
			require.NoError(t, manager.DrainEventDelivery(ctx))
			initial := selectedDispatchAwait(t, ctx, written)
			selectedDispatchAwait(t, ctx, order)
			r.dispatchHook = func(command string) { order <- command }

			ordinaryRefresh(t, r, native, peers[0], 196, true, route.key.family)
			require.NoError(t, manager.DrainEventDelivery(ctx))
			select {
			case refresh := <-written:
				require.Equal(t, initial.DestinationSession, refresh.DestinationSession)
				require.Equal(t, initial.Routes[0].Revision, refresh.Routes[0].Revision)
				require.Equal(t, initial.Routes[0].SourceReceipt, refresh.Routes[0].SourceReceipt)
			default:
				t.Fatal("ordinary refresh replayed SENT history instead of the unchanged current selected route")
			}
			require.Equal(t, "request peer 192.0.2.196 borr ipv4/unicast", selectedDispatchAwait(t, ctx, order))
			require.Equal(t, "selected final receipt", selectedDispatchAwait(t, ctx, order))
			require.Equal(t, "request peer 192.0.2.196 eorr ipv4/unicast", selectedDispatchAwait(t, ctx, order))
		})
	}
}
