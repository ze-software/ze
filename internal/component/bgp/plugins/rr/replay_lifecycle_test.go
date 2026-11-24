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

// replayQuestionReportMargin is the time kept before the test binary's own
// deadline, so a wait reports which milestone never came instead of the
// binary's timeout panic, which names no milestone.
const replayQuestionReportMargin = 30 * time.Second

// replayQuestionDeadline is the failure deadline of one milestone wait:
// peerReplayTimeout, the bound the product itself puts on a peer-up replay's
// stored-page phase, or the test binary's deadline less the report margin when
// that comes first. Load only delays a milestone; one not reached within the
// replay's own bound is a hang, and it fails here instead of at the package
// timeout. A product deadline that could fake a milestone (the 30 s
// cached-write bound ends a held relay too) is refused by each test on state,
// never by this bound.
func replayQuestionDeadline(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	limit := time.Now().Add(peerReplayTimeout)
	if deadline, bounded := t.Deadline(); bounded {
		margin := min(replayQuestionReportMargin, time.Until(deadline)/4)
		if report := deadline.Add(-margin); report.Before(limit) {
			limit = report
		}
	}
	return context.WithDeadline(t.Context(), limit)
}

// replayQuestionEnter waits for one producer to enter its held relay. A
// producer whose lifecycle reaches its final readiness report has passed its
// relay phase, so a finish before the entry fails at once rather than at the
// deadline: the producer skipped its relay and will never enter it.
func replayQuestionEnter(t *testing.T, entered, finished <-chan struct{}, name string) {
	t.Helper()
	wait, cancel := replayQuestionDeadline(t)
	defer cancel()
	select {
	case <-entered:
	case <-finished:
		t.Fatal(name + " finished its replay without entering its held relay")
	case <-wait.Done():
		t.Fatal(name + " did not enter its held relay within the replay bound")
	}
}

// replayQuestionWait names an actual lifecycle milestone with a bounded failure
// wait. The bound is a test failure deadline, never a success condition.
func replayQuestionWait(t *testing.T, milestone <-chan struct{}, name string) {
	t.Helper()
	wait, cancel := replayQuestionDeadline(t)
	defer cancel()
	select {
	case <-milestone:
	case <-wait.Done():
		t.Fatal(name + " did not finish within the replay bound")
	}
}

// replayQuestionResult is replayQuestionWait for a milestone that carries the
// error its producer returned, which the caller then asserts.
func replayQuestionResult(t *testing.T, result <-chan error, name string) error {
	t.Helper()
	wait, cancel := replayQuestionDeadline(t)
	defer cancel()
	select {
	case err := <-result:
		return err
	case <-wait.Done():
		t.Fatal(name + " did not finish within the replay bound")
		return nil
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
			State: rpc.SessionStateUp, InitialReplay: receipt, IsIBGP: true, RouteReflectorClient: true,
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
	wait, stopWait := replayQuestionDeadline(t)
	defer stopWait()
	select {
	case err := <-joined:
		t.Fatalf("join acknowledged an escaped original page: %v", err)
	case <-ctx.entered:
	case <-wait.Done():
		t.Fatal("replay join did not begin within the replay bound")
	}
	release.Do(func() { close(firstRelease) })
	replayQuestionWait(t, firstWritten, "original relay return")
	require.NoError(t, replayQuestionResult(t, joined, "replay join observing original completion"))
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
	readyReturned := make(chan struct{})
	var release sync.Once
	role, start := newReplayQuestion(t, func(context.Context, []rpc.StoredRoute) error { return nil },
		func(ctx context.Context) error {
			defer close(readyReturned)
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
	wait, stopWait := replayQuestionDeadline(t)
	defer stopWait()
	select {
	case err := <-joined:
		t.Fatalf("join passed the held readiness RPC: %v", err)
	case <-ctx.entered:
	case <-wait.Done():
		t.Fatal("replay join did not begin within the replay bound")
	}
	cancel()
	require.ErrorIs(t, replayQuestionResult(t, joined, "cancelled replay join refusal"), context.Canceled)
	// The refusal MUST come from the join's own cancellation, while the
	// readiness RPC is still held: a join that ignored cancellation and
	// returned its context error once the RPC ended would pass the check above.
	select {
	case <-readyReturned:
		t.Fatal("cancelled replay join refused only after the held readiness RPC returned")
	default:
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
	cancelled := make(chan error, 2)
	// A held producer never reaches its final readiness report before stop,
	// and a cancelled one returns before it, so any report means a producer
	// passed its relay phase without entering the held relay.
	finished := make(chan struct{}, 3)
	var writes atomic.Uint64
	role, start := newReplayQuestion(t, func(ctx context.Context, _ []rpc.StoredRoute) error {
		writes.Add(1)
		entered <- struct{}{}
		select {
		case <-ctx.Done():
			cancelled <- ctx.Err()
			return ctx.Err()
		case <-t.Context().Done():
			return t.Context().Err()
		}
	}, func(context.Context) error {
		finished <- struct{}{}
		return nil
	})
	// The fixture's store answers each page read with the next message id of
	// the same route. The second producer starts after the first entered, so
	// it reads the newer message and its admission supersedes the held one.
	// Started together, the producer that read the older message could admit
	// last, and admission rightly refuses a page older than the inventory
	// (putWithdrawalLocked keeps the higher MsgID): it would skip its relay.
	start("192.0.2.252", 174, 204)
	replayQuestionEnter(t, entered, finished, "first active producer")
	start("192.0.2.253", 175, 205)
	replayQuestionEnter(t, entered, finished, "second active producer")
	stopped := make(chan struct{})
	go func() { role.replays.stop(); close(stopped) }()
	replayQuestionWait(t, stopped, "cancelled replay lifecycle owner")
	// Each held relay also ends at its own cached-write deadline. Only
	// context.Canceled proves stop cancelled it; DeadlineExceeded means stop
	// waited for the deadline instead.
	for _, producer := range []string{"first", "second"} {
		err := replayQuestionResult(t, cancelled, producer+" actual producer cancellation")
		require.ErrorIs(t, err, context.Canceled, "%s producer ended by its own deadline, not by stop", producer)
	}
	start("192.0.2.254", 176, 206)
	require.Equal(t, uint64(2), writes.Load(), "stopped owner admits no new replay")
	require.Empty(t, role.replays.snapshot(), "completed lifecycles leave no history")
}
