package rs

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

// TestValidationReplayFollowsBufferedForward observes the commands delivered to
// the engine: a cache-change withdrawal must follow a previously buffered live
// UPDATE, even when its received ID falls below a target's peer-up replay cut.
func TestValidationReplayFollowsBufferedForward(t *testing.T) {
	rs := newTestRouteServer(t)
	source := "10.0.0.1"
	target := "10.0.0.2"
	rs.peers[source] = &PeerState{Address: source, Up: true}
	rs.peers[target] = &PeerState{Address: target, Up: true, ForwardFrom: 100}
	key := workerKey{sourcePeer: source}
	rs.batches.Store(key, &forwardBatch{ids: []uint64{90}, targets: []string{target}})
	var delivered []string
	rs.forwardCachedHook = func(_ []uint64, _ []string) {
		delivered = append(delivered, "announce")
	}
	rs.dispatchCommandHook = func(command string, args []string, _ string) (string, json.RawMessage, error) {
		if command != "request bgp adj-rib-in replay-path" || !slices.Equal(args, []string{target, source, "ipv4/unicast", "192.0.2.0/24", "7"}) {
			t.Errorf("replayed the wrong retained path: %s %v", command, args)
		}
		delivered = append(delivered, "withdraw")
		return statusDone, nil, nil
	}
	rs.validationChanged([]ribevents.ValidationRoute{{
		Peer: netip.MustParseAddr(source), Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("192.0.2.0/24"), PathID: 7,
	}})
	rs.workers.Stop()
	if !slices.Equal(delivered, []string{"announce", "withdraw"}) {
		t.Fatalf("engine delivery order = %v; stale announcement must precede reconciliation", delivered)
	}
}

// TestValidationDispatchDuringPeerDown keeps a forward in flight while the
// source shuts down. A concurrent validation callback cannot create a second
// source worker and replay an obsolete path ahead of the departing worker.
func TestValidationDispatchDuringPeerDown(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	pool := newWorkerPool(func(_ workerKey, _ workItem) {
		close(started)
		<-release
	}, poolConfig{chanSize: 1})
	defer pool.Stop()
	key := workerKey{sourcePeer: "192.0.2.1"}
	if !pool.Dispatch(key, workItem{msgID: 1}) {
		t.Fatal("initial forward refused")
	}
	<-started
	pool.mu.Lock()
	closing := pool.workers[key].closeCh
	pool.mu.Unlock()
	down := make(chan struct{})
	go func() {
		pool.PeerDown(key.sourcePeer)
		close(down)
	}()
	<-closing
	accepted := pool.Dispatch(key, workItem{validation: ribevents.ValidationRoute{
		Peer: netip.MustParseAddr(key.sourcePeer), Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("198.51.100.0/24"),
	}})
	close(release)
	<-down
	if accepted {
		t.Fatal("validation replay accepted while source worker was still retiring")
	}
}

func TestPeerUpReplaysExistingFlowSpecBeforeEOR(t *testing.T) {
	for _, optionalStore := range []bool{true, false} {
		name := "without optional store"
		if optionalStore {
			name = "with optional store"
		}
		t.Run(name, func(t *testing.T) {
			const source, target = "192.0.2.1", "192.0.2.2"
			fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
			key := ribevents.ValidationRoute{Peer: netip.MustParseAddr(source), Family: fam,
				NLRI: string([]byte{5, 1, 24, 10, 0, 0})}
			// The selecting RIB can be one event ahead of this forwarding
			// consumer. Its post-cut sibling belongs to the live rail.
			newer := key
			newer.PathID = 7
			attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x80, 14, 11,
				0, 1, 133, 0, 0, 5, 1, 24, 10, 0, 0}
			ribevents.RegisterFlowSpecLookup(
				func(route ribevents.ValidationRoute, id uint64) bool {
					return route == key && id == 42 || route == newer && id == 43
				},
				func(route ribevents.ValidationRoute) bool { return route == key || route == newer },
				func(route ribevents.ValidationRoute) (ribevents.FlowSpecPath, bool) {
					if route == newer {
						return ribevents.FlowSpecPath{Attributes: attrs, MsgID: 43}, true
					}
					return ribevents.FlowSpecPath{Attributes: attrs, MsgID: 42}, route == key
				},
				func() []ribevents.ValidationRoute { return []ribevents.ValidationRoute{key, newer} })
			t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
			bridge := rpc.NewDirectBridge()
			pluginEnd, engineEnd := net.Pipe()
			t.Cleanup(func() { _ = engineEnd.Close() })
			plugin := sdk.NewWithConn("rs-flowspec-replay-test", rpc.NewBridgedConn(pluginEnd, bridge))
			t.Cleanup(func() { _ = plugin.Close() })
			var delivered []string
			done := make(chan struct{}, 2)
			bridge.SetDispatchCommandArgs(func(_ context.Context, command string, _ []string, _ string) (*rpc.DispatchCommandOutput, error) {
				if command != cmdAdjRIBInReplay {
					t.Errorf("unexpected replay command %q", command)
				}
				if !optionalStore {
					return nil, errors.New("unknown command: request bgp adj-rib-in replay")
				}
				return &rpc.DispatchCommandOutput{Status: statusDone, Data: json.RawMessage(`{"last-index":0,"replayed":0,"ingested-msg-id":42}`)}, nil
			})
			bridge.SetRelayStoredRoute(func(_ context.Context, destination string, routes []rpc.StoredRoute) error {
				if destination != target || len(routes) != 1 || routes[0].SourcePeer != source ||
					routes[0].MsgID != 42 || routes[0].NLRIHex != "0501180a0000" || routes[0].Withdraw {
					t.Errorf("wrong pre-existing FlowSpec replay: %s %+v", destination, routes)
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
			rs := &routeServer{plugin: plugin, seenMsgID: 42, peers: map[string]*PeerState{
				source: {Address: source, Up: true},
				target: {Address: target, Families: map[family.Family]bool{fam: true}},
			}}
			rs.handleState(&Event{PeerAddr: target, State: "up"})
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("peer-up replay never completed")
			}
			if !slices.Equal(delivered, []string{"rule", "eor", "ready"}) {
				t.Fatalf("initial FlowSpec delivery order = %v", delivered)
			}
			rs.mu.Lock()
			rs.peers[target].ReplayGen++
			rs.mu.Unlock()
			rs.replayForPeer(target, 1, 42)
			if !slices.Equal(delivered, []string{"rule", "eor", "ready"}) {
				t.Fatalf("old-session replay reached reconnected peer: %v", delivered)
			}
		})
	}
}
