package rr

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type validationTestBus struct {
	handler func(any)
}

func (b *validationTestBus) Emit(_, _ string, payload any) (int, error) {
	b.handler(payload)
	return 0, nil
}

func (b *validationTestBus) Subscribe(_, _ string, handler func(any)) func() {
	b.handler = handler
	return func() { b.handler = nil }
}

// A unicast eligibility event must reconcile its ADD-PATH identity. An RPC can
// itself trigger a burst of validation events; the synchronous publisher must
// return so that this RPC and the coalesced recovery replay can finish.
func TestValidationReplaysRetainedUnicastDuringReentrantBurst(t *testing.T) {
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = engineEnd.Close() })
	plugin := sdk.NewWithConn("rr-validation-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = plugin.Close() })
	calls := make(chan replayDispatchCall, 4)
	bus := &validationTestBus{}
	key := ribevents.ValidationRoute{
		Peer: netip.MustParseAddr("192.0.2.1"), Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("203.0.113.0/24"), PathID: 7,
	}
	first := true
	bridge.SetDispatchCommandArgs(func(_ context.Context, command string, args []string, peer string) (*rpc.DispatchCommandOutput, error) {
		if first {
			first = false
			for range 65 {
				if _, err := ribevents.ValidationChange.Emit(bus, []ribevents.ValidationRoute{key}); err != nil {
					return nil, err
				}
			}
		}
		calls <- replayDispatchCall{command: command, args: slices.Clone(args), peer: peer}
		return &rpc.DispatchCommandOutput{Status: statusDone}, nil
	})
	bridge.SetReady()
	rr := &routeReflector{plugin: plugin, peers: map[string]*peerState{
		"192.0.2.1": {Up: true, Families: map[family.Family]bool{family.IPv4Unicast: true}},
		"192.0.2.2": {Up: true, Families: map[family.Family]bool{family.IPv4Unicast: true}},
		"192.0.2.3": {Up: false, Families: map[family.Family]bool{family.IPv4Unicast: true}},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stop := rr.startValidation(ctx, bus)
	defer stop()
	if _, err := ribevents.ValidationChange.Emit(bus, []ribevents.ValidationRoute{key}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case call := <-calls:
			want := []string{"192.0.2.2", "192.0.2.1", "ipv4/unicast", "203.0.113.0/24", "7"}
			if call.command != "request bgp adj-rib-in replay-path" || !slices.Equal(call.args, want) {
				t.Fatalf("wrong retained-path reconciliation: %+v", call)
			}
		case <-ctx.Done():
			t.Fatal("unicast eligibility change never reached the receive store")
		}
	}
}

func TestPeerUpReflectsExistingFlowSpecBeforeEOR(t *testing.T) {
	for _, optionalReplayFails := range []bool{false, true} {
		name := "optional replay succeeds"
		if optionalReplayFails {
			name = "optional replay fails"
		}
		t.Run(name, func(t *testing.T) {
			const source, target = "192.0.2.1", "192.0.2.2"
			fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
			key := ribevents.ValidationRoute{Peer: netip.MustParseAddr(source), Family: fam,
				NLRI: string([]byte{5, 1, 24, 10, 0, 0})}
			attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x80, 14, 11,
				0, 1, 133, 0, 0, 5, 1, 24, 10, 0, 0}
			ribevents.RegisterFlowSpecLookup(
				func(route ribevents.ValidationRoute, id uint64) bool { return route == key && id == 42 },
				func(route ribevents.ValidationRoute) bool { return route == key },
				func(route ribevents.ValidationRoute) (ribevents.FlowSpecPath, bool) {
					return ribevents.FlowSpecPath{Attributes: attrs, MsgID: 42}, route == key
				},
				func() []ribevents.ValidationRoute { return []ribevents.ValidationRoute{key} })
			t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
			bridge := rpc.NewDirectBridge()
			pluginEnd, engineEnd := net.Pipe()
			t.Cleanup(func() { _ = engineEnd.Close() })
			plugin := sdk.NewWithConn("rr-flowspec-replay-test", rpc.NewBridgedConn(pluginEnd, bridge))
			t.Cleanup(func() { _ = plugin.Close() })
			var delivered []string
			done := make(chan struct{}, 2)
			bridge.SetDispatchCommandArgs(func(_ context.Context, command string, _ []string, _ string) (*rpc.DispatchCommandOutput, error) {
				if command != "request bgp adj-rib-in replay" {
					t.Errorf("unexpected replay command %q", command)
				}
				if optionalReplayFails {
					return nil, errors.New("adj-rib-in replay refused")
				}
				return &rpc.DispatchCommandOutput{Status: statusDone, Data: json.RawMessage(`{"last-index":0,"replayed":0}`)}, nil
			})
			bridge.SetRelayStoredRoute(func(_ context.Context, destination string, routes []rpc.StoredRoute) error {
				if destination != target || len(routes) != 1 || routes[0].SourcePeer != source ||
					routes[0].MsgID != 42 || routes[0].NLRIHex != "0501180a0000" || routes[0].Withdraw {
					t.Errorf("wrong pre-existing FlowSpec reflection: %s %+v", destination, routes)
				}
				delivered = append(delivered, "rule")
				return nil
			})
			bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
				var input rpc.UpdateRouteInput
				if err := json.Unmarshal(params, &input); err != nil {
					return nil, err
				}
				if method != rpc.MethodUpdateRoute || input.PeerSelector != target ||
					input.Command != "update text nlri "+fam.String()+" eor" {
					t.Errorf("unexpected route RPC %q: %+v", method, input)
				}
				delivered = append(delivered, "eor")
				return json.RawMessage(`{"result":{"announced":0,"withdrawn":0}}`), nil
			})
			bridge.SetDispatchCommand(func(_ context.Context, command string) (*rpc.DispatchCommandOutput, error) {
				if command != "request peer "+target+" plugin session ready" {
					t.Errorf("unexpected peer action %q", command)
				}
				delivered = append(delivered, "ready")
				done <- struct{}{}
				return &rpc.DispatchCommandOutput{Status: statusDone}, nil
			})
			bridge.SetReady()
			rr := &routeReflector{plugin: plugin, peers: map[string]*peerState{
				source: {Address: source, Up: true},
				target: {Address: target, Families: map[family.Family]bool{fam: true}},
			}}
			rr.handleStructuredState(&rpc.StructuredEvent{PeerAddress: target, State: rpc.SessionStateUp})
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("peer-up reflection never completed")
			}
			if !slices.Equal(delivered, []string{"rule", "eor", "ready"}) {
				t.Fatalf("initial FlowSpec reflection order = %v", delivered)
			}
			rr.mu.Lock()
			rr.peers[target].ReplayGen++
			rr.mu.Unlock()
			rr.replayForPeer(target, 1)
			if !slices.Equal(delivered, []string{"rule", "eor", "ready"}) {
				t.Fatalf("old-session reflection reached reconnected peer: %v", delivered)
			}
		})
	}
}
