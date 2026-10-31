// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on, not the Section 3 verdict itself, which rfc8212_default_reject_test.go and the test/plugin/rfc8212-* carriers prove.
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: source_policy.go -- the phases these tests drive
// Related: forward_gate.go -- the forward work a phase settles
// Related: validation.go -- the revalidation work a phase settles

package rr

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The peers of these tests: the source whose import policy is withdrawn, a
// second source holding one of the same prefixes, and a client.
const (
	spSource = "192.0.2.1"
	spOther  = "192.0.2.2"
	spClient = "192.0.2.3"
)

// spEngine is the engine and receive store the reflector talks to. It
// records every call the reflector makes to it, and while hold is set every
// call reports itself on entered and blocks until hold is closed.
type spEngine struct {
	mu      sync.Mutex
	calls   []string
	hold    chan struct{}
	entered chan string
}

// call records one call and blocks while the engine is held.
func (e *spEngine) call(what string) {
	e.mu.Lock()
	e.calls = append(e.calls, what)
	hold := e.hold
	e.mu.Unlock()
	if hold == nil {
		return
	}
	e.entered <- what
	<-hold
}

// arm makes every later call block until the returned release runs.
func (e *spEngine) arm() (release func()) {
	hold := make(chan struct{})
	e.mu.Lock()
	e.hold = hold
	e.mu.Unlock()
	return sync.OnceFunc(func() {
		e.mu.Lock()
		e.hold = nil
		e.mu.Unlock()
		close(hold)
	})
}

// take returns the calls recorded so far and forgets them.
func (e *spEngine) take() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	calls := e.calls
	e.calls = nil
	return calls
}

// newSourcePolicyReflector returns a reflector wired to an spEngine through
// DirectBridge, with its revalidation worker running and the three peers up
// for IPv4 unicast; the source holds 10.1.0.0/24 and 10.2.0.0/24, the other
// source 10.1.0.0/24, all admitted at epoch 5.
func newSourcePolicyReflector(t *testing.T) (*routeReflector, *spEngine, *validationTestBus) {
	t.Helper()
	engine := &spEngine{entered: make(chan string, 16)}
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = engineEnd.Close() })
	p := sdk.NewWithConn("rr-source-policy-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = p.Close() })
	bridge.SetDispatchCommandArgs(func(_ context.Context, command string, args []string, _ string) (*rpc.DispatchCommandOutput, error) {
		engine.call("dispatch " + command)
		page, err := json.Marshal(ribevents.ReplayPage{Routes: []rpc.StoredRoute{{SourcePeer: spSource,
			Family: "ipv4/unicast", NLRIHex: "180a0100", NLRIFraming: rpc.NLRIFramingPrefixOnly, Withdraw: true}}})
		if err != nil {
			return nil, err
		}
		return &rpc.DispatchCommandOutput{Status: statusDone, Data: page}, nil
	})
	bridge.SetRelayStoredRoute(func(_ context.Context, destination string, _ []rpc.StoredRoute) error {
		engine.call("relay " + destination)
		return nil
	})
	bridge.SetUpdateRouteSel(func(_ context.Context, _ *selector.Selector, command string, _ map[string]any, _ uint64) (uint32, uint32, error) {
		engine.call("update-route-sel " + command)
		return 0, 1, nil
	})
	bridge.SetDispatchRPC(func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
		engine.call("rpc " + method)
		return json.RawMessage(`{"announced":1,"withdrawn":0}`), nil
	})
	bridge.SetReady()
	unicast := func() map[family.Family]bool { return map[family.Family]bool{family.IPv4Unicast: true} }
	rr := &routeReflector{plugin: p,
		peers: map[string]*peerState{
			spSource: {Up: true, Families: unicast()},
			spOther:  {Up: true, Families: unicast()},
			spClient: {Up: true, Families: unicast()},
		},
		withdrawals: map[string]map[string]withdrawalInfo{
			spSource: {
				"ipv4/unicast|10.1.0.0/24": {Family: family.IPv4Unicast, Prefix: "prefix 10.1.0.0/24", SourcePolicyEpoch: 5},
				"ipv4/unicast|10.2.0.0/24": {Family: family.IPv4Unicast, Prefix: "prefix 10.2.0.0/24", SourcePolicyEpoch: 5},
			},
			spOther: {
				"ipv4/unicast|10.1.0.0/24": {Family: family.IPv4Unicast, Prefix: "prefix 10.1.0.0/24", SourcePolicyEpoch: 5},
			},
		}}
	bus := &validationTestBus{}
	ctx, cancel := context.WithCancel(context.Background())
	stop := rr.startValidation(ctx, bus)
	t.Cleanup(func() {
		cancel()
		stop()
	})
	t.Cleanup(rr.stopDepartures)
	return rr, engine, bus
}

// spInput is one lifecycle request of transaction tx for the source,
// captured at epoch 5.
func spInput(tx string, op rpc.SourcePolicyOperation, next uint64) rpc.SourcePolicyInput {
	return rpc.SourcePolicyInput{TransactionID: tx, Operation: op,
		Source: rpc.SourcePolicyIdentity{Peer: spSource, PeerOwner: 1, SessionGeneration: 3, PolicyEpoch: 5,
			ReceiveFrontier: 9, TemplateGeneration: 1},
		NextPolicyEpoch: next, DeadlineMS: time.Now().Add(time.Minute).UnixMilli()}
}

// spPhase runs one lifecycle phase of transaction tx1 for the source.
func spPhase(rr *routeReflector, op rpc.SourcePolicyOperation, next uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return rr.handleSourcePolicy(ctx, spInput("tx1", op, next))
}

// spEpochs renders the source's withdrawal map as key to epoch.
func spEpochs(rr *routeReflector, source string) map[string]uint64 {
	epochs := make(map[string]uint64)
	for key, info := range rr.withdrawals[source] {
		epochs[key] = info.SourcePolicyEpoch
	}
	return epochs
}

func spSameEpochs(t *testing.T, what string, got, want map[string]uint64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
	for key, epoch := range want {
		if have, held := got[key]; !held || have != epoch {
			t.Fatalf("%s: got %v, want %v", what, got, want)
		}
	}
}

// TestRFC8212RouteReflectorWritesNothingDownstream runs a full applied
// transaction and a transaction aborted with a target.
//
// VALIDATES: no phase sends the engine or the receive store anything: no
// relay, no replay-path question, no update-route. bgp-rib withdraws and
// restores the reflected copies, so a write here would duplicate them.
// PREVENTS: the removed relays coming back, which sent duplicate UPDATEs and
// destroyed the released receipt bgp-rib's abort needs at ADD-PATH clients.
func TestRFC8212RouteReflectorWritesNothingDownstream(t *testing.T) {
	rr, engine, _ := newSourcePolicyReflector(t)
	for _, step := range []struct {
		tx   string
		op   rpc.SourcePolicyOperation
		next uint64
	}{
		{"tx1", rpc.SourcePolicyPrepare, 0}, {"tx1", rpc.SourcePolicyApply, 6}, {"tx1", rpc.SourcePolicyFinalize, 0},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := rr.handleSourcePolicy(ctx, spInput(step.tx, step.op, step.next))
		cancel()
		if err != nil {
			t.Fatalf("%s operation %d: %v", step.tx, step.op, err)
		}
	}
	rr, engine2, _ := newSourcePolicyReflector(t)
	for _, step := range []struct {
		op   rpc.SourcePolicyOperation
		next uint64
	}{{rpc.SourcePolicyPrepare, 0}, {rpc.SourcePolicyApply, 6}, {rpc.SourcePolicyAbort, 7}} {
		if err := spPhase(rr, step.op, step.next); err != nil {
			t.Fatalf("operation %d: %v", step.op, err)
		}
	}
	if calls := append(engine.take(), engine2.take()...); len(calls) != 0 {
		t.Fatalf("the source-policy owner wrote downstream: %q", calls)
	}
}

// TestRFC8212RouteReflectorApplyKeepsFinalizeDrops applies a denial of the
// source and then finalizes it.
//
// VALIDATES: apply keeps the source's routes in the withdrawal map at their
// epoch and its pins; a duplicate apply is accepted. Finalize drops the
// source's routes at or below the captured epoch, keeps a route the session
// admitted later and the other source's routes, releases the pins, and an
// abort after it is refused.
// PREVENTS: a denial that loses the routes an abort must restore, and a
// finalize that keeps denied state a later departure would withdraw again.
func TestRFC8212RouteReflectorApplyKeepsFinalizeDrops(t *testing.T) {
	rr, _, _ := newSourcePolicyReflector(t)
	if err := spPhase(rr, rpc.SourcePolicyPrepare, 0); err != nil {
		t.Fatal(err)
	}
	if err := spPhase(rr, rpc.SourcePolicyApply, 6); err != nil {
		t.Fatal(err)
	}
	spSameEpochs(t, "source routes after apply", spEpochs(rr, spSource),
		map[string]uint64{"ipv4/unicast|10.1.0.0/24": 5, "ipv4/unicast|10.2.0.0/24": 5})
	if got := len(rr.sourcePolicy[spSource].pins); got != 2 {
		t.Fatalf("apply kept %d of 2 pins", got)
	}
	if err := spPhase(rr, rpc.SourcePolicyApply, 6); err != nil {
		t.Fatalf("duplicate apply: %v", err)
	}
	// The source's session admits a route after the apply, under its epoch.
	rr.withdrawals[spSource]["ipv4/unicast|10.3.0.0/24"] = withdrawalInfo{Family: family.IPv4Unicast,
		Prefix: "prefix 10.3.0.0/24", SourcePolicyEpoch: 6}
	if err := spPhase(rr, rpc.SourcePolicyFinalize, 0); err != nil {
		t.Fatal(err)
	}
	spSameEpochs(t, "source routes after finalize", spEpochs(rr, spSource),
		map[string]uint64{"ipv4/unicast|10.3.0.0/24": 6})
	spSameEpochs(t, "other source routes after finalize", spEpochs(rr, spOther),
		map[string]uint64{"ipv4/unicast|10.1.0.0/24": 5})
	if rr.sourcePolicy[spSource].pins != nil {
		t.Fatal("finalize kept the pins")
	}
	if err := spPhase(rr, rpc.SourcePolicyAbort, 7); err == nil {
		t.Fatal("an abort after finalize was admitted")
	}
}

// TestRFC8212RouteReflectorAbortTargets aborts a prepared transaction with
// target zero, and an applied one and a prepared one with a target.
//
// VALIDATES: target zero releases the pins and changes nothing else: every
// route keeps its epoch, and a later finalize is refused. A nonzero target
// restamps each pinned route the map still holds unchanged to the target and
// releases the pins; after apply it drops a denied route that changed since
// prepare, and from prepare it keeps that route at its epoch.
// PREVENTS: an abort before apply that restamps, an abort that restores a
// route a later UPDATE replaced, and a rollback that leaves a pinned route
// denied.
func TestRFC8212RouteReflectorAbortTargets(t *testing.T) {
	t.Run("zero before apply", func(t *testing.T) {
		rr, _, _ := newSourcePolicyReflector(t)
		if err := spPhase(rr, rpc.SourcePolicyPrepare, 0); err != nil {
			t.Fatal(err)
		}
		if err := spPhase(rr, rpc.SourcePolicyAbort, 0); err != nil {
			t.Fatal(err)
		}
		spSameEpochs(t, "source routes", spEpochs(rr, spSource),
			map[string]uint64{"ipv4/unicast|10.1.0.0/24": 5, "ipv4/unicast|10.2.0.0/24": 5})
		if rr.sourcePolicy[spSource].pins != nil {
			t.Fatal("abort with target zero kept the pins")
		}
		if err := spPhase(rr, rpc.SourcePolicyFinalize, 0); err == nil {
			t.Fatal("finalize after abort was admitted")
		}
	})
	for _, applied := range []bool{true, false} {
		name := "target after apply"
		if !applied {
			name = "target from prepare"
		}
		t.Run(name, func(t *testing.T) {
			rr, _, _ := newSourcePolicyReflector(t)
			if err := spPhase(rr, rpc.SourcePolicyPrepare, 0); err != nil {
				t.Fatal(err)
			}
			if applied {
				if err := spPhase(rr, rpc.SourcePolicyApply, 6); err != nil {
					t.Fatal(err)
				}
			}
			// A later UPDATE replaced 10.2.0.0/24 under the captured epoch.
			rr.withdrawals[spSource]["ipv4/unicast|10.2.0.0/24"] = withdrawalInfo{Family: family.IPv4Unicast,
				Prefix: "prefix 10.2.0.0/24", Generation: 9, SourcePolicyEpoch: 5}
			if err := spPhase(rr, rpc.SourcePolicyAbort, 7); err != nil {
				t.Fatal(err)
			}
			want := map[string]uint64{"ipv4/unicast|10.1.0.0/24": 7}
			if !applied {
				want["ipv4/unicast|10.2.0.0/24"] = 5
			}
			spSameEpochs(t, "source routes", spEpochs(rr, spSource), want)
			if rr.sourcePolicy[spSource].pins != nil {
				t.Fatal("abort kept the pins")
			}
		})
	}
}

// TestRFC8212RouteReflectorRefusals drives the refusals of prepare.
//
// VALIDATES: prepare refuses a source whose session is down, a token older
// than a reflected route, and a source another open transaction holds, each
// naming the source and the reason. A source holding a FlowSpec route is
// prepared and pinned: bgp-rib covers every family, so no family is refused.
// PREVENTS: pins taken for a source the reflector cannot act for, and a
// valid reload refused for a family the owner never re-decides.
func TestRFC8212RouteReflectorRefusals(t *testing.T) {
	rr, _, _ := newSourcePolicyReflector(t)
	rr.peers[spSource].Up = false
	err := spPhase(rr, rpc.SourcePolicyPrepare, 0)
	if err == nil || !strings.Contains(err.Error(), spSource) || !strings.Contains(err.Error(), "no established session") {
		t.Fatalf("prepare of a down source: %v", err)
	}
	rr, _, _ = newSourcePolicyReflector(t)
	rr.withdrawals[spSource]["ipv4/unicast|10.3.0.0/24"] = withdrawalInfo{Family: family.IPv4Unicast, SourcePolicyEpoch: 6}
	err = spPhase(rr, rpc.SourcePolicyPrepare, 0)
	if err == nil || !strings.Contains(err.Error(), spSource) || !strings.Contains(err.Error(), "capture the source again") {
		t.Fatalf("prepare under a stale token: %v", err)
	}
	if rr.sourcePolicy[spSource] != nil {
		t.Fatal("a refused prepare installed a transaction")
	}
	rr, _, _ = newSourcePolicyReflector(t)
	if err := spPhase(rr, rpc.SourcePolicyPrepare, 0); err != nil {
		t.Fatal(err)
	}
	// A Prepared ledger only pins, so a lost target-zero abort cannot lock the
	// source: the applied ledger is the one that holds it at the same identity.
	if err := spPhase(rr, rpc.SourcePolicyApply, 6); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = rr.handleSourcePolicy(ctx, spInput("tx2", rpc.SourcePolicyPrepare, 0))
	cancel()
	if err == nil || !strings.Contains(err.Error(), spSource) || !strings.Contains(err.Error(), "tx1") {
		t.Fatalf("prepare of a source another applied transaction holds: %v", err)
	}
	rr, _, _ = newSourcePolicyReflector(t)
	flowspec := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	rr.withdrawals[spSource]["ipv4/flowspec|x"] = withdrawalInfo{Family: flowspec, WireForm: true, SourcePolicyEpoch: 5}
	if err := spPhase(rr, rpc.SourcePolicyPrepare, 0); err != nil {
		t.Fatalf("prepare with a FlowSpec route: %v", err)
	}
	if _, pinned := rr.sourcePolicy[spSource].pins["ipv4/flowspec|x"]; !pinned {
		t.Fatal("prepare did not pin the FlowSpec route")
	}
}

// TestRFC8212RouteReflectorPhasesSettleWork holds one kind of the reflector's
// work in the engine and runs prepare, apply or abort beside it.
//
// VALIDATES: each phase answers only after the work is done: a departure
// recovery of the source, an UPDATE being reflected through the event path,
// and a revalidation relay the receive store queued. While the engine holds
// the work the phase has not answered; once released it answers success.
// PREVENTS: a receipt sent while a forward or relay of the source is still on
// its way, which the coordinator's writer join would not see.
func TestRFC8212RouteReflectorPhasesSettleWork(t *testing.T) {
	work := map[string]func(*routeReflector, *validationTestBus) error{
		"departure": func(rr *routeReflector, _ *validationTestBus) error {
			if !rr.enqueueDeparture(netip.MustParseAddr(spSource), map[string]withdrawalInfo{
				"ipv4/unicast|10.9.0.0/24": {Family: family.IPv4Unicast, Prefix: "prefix 10.9.0.0/24", Generation: 2},
			}) {
				return context.Canceled
			}
			return nil
		},
		"forward": func(rr *routeReflector, _ *validationTestBus) error {
			go rr.dispatchText("peer " + spSource + " remote as 65001 received update 100 source-policy-epoch 5 " +
				"origin igp next " + spSource + " nlri ipv4/unicast add 10.4.0.0/24\n")
			return nil
		},
		"validation": func(_ *routeReflector, bus *validationTestBus) error {
			_, err := ribevents.ValidationChange.Emit(bus, []ribevents.ValidationRoute{{
				Peer: netip.MustParseAddr(spSource), Family: family.IPv4Unicast,
				Prefix: netip.MustParsePrefix("10.1.0.0/24")}})
			return err
		},
	}
	phases := []struct {
		name   string
		before []rpc.SourcePolicyOperation
		op     rpc.SourcePolicyOperation
		next   uint64
	}{
		{"prepare", nil, rpc.SourcePolicyPrepare, 0},
		{"apply", []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare}, rpc.SourcePolicyApply, 6},
		{"abort", []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply}, rpc.SourcePolicyAbort, 7},
	}
	for kind, start := range work {
		for _, phase := range phases {
			t.Run(kind+"/"+phase.name, func(t *testing.T) {
				rr, engine, bus := newSourcePolicyReflector(t)
				for _, op := range phase.before {
					next := uint64(0)
					if op == rpc.SourcePolicyApply {
						next = 6
					}
					if err := spPhase(rr, op, next); err != nil {
						t.Fatal(err)
					}
				}
				release := engine.arm()
				t.Cleanup(release)
				if err := start(rr, bus); err != nil {
					t.Fatal(err)
				}
				select {
				case <-engine.entered:
				case <-time.After(5 * time.Second):
					t.Fatal("the work never reached the engine")
				}
				answered := make(chan error, 1)
				go func() { answered <- spPhase(rr, phase.op, phase.next) }()
				select {
				case err := <-answered:
					t.Fatalf("%s answered (%v) while the %s work was still in the engine", phase.name, err, kind)
				case <-time.After(100 * time.Millisecond):
				}
				release()
				select {
				case err := <-answered:
					if err != nil {
						t.Fatalf("%s after the %s work finished: %v", phase.name, kind, err)
					}
				case <-time.After(5 * time.Second):
					t.Fatalf("%s did not answer once the %s work finished", phase.name, kind)
				}
			})
		}
	}
}

// TestRFC8212RouteReflectorSettleRefusesAtDeadline holds a departure of the
// source past a prepare's deadline.
//
// VALIDATES: the prepare fails at its deadline, naming the source and the
// departure, and pins nothing.
// PREVENTS: a phase that hangs past its deadline, or pins over a route set a
// departure is still changing.
func TestRFC8212RouteReflectorSettleRefusesAtDeadline(t *testing.T) {
	rr, engine, _ := newSourcePolicyReflector(t)
	release := engine.arm()
	t.Cleanup(release)
	if !rr.enqueueDeparture(netip.MustParseAddr(spSource), map[string]withdrawalInfo{
		"ipv4/unicast|10.9.0.0/24": {Family: family.IPv4Unicast, Prefix: "prefix 10.9.0.0/24", Generation: 2},
	}) {
		t.Fatal("departure not queued")
	}
	<-engine.entered
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	err := rr.handleSourcePolicy(ctx, spInput("tx1", rpc.SourcePolicyPrepare, 0))
	cancel()
	if err == nil || !strings.Contains(err.Error(), "departure recovery in flight") || !strings.Contains(err.Error(), spSource) {
		t.Fatalf("prepare behind a held departure: %v", err)
	}
	if rr.sourcePolicy[spSource] != nil {
		t.Fatal("prepare pinned while the departure was in the engine")
	}
}
