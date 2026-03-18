package rib

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestRIBPluginEventLoopBlocking demonstrates the head-of-line blocking bug
// in the rib plugin's event loop.
//
// VALIDATES: The rib plugin's synchronous updateRoute calls from within the
// onEvent callback (Socket B) block the entire event loop while waiting for
// the engine to respond on Socket A.
//
// PREVENTS: Regression if the blocking pattern is fixed — the test documents
// the expected latency characteristic. When fixed, the second event delivery
// should complete promptly regardless of Socket A response time.
//
// Architecture:
//
//	Plugin (goroutine)          Fake Engine (test)
//	  Socket A (plugin→engine)    engineA (reads requests, sends responses)
//	  Socket B (engine→plugin)    engineB (sends deliver-event RPCs)
//
// The blocking occurs because:
// 1. Engine sends "state up" event via Socket B deliver-event
// 2. Plugin's onEvent callback calls handleState → replayRoutes → updateRoute
// 3. updateRoute calls plugin.UpdateRoute which sends RPC on Socket A and WAITS
// 4. While waiting, Socket B's event loop can't process any new events
// 5. Engine's next deliver-event on Socket B blocks on callMu (serialized writes).
func TestRIBPluginEventLoopBlocking(t *testing.T) {
	// Create two net.Pipe pairs for Socket A and Socket B.
	// Plugin gets one end, fake engine gets the other.
	pluginA, engineA := net.Pipe()
	pluginB, engineB := net.Pipe()

	// Wrap engine ends in rpc.Conn for structured RPC communication.
	connA := rpc.NewConn(engineA, engineA) // Engine reads/writes on Socket A
	connB := rpc.NewConn(engineB, engineB) // Engine reads/writes on Socket B

	// Start the rib plugin in a goroutine — it will run the 5-stage protocol
	// then enter the event loop.
	pluginDone := make(chan int, 1)
	go func() {
		pluginDone <- RunRIBPlugin(pluginA, pluginB)
	}()

	ctx := context.Background()

	// ── 5-Stage Handshake (fake engine side) ─────────────────────────────

	// Stage 1: Read declare-registration from Socket A, send OK.
	stage1Req := readRequestTimeout(t, ctx, connA)
	require.Equal(t, "ze-plugin-engine:declare-registration", stage1Req.Method)
	require.NoError(t, connA.SendOK(ctx, stage1Req.ID))

	// Stage 2: Send configure on Socket B (empty config is fine).
	configInput := &rpc.ConfigureInput{Sections: []rpc.ConfigSection{}}
	raw, err := connB.CallRPC(ctx, "ze-plugin-callback:configure", configInput)
	require.NoError(t, err)
	_ = raw // CallRPC returns errors directly; result unused for ok-only RPCs

	// Stage 3: Read declare-capabilities from Socket A, send OK.
	stage3Req := readRequestTimeout(t, ctx, connA)
	require.Equal(t, "ze-plugin-engine:declare-capabilities", stage3Req.Method)
	require.NoError(t, connA.SendOK(ctx, stage3Req.ID))

	// Stage 4: Send share-registry on Socket B (empty registry).
	registryInput := &rpc.ShareRegistryInput{Commands: []rpc.RegistryCommand{}}
	raw, err = connB.CallRPC(ctx, "ze-plugin-callback:share-registry", registryInput)
	require.NoError(t, err)
	_ = raw // CallRPC returns errors directly; result unused for ok-only RPCs

	// Stage 5: Read ready from Socket A, send OK.
	// The ready RPC includes startup subscriptions (events: update direction sent, state, refresh).
	stage5Req := readRequestTimeout(t, ctx, connA)
	require.Equal(t, "ze-plugin-engine:ready", stage5Req.Method)
	require.NoError(t, connA.SendOK(ctx, stage5Req.ID))

	// ── Plugin is now in event loop ──────────────────────────────────────

	// Step 1: Send a "sent" event to populate ribOut with a route.
	// This is a "type":"sent" event — the rib plugin stores it in ribOut.
	sentEvent := `{"type":"sent","msg-id":1,"peer":{"address":"10.0.0.1","asn":65001},"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24"]}]}`
	deliverEventSync(t, ctx, connB, sentEvent)

	// Step 2: Start a goroutine to handle update-route requests on Socket A.
	// We add a deliberate delay to simulate real engine response time.
	// This delay is what causes the head-of-line blocking.
	const updateRouteDelay = 500 * time.Millisecond
	var updateRouteCount atomic.Int32
	handlerCtx, cancelHandler := context.WithCancel(ctx)
	defer cancelHandler()

	go func() {
		for {
			req, readErr := connA.ReadRequest(handlerCtx)
			if readErr != nil {
				return // Connection closed or context canceled
			}

			if req.Method == "ze-plugin-engine:update-route" {
				// Simulate engine processing time — this is the delay that
				// blocks the plugin's event loop via synchronous updateRoute.
				time.Sleep(updateRouteDelay)
				updateRouteCount.Add(1)
				result := &rpc.UpdateRouteOutput{PeersAffected: 1, RoutesSent: 1}
				if sendErr := connA.SendResult(handlerCtx, req.ID, result); sendErr != nil {
					t.Logf("SendResult failed: %v", sendErr)
					return
				}
			} else {
				if sendErr := connA.SendOK(handlerCtx, req.ID); sendErr != nil {
					t.Logf("SendOK failed: %v", sendErr)
					return
				}
			}
		}
	}()

	// Step 3: Deliver "state up" event and "probe" event concurrently.
	//
	// The state-up event triggers replayRoutes → updateRoute (synchronous RPC on Socket A).
	// While that blocks, the probe event can't be delivered because:
	//   (a) connB.CallRPC serializes via callMu (engine-side blocking)
	//   (b) SDK's eventLoop is single-threaded (plugin-side blocking)
	//
	// Both effects compound, but (b) is the root cause we're testing:
	// even if the engine could send the probe, the plugin couldn't process it
	// until replayRoutes finishes.

	stateUpEvent := `{"type":"state","peer":{"address":"10.0.0.1","asn":65001},"state":"up"}`
	probeEvent := `{"type":"sent","msg-id":2,"peer":{"address":"10.0.0.1","asn":65001},"ipv4/unicast":[{"next-hop":"2.2.2.2","action":"add","nlri":["10.0.1.0/24"]}]}`

	type deliverResult struct {
		duration time.Duration
		err      error
	}

	stateUpDone := make(chan deliverResult, 1)
	probeDone := make(chan deliverResult, 1)

	// Goroutine 1: deliver state-up event (triggers blocking replayRoutes)
	go func() {
		start := time.Now()
		callErr := deliverEvent(connB, stateUpEvent)
		stateUpDone <- deliverResult{duration: time.Since(start), err: callErr}
	}()

	// Give plugin time to start processing the state-up event before sending the probe.
	time.Sleep(50 * time.Millisecond)

	// Goroutine 2: deliver probe event — blocked by head-of-line blocking
	go func() {
		start := time.Now()
		callErr := deliverEvent(connB, probeEvent)
		probeDone <- deliverResult{duration: time.Since(start), err: callErr}
	}()

	// Wait for both with timeout
	var stateResult, probeResult deliverResult
	select {
	case stateResult = <-stateUpDone:
	case <-time.After(30 * time.Second):
		t.Fatal("state-up delivery timed out")
	}
	select {
	case probeResult = <-probeDone:
	case <-time.After(30 * time.Second):
		t.Fatal("probe delivery timed out")
	}

	require.NoError(t, stateResult.err, "state-up delivery should succeed")
	require.NoError(t, probeResult.err, "probe delivery should succeed")

	// ── Assertions ───────────────────────────────────────────────────────

	t.Logf("state-up delivery: %v", stateResult.duration)
	t.Logf("probe delivery: %v", probeResult.duration)
	t.Logf("update-route RPCs processed: %d", updateRouteCount.Load())

	// The state-up delivery itself must take at least the updateRoute delay,
	// because replayRoutes sends the route + "plugin session ready" synchronously.
	// With 1 route, that's 2 updateRoute calls × 500ms = ~1000ms.
	assert.Greater(t, stateResult.duration, updateRouteDelay,
		"state-up delivery should take at least one updateRoute delay")

	// The probe delivery must be delayed by head-of-line blocking.
	// It was submitted 50ms after the state-up event. If the event loop were
	// non-blocking, it would complete in ~milliseconds. Instead, it must wait
	// for replayRoutes to finish all its synchronous updateRoute calls.
	assert.Greater(t, probeResult.duration, updateRouteDelay,
		"probe event should be delayed by head-of-line blocking: "+
			"the event loop is blocked while updateRoute waits for Socket A response")

	// Verify that update-route RPCs were actually processed.
	// replayRoutes sends: N routes + 1 "plugin session ready" command.
	// We populated 1 route, so expect at least 2 update-route calls.
	assert.GreaterOrEqual(t, updateRouteCount.Load(), int32(2),
		"should have processed at least 2 update-route RPCs (1 route + plugin session ready)")

	// ── Cleanup ──────────────────────────────────────────────────────────
	cancelHandler()

	if closeErr := connA.Close(); closeErr != nil {
		t.Logf("connA close: %v", closeErr)
	}
	if closeErr := connB.Close(); closeErr != nil {
		t.Logf("connB close: %v", closeErr)
	}
	if closeErr := engineA.Close(); closeErr != nil {
		t.Logf("engineA close: %v", closeErr)
	}
	if closeErr := engineB.Close(); closeErr != nil {
		t.Logf("engineB close: %v", closeErr)
	}

	select {
	case exitCode := <-pluginDone:
		t.Logf("plugin exited with code %d", exitCode)
	case <-time.After(5 * time.Second):
		t.Error("plugin did not exit within timeout")
	}
}

// deliverEvent sends a deliver-event RPC on Socket B and waits for the response.
// Returns an error rather than failing the test, so it's safe for goroutine use.
func deliverEvent(conn *rpc.Conn, event string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	input := &rpc.DeliverEventInput{Event: event}
	_, err := conn.CallRPC(ctx, "ze-plugin-callback:deliver-event", input)
	return err
}

// deliverEventSync sends a deliver-event RPC and fails the test on error.
// Only call from the main test goroutine (not from spawned goroutines).
func deliverEventSync(t *testing.T, ctx context.Context, conn *rpc.Conn, event string) {
	t.Helper()
	input := &rpc.DeliverEventInput{Event: event}
	_, err := conn.CallRPC(ctx, "ze-plugin-callback:deliver-event", input)
	require.NoError(t, err, "deliver-event should succeed")
}

// readRequestTimeout reads the next RPC request with a 5-second timeout.
func readRequestTimeout(t *testing.T, ctx context.Context, conn *rpc.Conn) *rpc.Request {
	t.Helper()
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := conn.ReadRequest(timeoutCtx)
	require.NoError(t, err, "should read RPC request within timeout")
	return req
}
