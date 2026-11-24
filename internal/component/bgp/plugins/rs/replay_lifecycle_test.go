// Design: docs/architecture/plugin/rib-storage-design.md -- joining actual role peer-up replay.
// Related: replay_lifecycle.go -- active lifetimes, not the current peer gate.
package rs

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// replayJoinContext observes entry to the real blocking select, so the question
// does not treat an unscheduled goroutine or a short sleep as a withheld ACK.
type replayJoinContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (ctx *replayJoinContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.entered) })
	return ctx.Context.Done()
}

// replayQuestionWait names an actual lifecycle milestone with a bounded failure
// wait. The bound is a test failure deadline, never a success condition.
func replayQuestionWait(t *testing.T, milestone <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-milestone:
	case <-time.After(5 * time.Second):
		t.Fatal(name + " did not finish")
	}
}

// newReplayQuestion drives the real role state handler, paginated replay
// producer and SDK stored-route/final-readiness calls through DirectBridge.
// Cleanup cancels and joins role-owned work before closing the connection.
func newReplayQuestion(t *testing.T, relay func(context.Context, []rpc.StoredRoute) error,
	ready func(context.Context) error) (*routeServer, func(string, uint64, uint64, uint64, uint64, uint64)) {
	t.Helper()
	down, cancel := context.WithCancel(t.Context())
	bridge := rpc.NewDirectBridge()
	consumer, engine := net.Pipe()
	p := sdk.NewWithConn("role-replay-lifetime-question", rpc.NewBridgedConn(consumer, bridge))
	source := rsFixtureKey("192.0.2.247", 171, 181)
	role := &routeServer{plugin: p, peers: map[workerKey]*PeerState{source: {Address: "192.0.2.247", Up: true, RSClient: true, StateSeen: true, DestinationSession: 381, SessionGeneration: 7}}, withdrawals: make(map[string]map[withdrawalKey]withdrawalEntry), downCtx: down, clk: clock.RealClock{}}
	var targetsMu sync.Mutex
	targets := make(map[rpc.ForwardTarget]uint64)
	var leased bool
	bridge.SetDispatchNative(func(ctx context.Context, method string, value any) (any, bool, error) {
		if method == rpc.MethodSourceLease {
			input, ok := value.(rpc.SourceLeaseInput)
			if !ok {
				return nil, true, fmt.Errorf("original source lease shape %T", value)
			}
			if err := input.Validate(time.Now()); err != nil {
				return nil, true, err
			}
			if input.PeerOwner != 171 || input.SourceReceipt != 191 {
				return nil, true, fmt.Errorf("replay changed original source lease: %+v", input)
			}
			targetsMu.Lock()
			defer targetsMu.Unlock()
			switch input.Action {
			case rpc.SourceLeaseAcquire:
				leased = true
			case rpc.SourceLeaseRetain:
				if !leased {
					return nil, true, fmt.Errorf("original retention preceded acquisition")
				}
			case rpc.SourceLeaseRelease:
				leased = false
			default:
				return nil, true, fmt.Errorf("unknown source action")
			}
			return &rpc.SourceLeaseOutput{Action: input.Action, PeerOwner: input.PeerOwner, SourceReceipt: input.SourceReceipt}, true, nil
		}
		if method != rpc.MethodDispatchCommand {
			return nil, false, nil
		}
		input, err := rsFixtureCommandInput(value)
		if err != nil {
			return nil, true, err
		}
		targetsMu.Lock()
		var receipt uint64
		var originalTarget rpc.ForwardTarget
		for target, token := range targets {
			if target.DestinationOwner == input.DestinationOwner && target.DestinationSession == input.DestinationSession {
				receipt = token
				originalTarget = target
				break
			}
		}
		targetsMu.Unlock()
		if receipt == 0 || input.Command != "request peer "+originalTarget.Destination+" plugin session ready session "+strconv.FormatUint(receipt, 10) {
			return nil, true, fmt.Errorf("readiness lost its captured original UP: %+v", input)
		}
		if ready != nil {
			if err := ready(ctx); err != nil {
				return nil, true, err
			}
		}
		return rsFixtureBoundCommand(input), true, nil
	})
	var next atomic.Uint64
	bridge.SetDispatchCommandArgs(func(_ context.Context, command string, args []string, _ string) (*rpc.DispatchCommandOutput, error) {
		if command != ribevents.ReplayRoutesCommand {
			return nil, fmt.Errorf("unexpected replay command %q", command)
		}
		if args[1] != "0" {
			return &rpc.DispatchCommandOutput{Status: "done", Data: json.RawMessage(`{"routes":[],"last-index":0,"more":false,"ingested-msg-id":0}`)}, nil
		}
		message := next.Add(1)
		// Both admitted source generations must remain current together: two
		// simultaneous destination replays may capture either page first.
		// Distinct original native rows preserve message1/2 and the SAME source
		// receipt without replacing the other held producer's membership.
		var nlri string
		switch message {
		case 1:
			nlri = "18c00002"
		case 2:
			nlri = "18c00003"
		default:
			return nil, fmt.Errorf("unexpected original replay page %d", message)
		}
		row := rpc.StoredRoute{SourcePeer: "192.0.2.247", SourceOwner: 171, SourceConnectionID: 181, SourceReceipt: 191, SourcePolicyEpoch: 5, MsgID: message, Family: "ipv4/unicast", NLRIFraming: rpc.NLRIFramingPrefixOnly, NLRIHex: nlri}
		// The page carries an actual current original receive row before replay
		// admission; the source lease was acquired through this same SDK process.
		rsFixtureCurrentFlowSpec(t, role, source, 191, 5, message, family.IPv4Unicast, row.NLRIHex)
		ingested := uint64(0)
		page, err := json.Marshal(ribevents.ReplayPage{Routes: []rpc.StoredRoute{row}, LastIndex: message, IngestedMsgID: &ingested})
		return &rpc.DispatchCommandOutput{Status: "done", Data: page}, err
	})
	bridge.SetRelayStoredRoute(func(ctx context.Context, input rpc.RelayStoredRouteInput) (*rpc.CachedWriteOutput, error) {
		target := rpc.ForwardTarget{Destination: input.Destination, DestinationOwner: input.DestinationOwner, DestinationSession: input.DestinationSession}
		targetsMu.Lock()
		_, current := targets[target]
		sourceLive := leased
		targetsMu.Unlock()
		if !current || !sourceLive {
			return nil, fmt.Errorf("relay lost actual original source/target: %+v", input)
		}
		if _, bounded := ctx.Deadline(); !bounded {
			return nil, fmt.Errorf("actual relay has no finite deadline")
		}
		if err := relay(ctx, input.Routes); err != nil {
			return nil, err
		}
		return &rpc.CachedWriteOutput{Completed: true, DeadlineMS: input.DeadlineMS}, nil
	})
	bridge.SetReady()
	t.Cleanup(func() {
		cancel()
		role.replays.stop()
		require.NoError(t, p.Close())
		require.NoError(t, engine.Close())
	})
	if err := role.acquireSource(source, 191); err != nil {
		t.Fatal(err)
	}
	return role, func(destination string, owner, connection, session, generation, receipt uint64) {
		target := rsFixtureTarget(destination, owner, session)
		targetsMu.Lock()
		targets[target] = receipt
		targetsMu.Unlock()
		role.handleStateUp(rsFixtureKey(destination, owner, connection), receipt, session, generation)
	}
}

// TestRoleReplayJoinIncludesSupersededStoredPage holds the first real stored
// page inside its SDK relay, reconnects that destination and lets the successor
// replay use the same source receipt. The completed successor MUST NOT remove
// the original escaped page from settlement. Only that page's actual return
// permits the join. This proves lifetime joining, not per-route GR admission.
// MUTATION: omit replay admission before launching the peer-up lifecycle.
func TestRoleReplayJoinIncludesSupersededStoredPage(t *testing.T) {
	firstEntered := make(chan struct{})
	firstRelease := make(chan struct{})
	secondWritten := make(chan struct{})
	firstWritten := make(chan struct{})
	var release sync.Once
	role, start := newReplayQuestion(t, func(ctx context.Context, routes []rpc.StoredRoute) error {
		if len(routes) != 1 {
			return fmt.Errorf("replay page has %d routes; want one", len(routes))
		}
		if routes[0].SourceReceipt != 191 {
			return fmt.Errorf("replay lost the original source receipt")
		}
		switch routes[0].MsgID {
		case 1:
			close(firstEntered)
			select {
			case <-firstRelease:
				close(firstWritten)
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		case 2:
			close(secondWritten)
			return nil
		default:
			t.Errorf("unexpected replay message %d", routes[0].MsgID)
			return nil
		}
	}, nil)
	t.Cleanup(func() { release.Do(func() { close(firstRelease) }) })
	start("192.0.2.250", 250, 350, 450, 7, 201)
	replayQuestionWait(t, firstEntered, "original stored-page relay")
	start("192.0.2.250", 250, 351, 451, 8, 202)
	replayQuestionWait(t, secondWritten, "useful replacement replay")
	joined := make(chan error, 1)
	ctx := &replayJoinContext{Context: t.Context(), entered: make(chan struct{})}
	go func() { joined <- role.settleReplays(ctx) }()
	select {
	case err := <-joined:
		t.Fatalf("join acknowledged an escaped original page: %v", err)
	case <-ctx.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("replay join did not begin")
	}
	release.Do(func() { close(firstRelease) })
	replayQuestionWait(t, firstWritten, "original relay return")
	select {
	case err := <-joined:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("replay join did not observe original completion")
	}
	require.NoError(t, role.settleReplays(t.Context()))
}

// TestRoleReplayJoinWaitsForReadinessAndRefusesCancellation holds the actual
// last readiness RPC after the stored page has been relayed. Settlement MUST
// refuse cancellation rather than reporting that queued work was applied;
// after the last RPC returns, a healthy join succeeds and leaves no history.
// MUTATION: close the lifecycle completion before the readiness call returns.
func TestRoleReplayJoinWaitsForReadinessAndRefusesCancellation(t *testing.T) {
	readyEntered := make(chan struct{})
	readyRelease := make(chan struct{})
	var release sync.Once
	role, start := newReplayQuestion(t, func(context.Context, []rpc.StoredRoute) error { return nil },
		func(ctx context.Context) error {
			close(readyEntered)
			select {
			case <-readyRelease:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	t.Cleanup(func() { release.Do(func() { close(readyRelease) }) })
	start("192.0.2.251", 251, 351, 451, 7, 203)
	replayQuestionWait(t, readyEntered, "final readiness RPC")
	parent, cancel := context.WithCancel(t.Context())
	defer cancel()
	ctx := &replayJoinContext{Context: parent, entered: make(chan struct{})}
	joined := make(chan error, 1)
	go func() { joined <- role.settleReplays(ctx) }()
	select {
	case err := <-joined:
		t.Fatalf("join passed the held readiness RPC: %v", err)
	case <-ctx.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("replay join did not begin")
	}
	cancel()
	select {
	case err := <-joined:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled replay join did not refuse")
	}
	release.Do(func() { close(readyRelease) })
	require.NoError(t, role.settleReplays(t.Context()))
}

// TestRoleReplayStopCancelsCapturedLifetimes drives two real peer-up producers
// into held stored-page RPCs, then stops their lifecycle owner. Both calls MUST
// observe cancellation before stop returns. A later peer-UP MUST start no work.
// The hold also watches test cancellation so a broken stop cannot leak workers.
// MUTATION: omit cancellation of the captured active replay contexts in stop.
func TestRoleReplayStopCancelsCapturedLifetimes(t *testing.T) {
	entered := make(chan struct{}, 2)
	cancelled := make(chan struct{}, 2)
	var writes atomic.Uint64
	role, start := newReplayQuestion(t, func(ctx context.Context, _ []rpc.StoredRoute) error {
		writes.Add(1)
		entered <- struct{}{}
		select {
		case <-ctx.Done():
			cancelled <- struct{}{}
			return ctx.Err()
		case <-t.Context().Done():
			return t.Context().Err()
		}
	}, nil)
	start("192.0.2.252", 252, 352, 452, 7, 204)
	start("192.0.2.253", 253, 353, 453, 7, 205)
	replayQuestionWait(t, entered, "first active producer")
	replayQuestionWait(t, entered, "second active producer")
	stopped := make(chan struct{})
	go func() { role.replays.stop(); close(stopped) }()
	replayQuestionWait(t, stopped, "cancelled replay lifecycle owner")
	replayQuestionWait(t, cancelled, "first actual producer cancellation")
	replayQuestionWait(t, cancelled, "second actual producer cancellation")
	start("192.0.2.254", 254, 354, 454, 7, 206)
	require.Equal(t, uint64(2), writes.Load(), "stopped owner admits no new replay")
	require.Empty(t, role.replays.snapshot(), "completed lifecycles leave no history")
}
