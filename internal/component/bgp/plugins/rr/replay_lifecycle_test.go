// Design: docs/guide/route-reflection.md -- joining actual role peer-up replay.
// Related: replay_lifecycle.go -- active lifetimes, not the current peer gate.
package rr

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
	ready func(context.Context) error) (*routeReflector, func(string, uint64, uint64)) {
	t.Helper()
	down, cancel := context.WithCancel(t.Context())
	bridge := rpc.NewDirectBridge()
	consumer, engine := net.Pipe()
	p := sdk.NewWithConn("role-replay-lifetime-question", rpc.NewBridgedConn(consumer, bridge))
	role := &routeReflector{plugin: p, peers: make(map[peerIdentity]*peerState), withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo), downCtx: down, receiveGate: "test-current"}
	var next atomic.Uint64
	bridge.SetDispatchCommandArgs(func(_ context.Context, command string, args []string, _ string) (*rpc.DispatchCommandOutput, error) {
		if command == "test-current" {
			return &rpc.DispatchCommandOutput{Status: statusDone, Data: json.RawMessage(`{"answers":[true]}`)}, nil
		}
		if command != ribevents.ReplayRoutesCommand {
			return nil, fmt.Errorf("unexpected replay command %s", command)
		}
		if args[1] != "0" {
			return &rpc.DispatchCommandOutput{Status: statusDone, Data: json.RawMessage(`{"routes":[],"last-index":0,"more":false,"ingested-msg-id":0}`)}, nil
		}
		message := next.Add(1)
		page := `{"routes":[{"source-peer":"192.0.2.247","source-owner":"171","source-connection-id":"181","source-receipt":"191","msg-id":` + strconv.FormatUint(message, 10) + `,"family":"ipv4/unicast","nlri-framing":"prefix-only","nlri-hex":"18c00002"}],"last-index":` + strconv.FormatUint(message, 10) + `,"more":false,"ingested-msg-id":0}`
		return &rpc.DispatchCommandOutput{Status: statusDone, Data: json.RawMessage(page)}, nil
	})
	bridge.SetRelayStoredRoute(func(ctx context.Context, input rpc.RelayStoredRouteInput) (*rpc.CachedWriteOutput, error) {
		err := relay(ctx, input.Routes)
		return rpc.CompletedCachedWrite(input.DeadlineMS, err), nil
	})
	bridge.SetDispatchRPC(func(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		if result, handled, err := rrFixtureLeaseRPC(method, params); handled {
			return result, err
		}
		var input rpc.DispatchCommandInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, err
		}
		if method != rpc.MethodDispatchCommand {
			return nil, fmt.Errorf("unexpected replay RPC %s", method)
		}
		if ready != nil {
			if err := ready(ctx); err != nil {
				return nil, err
			}
		}
		return json.Marshal(rpc.DispatchCommandOutput{Status: statusDone, DestinationOwner: input.DestinationOwner, DestinationSession: input.DestinationSession, DeadlineMS: input.DeadlineMS})
	})
	bridge.SetReady()
	rrFixtureStart(t, role, rrOriginalFixture(t, "192.0.2.247", 171, 181), 191, 1181, 281)
	t.Cleanup(func() {
		cancel()
		role.replays.stop()
		role.stopDepartures()
		require.NoError(t, p.Close())
		require.NoError(t, engine.Close())
	})
	return role, func(destination string, owner, receipt uint64) {
		role.replays.mu.Lock()
		stopped := role.replays.stopped
		role.replays.mu.Unlock()
		err := role.handleStructuredState(&rpc.StructuredEvent{
			PeerAddress: destination, PeerOwner: owner, ConnectionID: receipt, SessionGeneration: receipt + 1000, DestinationSession: receipt + 100,
			State: rpc.SessionStateUp, InitialReplay: receipt,
		})
		if !stopped {
			require.NoError(t, err)
		}
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
	start("192.0.2.250", 172, 201)
	replayQuestionWait(t, firstEntered, "original stored-page relay")
	start("192.0.2.250", 172, 202)
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
	start("192.0.2.251", 173, 203)
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
	start("192.0.2.252", 174, 204)
	start("192.0.2.253", 175, 205)
	replayQuestionWait(t, entered, "first active producer")
	replayQuestionWait(t, entered, "second active producer")
	stopped := make(chan struct{})
	go func() { role.replays.stop(); close(stopped) }()
	replayQuestionWait(t, stopped, "cancelled replay lifecycle owner")
	replayQuestionWait(t, cancelled, "first actual producer cancellation")
	replayQuestionWait(t, cancelled, "second actual producer cancellation")
	start("192.0.2.254", 176, 206)
	require.Equal(t, uint64(2), writes.Load(), "stopped owner admits no new replay")
	require.Empty(t, role.replays.snapshot(), "completed lifecycles leave no history")
}
