// Design: docs/architecture/plugin/rib-storage-design.md -- scoped policy reoffer.
// Related: rib_selected_policy_test.go -- learned-only reoffer keeps local output.
package rib

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgpformat "github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedPolicyAllStateRailsReofferScopedOnlyOrigin gives A a local key
// absent from the common learned map, while A and B share a useful learned key.
// Both state rails must reoffer A's two elected keys at their exact revisions,
// preserve its admitted local receipt, and leave B's useful output untouched.
// MUTATION: omit the scoped-key union from full policy reoffer; A's local key
// then never reaches the selected writer for revalidation.
func TestSelectedPolicyAllStateRailsReofferScopedOnlyOrigin(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
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
			learned := selectedDispatchRoute(235, 8235)
			require.NoError(t, d.publish(&learned))
			require.NoError(t, d.destinationOwnerUp(a, 1234, 2234))
			require.NoError(t, d.destinationOwnerUp(b, 1235, 2235))
			require.NoError(t, d.barrier(ctx))
			initial := make(map[string]rpc.SelectedRoute)
			for len(writes) != 0 {
				batch := <-writes
				require.Len(t, batch.Routes, 1)
				require.Equal(t, rpc.SelectedRouteReceived, batch.Routes[0].Kind)
				require.Equal(t, learned.keyNLRI, batch.Routes[0].KeyNLRI)
				require.Equal(t, learned.route.SourceReceipt, batch.Routes[0].SourceReceipt)
				initial[batch.Destination] = batch.Routes[0]
			}
			require.Len(t, initial, 2)
			require.Contains(t, initial, a.String())
			require.Contains(t, initial, b.String())

			local := originIntakeFixture(a, 1234, 9234, 234)
			accepted, err := d.originIntake(ctx, local)
			require.NoError(t, err)
			require.Equal(t, uint32(1), accepted.Announced)
			announced := selectedDispatchAwait(t, ctx, writes)
			require.Equal(t, a.String(), announced.Destination)
			require.Len(t, announced.Routes, 1)
			require.Equal(t, rpc.SelectedRouteOrigin, announced.Routes[0].Kind)
			require.Equal(t, local.Route.KeyNLRI, announced.Routes[0].KeyNLRI)
			require.Equal(t, local.OriginReceipt, announced.Routes[0].OriginReceipt)
			require.NoError(t, d.barrier(ctx))
			require.Empty(t, writes)
			localKey, err := selectedDispatchKey(local.Route.Family, local.Route.KeyNLRI)
			require.NoError(t, err)
			d.mu.Lock()
			_, common := d.current[localKey]
			localBefore := d.currentScopeLocked(originScope{peer: a, owner: 1234}, localKey).wire
			learnedBefore := d.currentForLocked(d.destinations[selectedDestinationID{peer: a, session: 2234}], learned.key).wire
			otherBefore := d.destinations[selectedDestinationID{peer: b, session: 2235}].acknowledged[learned.key]
			d.mu.Unlock()
			require.False(t, common, "the local-only key must exercise the scoped half of the reoffer union")
			require.Equal(t, initial[a.String()].Revision, learnedBefore.Revision)
			require.Equal(t, announced.Routes[0].Revision, localBefore.Revision)

			r := &RIBManager{selectedDispatch: d, peerUp: map[uint64]bool{1234: false}}
			if typed {
				r.handleStructuredState(&rpc.StructuredEvent{PeerOwner: 1234, PeerAddress: a.String(), State: rpc.SessionStateUp,
					Reason: rpc.ReasonExportPolicyAll, DestinationSession: 2234, SelectedDestination: true})
			}
			if !typed {
				info := &plugin.PeerInfo{PeerOwner: 1234, Address: a, PeerAS: rpc.RemoteASOf(65001),
					DestinationSession: 2234, SelectedDestination: true}
				event := mustParseEvent(t, string(bgpformat.AppendStateChange(nil, info, rpc.SessionStateUp,
					rpc.ReasonExportPolicyAll, nil, plugin.EncodingJSON)))
				require.Equal(t, rpc.ReasonExportPolicyAll, event.Reason)
				require.Equal(t, uint64(2234), event.DestinationSession)
				require.True(t, event.SelectedDestination)
				r.handleState(event)
			}
			require.NoError(t, r.settleEvents(ctx))
			reoffered := selectedDispatchAwait(t, ctx, writes)
			require.Equal(t, a.String(), reoffered.Destination)
			require.Equal(t, uint64(2234), reoffered.DestinationSession)
			require.Len(t, reoffered.Routes, 2)
			require.ElementsMatch(t, []rpc.SelectedRoute{learnedBefore, localBefore}, reoffered.Routes,
				"both useful elected keys must retain their kinds, exact revisions and original receipts")
			for _, route := range reoffered.Routes {
				require.NotNil(t, route.Route)
				require.False(t, route.Route.InitialUpdate, "policy reoffer must not arm an initial replay")
			}
			require.Empty(t, writes, "B must receive neither a withdrawal nor another announcement")
			require.False(t, r.peerUp[1234], "policy reconciliation must not become a connection transition")
			d.mu.Lock()
			localAfter := d.currentScopeLocked(originScope{peer: a, owner: 1234}, localKey).wire
			otherAfter := d.destinations[selectedDestinationID{peer: b, session: 2235}].acknowledged[learned.key]
			d.mu.Unlock()
			require.Equal(t, localBefore, localAfter, "revalidation must preserve the admitted scoped winner")
			require.Equal(t, otherBefore, otherAfter, "B must retain its useful acknowledged learned route")
			require.True(t, otherAfter.possible)
		})
	}
}
