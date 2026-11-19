// Design: docs/architecture/plugin/rib-storage-design.md -- legacy refresh retains useful SENT output.
// Related: rib_selected_refresh.go -- selected eligibility chooses the replay rail.
package rib

import (
	"context"
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestLegacyRefreshPreservesSentRoutesAndMarkers drives actual SENT ingestion
// and both refresh event routers without ordinary selected eligibility. The
// parsed real route must lie between both markers. Down and selected-owner
// controls prevent unconditional replay or borrowing a missing selected owner.
// This root observes plugin output commands, not a final peer socket write.
func TestLegacyRefreshPreservesSentRoutesAndMarkers(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			r := newTestRIBManager(t)
			t.Cleanup(r.shutdownReceivedSources)
			peer := netip.MustParseAddr("192.0.2.251")
			const owner uint64 = 1251
			r.peerUp[owner] = true
			r.peerMeta[owner] = &peerMetadata{Address: peer, Session: 2251, refreshOwner: owner, refreshSession: 2251}
			body := []byte{0, 0, 0, 14, 0x40, 1, 1, 0, 0x40, 2, 0,
				0x40, 3, 4, 192, 0, 2, 1, 24, 198, 51, 100}
			contextID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			require.NoError(t, err)
			wire := wireu.NewWireUpdate(body, contextID)
			attributes, err := wire.Attrs()
			require.NoError(t, err)
			r.handleSentStructured(&rpc.StructuredEvent{
				PeerAddress: peer.String(), PeerOwner: owner, MessageID: 3251,
				Meta:       map[string]any{"config-static": true},
				RawMessage: &bgptypes.RawMessage{WireUpdate: wire, AttrsWire: attributes},
			})
			require.Len(t, r.collectRibOutRoutes(owner, family.IPv4Unicast), 1,
				"the actual SENT producer must retain the useful original route")

			var sequence []string
			var routes []refreshRouteIdentity
			r.dispatchHook = func(command string) { sequence = append(sequence, command) }
			r.updateHook = func(command string, meta map[string]any) {
				sequence = append(sequence, "route")
				require.Equal(t, true, meta["replay"])
				groups := consumeRefreshCommand(t, command)
				routes = append(routes, consumedRefreshRoutes(t, groups)...)
				for _, group := range groups {
					require.Equal(t, attributes.Packed(), group.Wire.Packed())
				}
			}
			refresh := func(selected bool) {
				if native {
					r.handleRefreshStructured(&rpc.StructuredEvent{
						PeerAddress: peer.String(), PeerOwner: owner,
						DestinationSession: 2251, SelectedDestination: selected,
						RawMessage: &bgptypes.RawMessage{RawBytes: []byte{0, 1, 0, 1}},
					})
					return
				}
				r.dispatchText(string(mustMarshal(t, Event{
					Type: "refresh", PeerOwner: owner,
					Peer:               mustMarshal(t, PeerInfoJSON{Remote: PeerRemoteInfo{Address: peer.String()}}),
					DestinationSession: 2251, SelectedDestination: selected,
					AFI: family.AFIIPv4, SAFI: family.SAFIUnicast,
				})))
			}
			refresh(false)
			require.Equal(t, []string{
				"request peer 192.0.2.251 borr ipv4/unicast", "route",
				"request peer 192.0.2.251 eorr ipv4/unicast",
			}, sequence)
			require.Equal(t, []refreshRouteIdentity{{family: family.IPv4Unicast,
				prefix: netip.MustParsePrefix("198.51.100.0/24")}}, routes)

			sequence, routes = nil, nil
			r.peerUp[owner] = false
			refresh(false)
			require.Empty(t, sequence, "a down owner starts neither routes nor markers")
			require.Empty(t, routes)

			r.peerUp[owner] = true
			refresh(true)
			require.Empty(t, sequence, "a missing ordinary dispatcher cannot fall back to SENT history")
			require.Empty(t, routes)
			require.Len(t, r.collectRibOutRoutes(owner, family.IPv4Unicast), 1,
				"the controls must preserve the useful original SENT inventory")
		})
	}
}

// A real receiving Session's old refresh is projected ineligible after its
// replacement. Both event routers must refuse it before any legacy command.
// The replacement's current selected refresh remains useful.
// MUTATION: send all ineligible refresh observations to replayLegacyRefresh.
func TestStaleRefreshCannotReplaySentToOrdinaryReplacement(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			r := newTestRIBManager(t)
			t.Cleanup(r.shutdownReceivedSources)
			peer := netip.MustParseAddr("192.0.2.252")
			const owner uint64 = 1252
			const oldSession uint64 = 2251
			const currentSession uint64 = 2252
			selectedRefreshState(t, r, native, peer, owner, oldSession)
			body := []byte{0, 0, 0, 14, 0x40, 1, 1, 0, 0x40, 2, 0,
				0x40, 3, 4, 192, 0, 2, 1, 24, 198, 51, 100}
			contextID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			require.NoError(t, err)
			wire := wireu.NewWireUpdate(body, contextID)
			attributes, err := wire.Attrs()
			require.NoError(t, err)
			r.handleSentStructured(&rpc.StructuredEvent{PeerAddress: peer.String(), PeerOwner: owner, MessageID: 3252,
				Meta: map[string]any{"config-static": true}, RawMessage: &bgptypes.RawMessage{WireUpdate: wire, AttrsWire: attributes}})
			require.Len(t, r.collectRibOutRoutes(owner, family.IPv4Unicast), 1)
			var legacy []string
			r.dispatchHook = func(command string) { legacy = append(legacy, command) }
			r.updateHook = func(command string, _ map[string]any) { legacy = append(legacy, command) }
			written := make(chan rpc.SelectRoutesInput, 4)
			markers := make(chan rpc.SelectedRefreshAction, 4)
			d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				written <- input
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			}, nil, nil)
			require.NoError(t, err)
			r.selectedDispatch = d
			t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
			require.NoError(t, d.setRefreshSender(func(_ context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
				require.Equal(t, currentSession, input.DestinationSession)
				markers <- input.Action
				return selectedRefreshReceipt(input), nil
			}))
			route := selectedDispatchRoute(252, 8252)
			require.NoError(t, d.publish(&route))
			selectedRefreshState(t, r, native, peer, owner, currentSession)
			require.NoError(t, d.barrier(ctx, peer))
			selectedDispatchAwait(t, ctx, written)
			ordinaryRefresh(t, r, native, peer, owner, oldSession, false, family.IPv4Unicast)
			require.NoError(t, d.barrier(ctx, peer))
			require.Empty(t, legacy, "stale ineligible capture must issue no bare-address SENT replay or markers")
			require.Empty(t, markers)
			require.Empty(t, written)
			ordinaryRefresh(t, r, native, peer, owner, currentSession, true, family.IPv4Unicast)
			require.NoError(t, d.barrier(ctx, peer))
			require.Equal(t, rpc.SelectedRefreshBegin, selectedDispatchAwait(t, ctx, markers))
			require.Equal(t, rpc.SelectedRefreshEnd, selectedDispatchAwait(t, ctx, markers))
			require.Equal(t, route.keyNLRI, selectedDispatchAwait(t, ctx, written).Routes[0].KeyNLRI)
			require.Empty(t, legacy)
			require.Len(t, r.collectRibOutRoutes(owner, family.IPv4Unicast), 1)
		})
	}
}
