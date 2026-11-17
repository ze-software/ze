// Design: docs/architecture/api/process-protocol.md -- selected transport parity.
package rib

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestSelectedDispatchSDKTransportsPreserveOriginalReceipt(t *testing.T) {
	for _, native := range []bool{true, false} {
		name := "framed-json"
		if native {
			name = "typed-bridge"
		}
		t.Run(name, func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			clientEnd, engineEnd := net.Pipe()
			connection := clientEnd
			observed := make(chan rpc.SelectRoutesInput, 4)
			var handle rpc.SelectRoutesHandler = func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				observed <- input
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			}
			var engine *rpc.MuxConn
			serverDone := make(chan struct{})
			if native {
				bridge := rpc.NewDirectBridge()
				bridge.SetSelectRoutes(handle)
				bridge.SetReady()
				connection = rpc.NewBridgedConn(clientEnd, bridge)
				close(serverDone)
			} else {
				engine = rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
				go func() {
					defer close(serverDone)
					for request := range engine.Requests() {
						var input rpc.SelectRoutesInput
						err := json.Unmarshal(request.Params, &input)
						if request.Method != rpc.MethodSelectRoutes {
							err = errors.New("unexpected selected SDK operation")
						}
						if err != nil {
							if engine.SendError(ctx, request.ID, err.Error()) != nil {
								return
							}
							continue
						}
						output, err := handle(ctx, input)
						if err != nil {
							return
						}
						if engine.SendResult(ctx, request.ID, output) != nil {
							return
						}
					}
				}()
			}
			p := sdk.NewWithConn("bgp-rib", connection)
			d, err := newSelectedDispatch(ctx, p.SelectRoutes, nil, nil)
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, d.stop(ctx))
				require.NoError(t, p.Close())
				if engine != nil {
					require.NoError(t, engine.Close())
				} else {
					require.NoError(t, engineEnd.Close())
				}
				selectedDispatchAwait(t, ctx, serverDone)
			})
			route := selectedDispatchRoute(80, 0x7788)
			route.route.AddPath, route.route.PathID = true, 0
			route.route.NLRI = append([]byte{0, 0, 0, 0}, route.keyNLRI...)
			route.staleLevel = 1
			destination := netip.MustParseAddr("192.0.2.88")
			require.NoError(t, d.publish(&route))
			require.NoError(t, d.destinationUp(destination, 888))
			require.NoError(t, d.barrier(ctx, destination))
			input := selectedDispatchAwait(t, ctx, observed)
			require.Equal(t, uint64(888), input.DestinationSession)
			require.Len(t, input.Routes, 1)
			got := input.Routes[0]
			require.Equal(t, uint64(0x7788), got.SourceReceipt)
			require.True(t, got.SourceAddPath)
			require.True(t, got.Retained)
			require.Equal(t, rpc.SelectedStale, got.StaleLevel)
			require.Equal(t, route.keyNLRI, got.KeyNLRI)
			require.Equal(t, hex.EncodeToString(route.route.NLRI), got.Route.NLRIHex)
			require.Equal(t, uint32(0), got.Route.PathID)
			require.Equal(t, uint64(34), got.Route.MsgID)
			require.Equal(t, uint64(32), got.Route.SourcePolicyEpoch)
			require.Equal(t, rpc.NLRIFramingSourceWire, got.Route.NLRIFraming)
			require.True(t, got.Route.InitialUpdate)
			require.NoError(t, d.remove(route.key))
			require.NoError(t, d.barrier(ctx, destination))
			withdrawal := selectedDispatchAwait(t, ctx, observed)
			require.Equal(t, rpc.SelectedRouteAbsent, withdrawal.Routes[0].Kind)
			require.Nil(t, withdrawal.Routes[0].Route)
			require.Zero(t, withdrawal.Routes[0].SourceReceipt)
		})
	}
}

func TestSelectedDispatchEncodedExchangeSplitsBatch(t *testing.T) {
	attributes := strings.Repeat("a", rpc.MaxMessageSize/3)
	items := make([]selectedDispatchItem, 5)
	for i := range items {
		items[i] = selectedDispatchItem{route: rpc.SelectedRoute{Kind: rpc.SelectedRouteReceived,
			Family: family.IPv4Unicast.String(), KeyNLRI: []byte{24, 198, 18, byte(i)}, Revision: uint64(i + 1),
			Route: &rpc.StoredRoute{Family: family.IPv4Unicast.String(), AttrHex: attributes}}}
	}
	batch, selected, err := selectedDispatchBatch(rpc.SelectRoutesInput{Destination: "192.0.2.89", DestinationSession: 889}, slices.Values(items))
	require.NoError(t, err)
	require.NotEmpty(t, selected)
	require.Less(t, len(selected), len(items))
	size, err := rpc.SelectRoutesExchangeSize(batch)
	require.NoError(t, err)
	require.LessOrEqual(t, size, rpc.MaxMessageSize)
	batch.Routes = append(batch.Routes, items[len(selected)].route)
	size, err = rpc.SelectRoutesExchangeSize(batch)
	require.NoError(t, err)
	require.Greater(t, size, rpc.MaxMessageSize, "the next route really crosses the encoded request/reply budget")
}

func TestSelectedDispatchOversizedRouteDoesNotBlockUsefulKey(t *testing.T) {
	large := selectedDispatchItem{route: rpc.SelectedRoute{Kind: rpc.SelectedRouteReceived,
		Family: family.IPv4Unicast.String(), KeyNLRI: []byte{24, 198, 18, 1}, Revision: 1,
		Route: &rpc.StoredRoute{AttrHex: strings.Repeat("a", rpc.MaxMessageSize)}}}
	useful := selectedDispatchItem{route: rpc.SelectedRoute{Kind: rpc.SelectedRouteAbsent,
		Family: family.IPv4Unicast.String(), KeyNLRI: []byte{24, 198, 18, 2}, Revision: 2}}
	batch, selected, err := selectedDispatchBatch(rpc.SelectRoutesInput{Destination: "192.0.2.89", DestinationSession: 889}, slices.Values([]selectedDispatchItem{large, useful}))
	require.ErrorContains(t, err, "IPC frame limit")
	require.Len(t, selected, 1)
	require.Equal(t, useful.route, batch.Routes[0])
}
