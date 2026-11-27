// Design: docs/architecture/chaos-web-dashboard.md -- orchestrator type and event processor tests

package orchestrator

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/guard"
	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/chaos/validation"
	"github.com/ze-software/ze/internal/chaos/web"
)

func TestEstablishedStateSetAndSnapshot(t *testing.T) {
	es := newEstablishedState(4)

	snap := es.Snapshot()
	require.Len(t, snap, 4)
	assert.Equal(t, []bool{false, false, false, false}, snap)

	es.Set(0, true)
	es.Set(2, true)

	snap = es.Snapshot()
	assert.Equal(t, []bool{true, false, true, false}, snap)

	snap[0] = false
	assert.Equal(t, []bool{true, false, true, false}, es.Snapshot())
}

func TestEstablishedStateSetFalse(t *testing.T) {
	es := newEstablishedState(3)
	es.Set(1, true)
	assert.Equal(t, []bool{false, true, false}, es.Snapshot())

	es.Set(1, false)
	assert.Equal(t, []bool{false, false, false}, es.Snapshot())
}

func TestEstablishedStateConcurrent(t *testing.T) {
	es := newEstablishedState(10)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for range 100 {
				es.Set(idx, true)
				_ = es.Snapshot()
				es.Set(idx, false)
			}
		}(i)
	}
	wg.Wait()

	snap := es.Snapshot()
	require.Len(t, snap, 10)
	for i, v := range snap {
		assert.False(t, v, "peer %d should be false after concurrent toggle", i)
	}
}

func TestChaosConfigZeroRate(t *testing.T) {
	cfg := ChaosConfig{
		Rate:     0,
		Interval: 10 * time.Second,
		Warmup:   5 * time.Second,
	}
	assert.Equal(t, 0.0, cfg.Rate)
}

func TestOrchestratorEventProcessing(t *testing.T) {
	m := validation.NewModel(3)
	tr := validation.NewTracker(3)
	conv := validation.NewConvergence(3, 5*time.Second)

	ep := &validation.EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	now := time.Now()

	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 2, Time: now})

	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: prefix})

	recvTime := now.Add(50 * time.Millisecond)
	ep.Process(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: recvTime, Prefix: prefix})

	assert.True(t, m.Expected(1).Contains(prefix))
	assert.True(t, m.Expected(2).Contains(prefix))

	assert.True(t, tr.ActualRoutes(1).Contains(prefix))

	stats := conv.Stats()
	assert.Equal(t, 1, stats.Resolved)
	assert.Equal(t, 1, stats.Pending)
}

func TestOrchestratorWithdrawal(t *testing.T) {
	m := validation.NewModel(2)
	tr := validation.NewTracker(2)
	conv := validation.NewConvergence(2, 5*time.Second)

	ep := &validation.EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	now := time.Now()

	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: prefix})
	ep.Process(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: now.Add(10 * time.Millisecond), Prefix: prefix})

	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now.Add(100 * time.Millisecond), Prefix: prefix})
	ep.Process(peer.Event{Type: peer.EventRouteWithdrawn, PeerIndex: 1, Time: now.Add(200 * time.Millisecond), Prefix: prefix})

	assert.False(t, tr.ActualRoutes(1).Contains(prefix))
}

func TestOrchestratorDisconnect(t *testing.T) {
	m := validation.NewModel(3)
	tr := validation.NewTracker(3)
	conv := validation.NewConvergence(3, 5*time.Second)

	ep := &validation.EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	now := time.Now()

	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 2, Time: now})
	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: prefix})

	ep.Process(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: now.Add(time.Second)})

	assert.Equal(t, 0, m.Expected(1).Len())
	assert.Equal(t, 0, m.Expected(2).Len())

	assert.Equal(t, 0, tr.ActualRoutes(0).Len())
}

func TestOrchestratorCounters(t *testing.T) {
	m := validation.NewModel(2)
	tr := validation.NewTracker(2)
	conv := validation.NewConvergence(2, 5*time.Second)

	ep := &validation.EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	now := time.Now()
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})

	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: netip.MustParsePrefix("10.0.0.0/24")})
	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: netip.MustParsePrefix("10.0.1.0/24")})
	ep.Process(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: now, Prefix: netip.MustParsePrefix("10.0.0.0/24")})

	assert.Equal(t, 2, ep.Announced)
	assert.Equal(t, 1, ep.Received)
}

func TestOrchestratorChaosCounters(t *testing.T) {
	m := validation.NewModel(2)
	tr := validation.NewTracker(2)
	conv := validation.NewConvergence(2, 5*time.Second)

	ep := &validation.EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	now := time.Now()

	ep.Process(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 0, Time: now, ChaosAction: "tcp-disconnect"})
	ep.Process(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 1, Time: now, ChaosAction: "partial-withdraw"})
	ep.Process(peer.Event{Type: peer.EventReconnecting, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventWithdrawalSent, PeerIndex: 1, Time: now, Count: 15})
	ep.Process(peer.Event{Type: peer.EventWithdrawalSent, PeerIndex: 0, Time: now, Count: 5})

	assert.Equal(t, 2, ep.ChaosEvents)
	assert.Equal(t, 1, ep.Reconnections)
	assert.Equal(t, 20, ep.Withdrawn, "withdrawn should sum Count fields")

	assert.Equal(t, 0, ep.Announced)
	assert.Equal(t, 0, ep.Received)
}

// VALIDATES: allocatePortRange returns a contiguous range of N free TCP ports
// and checkPortsFree detects a busy port anywhere inside [base, base+count).
// PREVENTS: Multi-peer --port 0 runs or preflight checks missing occupied ports
// past port+0.
func TestAllocatePortRangeAndCheckPortsFree(t *testing.T) {
	ctx := context.Background()
	base, err := allocatePortRange(ctx, "127.0.0.1", 4)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, base, 1024)
	assert.LessOrEqual(t, base+3, 65535)
	require.NoError(t, checkPortsFree("127.0.0.1", base, 4))

	// Occupy the last port in the range and verify checkPortsFree catches it.
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(base+3)))
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	err = checkPortsFree("127.0.0.1", base, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), strconv.Itoa(base+3))
}

// VALIDATES: schedulerStopTime reserves a settle window before bounded deadlines
// while leaving unbounded runs unrestricted.
// PREVENTS: Faults injected in the final seconds of --duration leaving peers mid-backoff
// or withdrawals in flight at observation cutoff.
func TestSchedulerStopTime(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	assert.True(t, schedulerStopTime(time.Time{}, false, start, 5*time.Second).IsZero(),
		"unbounded runs must not stop scheduling")
	assert.True(t, schedulerStopTime(start.Add(4*time.Second), true, start, 5*time.Second).IsZero(),
		"runs shorter than warmup have no active window")

	// 15s run with 5s warmup (10s active) reserves full 4s settle -> stops at +11s.
	stop15 := schedulerStopTime(start.Add(15*time.Second), true, start, 5*time.Second)
	assert.Equal(t, start.Add(11*time.Second), stop15)

	// 9s run with 5s warmup (4s active) caps settle at active/2 (2s) -> stops at +7s.
	stop9 := schedulerStopTime(start.Add(9*time.Second), true, start, 5*time.Second)
	assert.Equal(t, start.Add(7*time.Second), stop9)
}

// TestControlDispatcherSynchronousOutcomes verifies that newControlDispatcher
// and runScheduler answer pause, rate, trigger (with params and guard/state
// checks), and stop synchronously, and reject commands after stop.
//
// VALIDATES: Programmatic MCP control receives real scheduler outcomes rather
// than fire-and-forget enqueue acknowledgments.
// PREVENTS: Triggers silently dropped by guards or unestablished peers, positive
// rate leaving the scheduler paused, or control calls hanging after stop.
func TestControlDispatcherSynchronousOutcomes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	es := newEstablishedState(2)
	es.Set(0, true)
	g := guard.New(2)
	g.OnEstablished(0)

	channels := []chan engine.ChaosAction{
		make(chan engine.ChaosAction, 1),
		make(chan engine.ChaosAction, 1),
	}
	controlCh := make(chan web.ControlCommand, 16)
	schedDone := make(chan struct{})
	var doneOnce sync.Once
	finish := func() { doneOnce.Do(func() { close(schedDone) }) }

	go runScheduler(ctx, ChaosConfig{
		Rate:     0,
		Interval: 50 * time.Millisecond,
		Warmup:   time.Hour,
	}, 42, 2, es, g, channels, controlCh, time.Time{}, true, finish)
	defer func() {
		cancel()
		<-schedDone
	}()

	dispatch := newControlDispatcher(ctx, controlCh, schedDone, nil)

	// Pause and rate commands return synchronously.
	res, err := dispatch(web.ControlCommand{Type: "pause"})
	require.NoError(t, err)
	require.NoError(t, res.Err)
	assert.Equal(t, "pause", res.Action)

	res, err = dispatch(web.ControlCommand{Type: "rate", Rate: 0.4})
	require.NoError(t, err)
	require.NoError(t, res.Err)
	require.NotNil(t, res.Rate)
	assert.Equal(t, 0.4, *res.Rate)

	// Trigger on peer 0 (up) and peer 1 (down): peer 0 dispatches with params, peer 1 blocked.
	res, err = dispatch(web.ControlCommand{
		Type: "trigger",
		Trigger: &web.ManualTrigger{
			ActionType: engine.NameRouteFlap,
			Peers:      []int{0, 1},
			Params:     map[string]string{"count": "3", "cycles": "1"},
		},
	})
	require.NoError(t, err)
	require.NoError(t, res.Err)
	assert.Equal(t, []int{0}, res.Dispatched)
	require.Len(t, res.Blocked, 1)
	assert.Equal(t, 1, res.Blocked[0].Peer)
	assert.Contains(t, res.Blocked[0].Reason, "not established")

	select {
	case got := <-channels[0]:
		assert.Equal(t, engine.ActionRouteFlap, got.Type)
		assert.Equal(t, "3", got.Params["count"])
	case <-time.After(time.Second):
		t.Fatal("expected route-flap action on peer 0 channel")
	}

	// Out-of-range peer returns a clear error.
	res, err = dispatch(web.ControlCommand{
		Type:    "trigger",
		Trigger: &web.ManualTrigger{ActionType: engine.NameTCPDisconnect, Peers: []int{9}},
	})
	require.NoError(t, err)
	require.Error(t, res.Err)
	assert.Contains(t, res.Err.Error(), "out of range")

	// Stop exits the scheduler; a follow-up command fails immediately instead of hanging.
	res, err = dispatch(web.ControlCommand{Type: "stop"})
	require.NoError(t, err)
	require.NoError(t, res.Err)
	<-schedDone

	_, err = dispatch(web.ControlCommand{Type: "pause"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stopped")
}

// TestBuildIfaceFaultParamsAndTriggerDefaults verifies CLI default interface
// fault params propagate to manual triggers and allow per-trigger overrides.
//
// VALIDATES: buildIfaceFaultParams and DispatchManualTrigger parameter merging.
// PREVENTS: Manual triggers dropping CLI --fault-iface/--fault-addr defaults or
// ignoring explicit per-trigger overrides.
func TestBuildIfaceFaultParamsAndTriggerDefaults(t *testing.T) {
	assert.Nil(t, buildIfaceFaultParams("", "", 1, 500*time.Millisecond))

	onlyIface := buildIfaceFaultParams("veth1", "", 2, 100*time.Millisecond)
	require.Contains(t, onlyIface, engine.ActionIfaceLinkFlap)
	assert.NotContains(t, onlyIface, engine.ActionIfaceAddrRemove)
	assert.Equal(t, "veth1", onlyIface[engine.ActionIfaceLinkFlap][engine.ParamIface])
	assert.Equal(t, "2", onlyIface[engine.ActionIfaceLinkFlap][engine.ParamCycles])
	assert.Equal(t, "100ms", onlyIface[engine.ActionIfaceLinkFlap][engine.ParamInterval])

	both := buildIfaceFaultParams("veth1", "10.242.0.2/24", 1, 250*time.Millisecond)
	require.Contains(t, both, engine.ActionIfaceAddrRemove)
	assert.Equal(t, "10.242.0.2/24", both[engine.ActionIfaceAddrRemove][engine.ParamAddr])

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	es := newEstablishedState(1)
	es.Set(0, true)
	g := guard.New(1)
	g.OnEstablished(0)
	channels := []chan engine.ChaosAction{make(chan engine.ChaosAction, 2)}
	controlCh := make(chan web.ControlCommand, 8)
	schedDone := make(chan struct{})
	var doneOnce sync.Once
	finish := func() { doneOnce.Do(func() { close(schedDone) }) }

	go runScheduler(ctx, ChaosConfig{
		Rate:         0,
		Interval:     50 * time.Millisecond,
		Warmup:       time.Hour,
		ActionParams: both,
	}, 42, 1, es, g, channels, controlCh, time.Time{}, true, finish)
	defer func() {
		cancel()
		<-schedDone
	}()

	dispatch := newControlDispatcher(ctx, controlCh, schedDone, nil)

	// Trigger without params inherits CLI defaults.
	res, err := dispatch(web.ControlCommand{
		Type:    "trigger",
		Trigger: &web.ManualTrigger{ActionType: engine.NameIfaceAddrRemove, Peers: []int{0}},
	})
	require.NoError(t, err)
	require.NoError(t, res.Err)
	got := <-channels[0]
	assert.Equal(t, "veth1", got.Params[engine.ParamIface])
	assert.Equal(t, "10.242.0.2/24", got.Params[engine.ParamAddr])
	assert.Equal(t, "250ms", got.Params[engine.ParamInterval])

	// Explicit trigger params override matching defaults while keeping others.
	res, err = dispatch(web.ControlCommand{
		Type: "trigger",
		Trigger: &web.ManualTrigger{
			ActionType: engine.NameIfaceLinkFlap,
			Peers:      []int{0},
			Params:     map[string]string{engine.ParamCycles: "3"},
		},
	})
	require.NoError(t, err)
	require.NoError(t, res.Err)
	got = <-channels[0]
	assert.Equal(t, "veth1", got.Params[engine.ParamIface])
	assert.Equal(t, "3", got.Params[engine.ParamCycles])
	assert.Equal(t, "250ms", got.Params[engine.ParamInterval])
}
