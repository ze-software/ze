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
// onEvent callback (engine-to-plugin) block the entire event loop while waiting for
// the engine to respond on plugin-to-engine.
//
// PREVENTS: Regression if the blocking pattern is fixed — the test documents
// the expected latency characteristic. When fixed, the second event delivery
// should complete promptly regardless of plugin-to-engine response time.
//
// Architecture:
//
//	Plugin (goroutine)          Fake Engine (test)
//	  plugin-to-engine (plugin→engine)    engineEnd (reads requests, sends responses)
//	  engine-to-plugin (engine→plugin)    engineEnd (sends deliver-event RPCs)
//
// The blocking occurs because:
// 1. Engine sends "state up" event via engine-to-plugin deliver-event
// 2. Plugin's onEvent callback calls handleState → replayRoutes → updateRoute
// 3. updateRoute calls plugin.UpdateRoute which sends RPC on plugin-to-engine and WAITS
// 4. While waiting, engine-to-plugin's event loop can't process any new events
// 5. Engine's next deliver-event on engine-to-plugin blocks on callMu (serialized writes).
func TestRIBPluginEventLoopBlocking(t *testing.T) {
	// Create two net.Pipe pairs for plugin-to-engine and engine-to-plugin.
	// Plugin gets one end, fake engine gets the other.
	pluginEnd, engineEnd := net.Pipe()

	// Wrap engine ends in rpc.Conn for structured RPC communication.
	engineConn := rpc.NewConn(engineEnd, engineEnd)
	mux := rpc.NewMuxConn(engineConn)

	// Start the rib plugin in a goroutine — it will run the 5-stage protocol
	// then enter the event loop.
	pluginDone := make(chan int, 1)
	go func() {
		pluginDone <- RunRIBPlugin(pluginEnd)
	}()

	ctx := context.Background()

	// ── 5-Stage Handshake (fake engine side) ─────────────────────────────

	// Stage 1: Read declare-registration from plugin-to-engine, send OK.
	stage1Req := readMuxRequestTimeout(t, ctx, mux)
	require.Equal(t, "ze-plugin-engine:declare-registration", stage1Req.Method)
	require.NoError(t, mux.SendOK(ctx, stage1Req.ID))

	// Stage 2: Send configure on engine-to-plugin (empty config is fine).
	configInput := &rpc.ConfigureInput{Sections: []rpc.ConfigSection{}}
	raw, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", configInput)
	require.NoError(t, err)
	_ = raw // CallRPC returns errors directly; result unused for ok-only RPCs

	// Stage 3: Read declare-capabilities from plugin-to-engine, send OK.
	stage3Req := readMuxRequestTimeout(t, ctx, mux)
	require.Equal(t, "ze-plugin-engine:declare-capabilities", stage3Req.Method)
	require.NoError(t, mux.SendOK(ctx, stage3Req.ID))

	// Stage 4: Send share-registry on engine-to-plugin (empty registry).
	registryInput := &rpc.ShareRegistryInput{Commands: []rpc.RegistryCommand{}}
	raw, err = mux.CallRPC(ctx, "ze-plugin-callback:share-registry", registryInput)
	require.NoError(t, err)
	_ = raw // CallRPC returns errors directly; result unused for ok-only RPCs

	// Stage 5: Read ready from plugin-to-engine, send OK.
	// The ready RPC includes startup subscriptions (events: update direction sent, state, refresh).
	stage5Req := readMuxRequestTimeout(t, ctx, mux)
	require.Equal(t, "ze-plugin-engine:ready", stage5Req.Method)
	require.NoError(t, mux.SendOK(ctx, stage5Req.ID))

	// ── Plugin is now in event loop ──────────────────────────────────────

	// Step 1: Send a "sent" event to populate ribOut with a route.
	// This is a "type":"sent" event — the rib plugin stores it in ribOut.
	sentEvent := `{"type":"sent","msg-id":1,"peer":{"address":"10.0.0.1","remote":{"as":65001}},"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24"]}]}`
	deliverEventSync(t, ctx, mux, sentEvent)

	// Step 2: Start a goroutine to handle update-route requests on plugin-to-engine.
	// We add a deliberate delay to simulate real engine response time.
	// This delay is what causes the head-of-line blocking.
	const updateRouteDelay = 500 * time.Millisecond
	var updateRouteCount atomic.Int32
	handlerCtx, cancelHandler := context.WithCancel(ctx)
	defer cancelHandler()

	go func() {
		for {
			var req *rpc.Request
			select {
			case r, ok := <-mux.Requests():
				if !ok {
					return
				}
				req = r
			case <-handlerCtx.Done():
				return
			}

			if req.Method == "ze-plugin-engine:update-route" {
				// Simulate engine processing time — this is the delay that
				// blocks the plugin's event loop via synchronous updateRoute.
				time.Sleep(updateRouteDelay)
				updateRouteCount.Add(1)
				result := &rpc.UpdateRouteOutput{PeersAffected: 1, RoutesSent: 1}
				if sendErr := mux.SendResult(handlerCtx, req.ID, result); sendErr != nil {
					t.Logf("SendResult failed: %v", sendErr)
					return
				}
			} else {
				if sendErr := mux.SendOK(handlerCtx, req.ID); sendErr != nil {
					t.Logf("SendOK failed: %v", sendErr)
					return
				}
			}
		}
	}()

	// Step 3: Deliver "state up" event and "probe" event concurrently.
	//
	// The state-up event triggers replayRoutes → updateRoute (synchronous RPC on plugin-to-engine).
	// While that blocks, the probe event can't be delivered because:
	//   (a) mux.CallRPC serializes via callMu (engine-side blocking)
	//   (b) SDK's eventLoop is single-threaded (plugin-side blocking)
	//
	// Both effects compound, but (b) is the root cause we're testing:
	// even if the engine could send the probe, the plugin couldn't process it
	// until replayRoutes finishes.

	stateUpEvent := `{"type":"state","peer":{"address":"10.0.0.1","remote":{"as":65001}},"state":"up"}`
	probeEvent := `{"type":"sent","msg-id":2,"peer":{"address":"10.0.0.1","remote":{"as":65001}},"ipv4/unicast":[{"next-hop":"2.2.2.2","action":"add","nlri":["10.0.1.0/24"]}]}`

	type deliverResult struct {
		duration time.Duration
		err      error
	}

	stateUpDone := make(chan deliverResult, 1)
	probeDone := make(chan deliverResult, 1)

	// Goroutine 1: deliver state-up event (triggers blocking replayRoutes)
	go func() {
		start := time.Now()
		callErr := deliverEvent(mux, stateUpEvent)
		stateUpDone <- deliverResult{duration: time.Since(start), err: callErr}
	}()

	// Give plugin time to start processing the state-up event before sending the probe.
	time.Sleep(50 * time.Millisecond)

	// Goroutine 2: deliver probe event — blocked by head-of-line blocking
	go func() {
		start := time.Now()
		callErr := deliverEvent(mux, probeEvent)
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
			"the event loop is blocked while updateRoute waits for plugin-to-engine response")

	// Verify that update-route RPCs were actually processed.
	// replayRoutes sends: N routes + 1 "plugin session ready" command.
	// We populated 1 route, so expect at least 2 update-route calls.
	assert.GreaterOrEqual(t, updateRouteCount.Load(), int32(2),
		"should have processed at least 2 update-route RPCs (1 route + plugin session ready)")

	// ── Cleanup ──────────────────────────────────────────────────────────
	cancelHandler()

	if closeErr := mux.Close(); closeErr != nil {
		t.Logf("mux close: %v", closeErr)
	}
	if closeErr := engineEnd.Close(); closeErr != nil {
		t.Logf("engineEnd close: %v", closeErr)
	}

	select {
	case exitCode := <-pluginDone:
		t.Logf("plugin exited with code %d", exitCode)
	case <-time.After(5 * time.Second):
		t.Error("plugin did not exit within timeout")
	}
}

// deliverEvent sends a deliver-event RPC on engine-to-plugin and waits for the response.
// Returns an error rather than failing the test, so it's safe for goroutine use.
func deliverEvent(mux *rpc.MuxConn, event string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	input := &rpc.DeliverEventInput{Event: event}
	_, err := mux.CallRPC(ctx, "ze-plugin-callback:deliver-event", input)
	return err
}

// deliverEventSync sends a deliver-event RPC and fails the test on error.
func deliverEventSync(t *testing.T, ctx context.Context, mux *rpc.MuxConn, event string) {
	t.Helper()
	input := &rpc.DeliverEventInput{Event: event}
	_, err := mux.CallRPC(ctx, "ze-plugin-callback:deliver-event", input)
	require.NoError(t, err, "deliver-event should succeed")
}

// readMuxRequestTimeout reads the next plugin request with a 5-second timeout.
func readMuxRequestTimeout(t *testing.T, ctx context.Context, mux *rpc.MuxConn) *rpc.Request {
	t.Helper()
	timeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	select {
	case req := <-mux.Requests():
		return req
	case <-timeout.Done():
		t.Fatal("timed out waiting for plugin request")
		return nil
	}
}
