// Design: docs/architecture/plugin/rib-storage-design.md -- automatic replay ownership.
package adj_rib_in

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/seqmap"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Both automatic producers stand down together, but state and exact-session
// readiness remain owed. With no ordinary owner, both useful replays survive.
func TestOrdinaryReplayOwnershipKeepsReadiness(t *testing.T) {
	for _, delivery := range []string{"structured", "json"} {
		for _, owned := range []bool{false, true} {
			t.Run(delivery+map[bool]string{false: "/fallback", true: "/ordinary"}[owned], func(t *testing.T) {
				const source, target = "192.0.2.41", "192.0.2.42"
				fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
				key := ribevents.ValidationRoute{Peer: netip.MustParseAddr(source), Family: fam, NLRI: string([]byte{5, 1, 24, 10, 0, 0})}
				attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x80, 14, 11, 0, 1, 133, 0, 0, 5, 1, 24, 10, 0, 0}
				ribevents.RegisterFlowSpecLookup(
					func(route ribevents.ValidationRoute, id uint64) bool { return route == key && id == 42 },
					func(route ribevents.ValidationRoute) bool { return route == key },
					func(route ribevents.ValidationRoute) (ribevents.FlowSpecPath, bool) {
						return ribevents.FlowSpecPath{Attributes: attrs, MsgID: 42}, route == key
					}, func() []ribevents.ValidationRoute { return []ribevents.ValidationRoute{key} })
				t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
				bridge := rpc.NewDirectBridge()
				pluginEnd, engineEnd := net.Pipe()
				t.Cleanup(func() { require.NoError(t, engineEnd.Close()) })
				p := sdk.NewWithConn("ordinary-replay-test", rpc.NewBridgedConn(pluginEnd, bridge))
				t.Cleanup(func() { require.NoError(t, p.Close()) })
				var delivered []string
				bridge.SetRelayStoredRoute(func(_ context.Context, destination string, routes []rpc.StoredRoute) error {
					require.Equal(t, target, destination)
					for _, route := range routes {
						require.Equal(t, source, route.SourcePeer)
						require.False(t, route.Withdraw)
						delivered = append(delivered, route.Family)
					}
					return nil
				})
				bridge.SetDispatchCommand(func(_ context.Context, command string) (*rpc.DispatchCommandOutput, error) {
					require.Equal(t, "request peer "+target+" plugin session ready session 471", command)
					delivered = append(delivered, "ready")
					return &rpc.DispatchCommandOutput{Status: statusDone}, nil
				})
				bridge.SetReady()
				r := newTestManager(t)
				r.plugin = p
				routes := seqmap.New[compactRouteKey, *RawRoute]()
				routes.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1,
					&RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "c0000229", NLRIHex: "180a0000"})
				r.ribIn[netip.MustParseAddr(source)] = routes
				if delivery == "structured" {
					r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: target, State: rpc.SessionStateUp,
						InitialReplay: 471, DestinationSession: 471, OrdinaryReplayOwned: owned})
				} else {
					data, err := json.Marshal(&bgp.Event{Type: "state", State: stateUp, InitialReplay: 471,
						DestinationSession: 471, OrdinaryReplayOwned: owned,
						Peer: mustMarshal(t, bgp.PeerInfoJSON{Remote: bgp.PeerRemoteInfo{Address: target, AS: 65001}})})
					require.NoError(t, err)
					event, err := bgp.ParseEvent(data)
					require.NoError(t, err)
					r.handleState(event)
				}
				require.True(t, r.peerUp[netip.MustParseAddr(target)], "stand-down must retain peer bookkeeping")
				want := []string{"ipv4/unicast", fam.String(), "ready"}
				if owned {
					want = []string{"ready"}
				}
				require.Equal(t, want, delivered)
				// An explicit operator replay remains a separate authorized action.
				delivered = nil
				status, _, err := r.replayCommand([]string{target})
				require.NoError(t, err)
				require.Equal(t, statusDone, status)
				require.Equal(t, []string{"ipv4/unicast"}, delivered)
			})
		}
	}
}
