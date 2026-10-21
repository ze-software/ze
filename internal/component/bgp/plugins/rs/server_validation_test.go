package rs

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync"
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
			// consumer. The post-cut sibling (43) is the live rail's: the
			// source worker relays it once it handles UPDATE 43, after the
			// unicast routes sent before it, so the replay MUST NOT carry it
			// (TestPeerUpReplaysFlowSpecJudgedBeforeTheTargetWasUp).
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
					routes[0].NLRIHex != "0501180a0000" || routes[0].Withdraw {
					t.Errorf("wrong pre-existing FlowSpec replay: %s %+v", destination, routes)
				}
				delivered = append(delivered, "rule-"+strconv.FormatUint(routes[0].MsgID, 10))
				return nil
			})
			bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
				var input rpc.UpdateRouteInput
				if err := json.Unmarshal(params, &input); err != nil {
					return nil, err
				}
				if method != rpc.MethodUpdateRoute || input.PeerSelector != target ||
					input.Command != "update text mp "+fam.String()+" eor" {
					t.Errorf("unexpected route RPC %q: %+v", method, input)
				}
				delivered = append(delivered, "eor")
				return json.RawMessage(`{"result":{"announced":0,"withdrawn":0}}`), nil
			})
			bridge.SetDispatchCommand(func(context.Context, string) (*rpc.DispatchCommandOutput, error) {
				delivered = append(delivered, "ready")
				done <- struct{}{}
				return &rpc.DispatchCommandOutput{Status: statusDone}, nil
			})
			bridge.SetReady()
			rs := &routeServer{plugin: plugin, seenMsgID: 42, peers: map[string]*PeerState{
				source: {Address: source, Up: true},
				target: {Address: target, Families: map[family.Family]bool{fam: true}},
			}}
			rs.handleState(&Event{PeerAddr: target, State: "up", InitialReplay: 17})
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("peer-up replay never completed")
			}
			if !slices.Equal(delivered, []string{"rule-42", "eor", "ready"}) {
				t.Fatalf("initial FlowSpec delivery order = %v; want only the rule at or below the cut", delivered)
			}
			rs.mu.Lock()
			rs.peers[target].ReplayGen++
			rs.mu.Unlock()
			rs.replayForPeer(target, 1, 42, 17, make(chan struct{}))
			if !slices.Equal(delivered, []string{"rule-42", "eor", "ready"}) {
				t.Fatalf("old-session replay reached reconnected peer: %v", delivered)
			}
		})
	}
}

// TestPeerUpReplaysFlowSpecJudgedBeforeTheTargetWasUp proves that a FlowSpec
// rule received after a target's peer-up cut reaches that target exactly once,
// by the live rail and after the replay. The RIB's ValidationChange is not
// ordered against this route server's peer-up event, so it can reach the source
// worker while the target is not yet a forward target and before the worker
// has handled the rule's UPDATE. The worker parks it until that UPDATE, which
// arrives after the cut, so the target is then a target. Replaying the rule as
// well sent it twice, and the replayed copy passed the target's replay fence
// ahead of the live unicast UPDATEs the source sent before the rule (handover
// 03, "FlowSpec ordering race"). Method: process the ValidationChange first (it
// must reach nobody), bring the target up with a cut below the rule's
// MessageID, then let the worker handle UPDATE 43, and require End-of-RIB
// followed by one relay of the rule.
func TestPeerUpReplaysFlowSpecJudgedBeforeTheTargetWasUp(t *testing.T) {
	const source, target = "192.0.2.1", "192.0.2.2"
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	rule := ribevents.ValidationRoute{Peer: netip.MustParseAddr(source), Family: fam,
		NLRI: string([]byte{5, 1, 24, 10, 0, 0})}
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x80, 14, 11,
		0, 1, 133, 0, 0, 5, 1, 24, 10, 0, 0}
	ribevents.RegisterFlowSpecLookup(
		func(route ribevents.ValidationRoute, id uint64) bool { return route == rule && id == 43 },
		func(route ribevents.ValidationRoute) bool { return route == rule },
		func(route ribevents.ValidationRoute) (ribevents.FlowSpecPath, bool) {
			return ribevents.FlowSpecPath{Attributes: attrs, MsgID: 43}, route == rule
		},
		func() []ribevents.ValidationRoute { return []ribevents.ValidationRoute{rule} })
	t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = engineEnd.Close() })
	plugin := sdk.NewWithConn("rs-flowspec-cut-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = plugin.Close() })
	var mu sync.Mutex
	var delivered []string
	record := func(item string) {
		mu.Lock()
		delivered = append(delivered, item)
		mu.Unlock()
	}
	done := make(chan struct{}, 1)
	bridge.SetDispatchCommandArgs(func(context.Context, string, []string, string) (*rpc.DispatchCommandOutput, error) {
		return nil, errors.New("unknown command: request bgp adj-rib-in replay")
	})
	bridge.SetRelayStoredRoute(func(_ context.Context, destination string, routes []rpc.StoredRoute) error {
		for _, route := range routes {
			record(destination + " rule-" + strconv.FormatUint(route.MsgID, 10))
		}
		return nil
	})
	bridge.SetDispatchRPC(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		record(target + " eor")
		return json.RawMessage(`{"result":{"announced":0,"withdrawn":0}}`), nil
	})
	bridge.SetDispatchCommand(func(context.Context, string) (*rpc.DispatchCommandOutput, error) {
		done <- struct{}{}
		return &rpc.DispatchCommandOutput{Status: statusDone}, nil
	})
	bridge.SetReady()
	// seenMsgID 42: this route server has not yet seen the UPDATE (43) the
	// RIB has already judged, so the target's cut lands below the rule.
	rs := &routeServer{plugin: plugin, seenMsgID: 42, peers: map[string]*PeerState{
		source: {Address: source, Up: true, StateSeen: true},
		target: {Address: target, Families: map[family.Family]bool{fam: true}},
	}}
	rs.processValidation(workerKey{sourcePeer: source}, rule)
	mu.Lock()
	early := slices.Clone(delivered)
	mu.Unlock()
	if len(early) != 0 {
		t.Fatalf("validation change reached a peer that was not up: %v", early)
	}
	rs.handleState(&Event{PeerAddr: target, State: "up", InitialReplay: 17})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("peer-up replay never completed")
	}
	rs.markProcessed(workerKey{sourcePeer: source}, 43)
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(delivered, []string{target + " eor", target + " rule-43"}) {
		t.Fatalf("rule judged before peer-up reached the target as %v; want End-of-RIB, then one live relay", delivered)
	}
}

// flowSpecOrderFixture records, in one sequence, what a source worker sends:
// "forward <ids>" for a live batch flush and "relay rule-<id>" for a FlowSpec
// validation relay, so a test can assert their relative order.
type flowSpecOrderFixture struct {
	rs   *routeServer
	key  workerKey
	rule ribevents.ValidationRoute
	mu   sync.Mutex
	seen []string
}

func (f *flowSpecOrderFixture) record(item string) {
	f.mu.Lock()
	f.seen = append(f.seen, item)
	f.mu.Unlock()
}

func (f *flowSpecOrderFixture) sequence() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.seen)
}

// forward hands the worker the live UPDATE msgID from the source, as
// dispatchText would.
func (f *flowSpecOrderFixture) forward(t *testing.T, msgID uint64) {
	t.Helper()
	_, id, peer, payload, err := quickParseTextEvent(buildTestUpdate(f.key.sourcePeer, msgID))
	if err != nil {
		t.Fatalf("test UPDATE %d does not parse: %v", msgID, err)
	}
	f.rs.processForward(f.key, workItem{msgID: id, sourcePeer: peer, textPayload: payload})
}

// newFlowSpecOrderFixture builds a route server whose RIB holds one FlowSpec
// rule from source, received in the UPDATE numbered ruleMsgID, and whose one
// target is up for every family.
func newFlowSpecOrderFixture(t *testing.T, ruleMsgID uint64) *flowSpecOrderFixture {
	t.Helper()
	const source, target = "192.0.2.1", "192.0.2.2"
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	f := &flowSpecOrderFixture{key: workerKey{sourcePeer: source}, rule: ribevents.ValidationRoute{
		Peer: netip.MustParseAddr(source), Family: fam, NLRI: string([]byte{5, 1, 24, 10, 0, 0})}}
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x80, 14, 11, 0, 1, 133, 0, 0, 5, 1, 24, 10, 0, 0}
	ribevents.RegisterFlowSpecLookup(
		func(route ribevents.ValidationRoute, id uint64) bool { return route == f.rule && id == ruleMsgID },
		func(route ribevents.ValidationRoute) bool { return route == f.rule },
		func(route ribevents.ValidationRoute) (ribevents.FlowSpecPath, bool) {
			return ribevents.FlowSpecPath{Attributes: attrs, MsgID: ruleMsgID}, route == f.rule
		},
		func() []ribevents.ValidationRoute { return []ribevents.ValidationRoute{f.rule} })
	t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = engineEnd.Close() })
	plugin := sdk.NewWithConn("rs-flowspec-order-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = plugin.Close() })
	bridge.SetRelayStoredRoute(func(_ context.Context, _ string, routes []rpc.StoredRoute) error {
		for _, route := range routes {
			f.record("relay rule-" + strconv.FormatUint(route.MsgID, 10))
		}
		return nil
	})
	bridge.SetReady()
	f.rs = &routeServer{plugin: plugin, peers: map[string]*PeerState{
		source: {Address: source, Up: true, StateSeen: true},
		target: {Address: target, Up: true, StateSeen: true},
	}, withdrawals: make(map[string]map[withdrawalKey]withdrawalEntry)}
	f.rs.forwardCachedHook = func(ids []uint64, _ []string) {
		item := "forward"
		for _, id := range ids {
			item += " " + strconv.FormatUint(id, 10)
		}
		f.record(item)
	}
	return f
}

// TestFlowSpecValidationWaitsForItsUpdate proves that a FlowSpec rule leaves
// after the UPDATEs its source sent before it. The RIB and the route server
// take the source's UPDATEs on independent deliveries, so the RIB can judge the
// rule (UPDATE 43) and its ValidationChange can reach the source worker before
// that worker has seen the unicast UPDATE 42 sent ahead of the rule. Method:
// process the validation first, then the live UPDATEs 42 and 43, and require
// the live forward of 42 to precede the relay of rule 43.
func TestFlowSpecValidationWaitsForItsUpdate(t *testing.T) {
	f := newFlowSpecOrderFixture(t, 43)
	f.rs.processForward(f.key, workItem{sourcePeer: f.key.sourcePeer, validation: f.rule})
	if early := f.sequence(); len(early) != 0 {
		t.Fatalf("rule 43 left before the worker had seen its UPDATE: %v", early)
	}
	f.forward(t, 42)
	f.forward(t, 43)
	f.rs.flushWorkerBatch(f.key)
	if got, want := f.sequence(), []string{"forward 42 43", "relay rule-43"}; !slices.Equal(got, want) {
		t.Fatalf("worker sent %v; want the live forward of 42 before the relay of rule 43 (%v)", got, want)
	}
}

// startWorkers gives the fixture's route server a worker pool wired as
// production wires it, with the idle and close callbacks recorded in the
// sequence before they run. handled receives once per processed item.
func (f *flowSpecOrderFixture) startWorkers(t *testing.T, idle time.Duration) <-chan struct{} {
	t.Helper()
	handled := make(chan struct{}, 16)
	f.rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		f.rs.processForward(key, item)
		handled <- struct{}{}
	}, poolConfig{
		chanSize: 64, idleTimeout: idle, onDrained: f.rs.flushWorkerBatch,
		onIdle: func(key workerKey) {
			f.record("idle")
			f.rs.releaseParkedValidations(key)
		},
		onClose: func(key workerKey) {
			f.record("close")
			f.rs.dropParkedValidations(key)
		},
	})
	t.Cleanup(f.rs.workers.Stop)
	return handled
}

// TestFlowSpecValidationReleasedWhenItsUpdateNeverArrives proves a parked rule
// is not withheld for ever when its UPDATE never reaches the route server (a
// route server started after the UPDATE, or one the engine did not deliver).
// Method: dispatch the validation and send no UPDATE; the rule must leave once
// the source worker goes idle, and not before.
func TestFlowSpecValidationReleasedWhenItsUpdateNeverArrives(t *testing.T) {
	f := newFlowSpecOrderFixture(t, 43)
	f.startWorkers(t, 50*time.Millisecond)
	f.rs.validationChanged([]ribevents.ValidationRoute{f.rule})
	deadline := time.Now().Add(5 * time.Second)
	for len(f.sequence()) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got, want := f.sequence(), []string{"idle", "relay rule-43"}; !slices.Equal(got[:min(2, len(got))], want) {
		t.Fatalf("worker sent %v; want the parked rule relayed when the idle source worker gave up waiting (%v)", got, want)
	}
}

// TestFlowSpecValidationDroppedWhenSourceGoesDown proves a parked rule whose
// source goes down before its UPDATE arrives is dropped, not relayed, and
// leaves no parked state behind: the source's peer-down withdraws its routes.
// Method: park the rule, take the source down, and close its worker.
func TestFlowSpecValidationDroppedWhenSourceGoesDown(t *testing.T) {
	f := newFlowSpecOrderFixture(t, 43)
	handled := f.startWorkers(t, 5*time.Second)
	f.rs.validationChanged([]ribevents.ValidationRoute{f.rule})
	select {
	case <-handled:
	case <-time.After(5 * time.Second):
		t.Fatal("source worker never processed the validation")
	}
	f.rs.mu.Lock()
	f.rs.peers[f.key.sourcePeer].Up = false
	f.rs.mu.Unlock()
	f.rs.workers.PeerDown(f.key.sourcePeer)
	if got, want := f.sequence(), []string{"close"}; !slices.Equal(got, want) {
		t.Fatalf("worker sent %v; want the parked rule dropped when its source went down (%v)", got, want)
	}
	if parked := f.rs.workerBatch(f.key).parked; len(parked) != 0 {
		t.Fatalf("source down left %d parked validations behind", len(parked))
	}
}
