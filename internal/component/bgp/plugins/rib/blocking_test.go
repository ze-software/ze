package rib

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRIBPluginSentEventsSurviveReplay exercises the external SDK event path.
// A delayed replay RPC must not lose the locally originated route reported by
// another sent event: a subsequent refresh must reproduce both exact prefixes.
func TestRIBPluginSentEventsSurviveReplay(t *testing.T) {
	// The real plugin and its engine-facing RPC consumer share one duplex pipe.
	pluginEnd, engineEnd := net.Pipe()

	// Wrap engine ends in rpc.Conn for structured RPC communication.
	engineConn := rpc.NewConn(engineEnd, engineEnd)
	mux := rpc.NewMuxConn(engineConn)

	// Start the rib plugin in a goroutine — it will run the 5-stage protocol
	// then enter the event loop.
	pluginDone := make(chan int, 1)
	go func() {
		pluginDone <- runRIBPlugin(pluginEnd)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(func() {
		cancel()
		_ = mux.Close()
		_ = engineEnd.Close()
		select {
		case <-pluginDone:
		case <-time.After(5 * time.Second):
			t.Error("plugin did not exit after transport closure")
		}
	})

	// ── 5-Stage Handshake (fake engine side) ─────────────────────────────

	// Stage 1: Read declare-registration from plugin-to-engine, send OK.
	stage1Req := readMuxRequestTimeout(t, ctx, mux)
	require.Equal(t, rpc.MethodDeclareRegistration, stage1Req.Method)
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

	// Step 0: Mark the peer as one this plugin has already seen a session for.
	//
	// Adj-RIB-Out replay is defined only for a RE-ESTABLISHED session: a peer's
	// first session has been advertised nothing, so entries recorded for it were
	// produced by that same session's own sends and replaying them puts a second
	// copy of the route on the wire (collectPeerUpReplay, rib_replay.go). Without
	// this earlier down event, state-up is a first session and replays nothing.
	// The down event records the peer as known without transitioning a session.
	deliverEventSync(t, ctx, mux,
		`{"type":"state","peer":{"address":"10.0.0.1","remote":{"address":"10.0.0.1","as":65001}},"state":"down"}`)

	// Step 1: Send a "sent" event to populate ribOut with a route.
	// This is a "type":"sent" event — the rib plugin stores it in ribOut.
	sentEvent := `{"type":"sent","msg-id":1,"peer":{"address":"10.0.0.1","remote":{"address":"10.0.0.1","as":65001}},"route-meta":{"source-local":true},"origin":"igp","ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24"]}]}`
	deliverEventSync(t, ctx, mux, sentEvent)

	firstUpdateRoute := make(chan struct{})
	releaseReplay := make(chan struct{})
	var commandsMu sync.Mutex
	var commands []string
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		first := true
		for {
			var req *rpc.Request
			select {
			case r, ok := <-mux.Requests():
				if !ok {
					return
				}
				req = r
			case <-ctx.Done():
				return
			}
			switch req.Method {
			case "ze-plugin-engine:update-route":
				var input rpc.UpdateRouteInput
				if err := json.Unmarshal(req.Params, &input); err != nil {
					t.Errorf("decode update-route: %v", err)
					return
				}
				commandsMu.Lock()
				commands = append(commands, input.Command)
				commandsMu.Unlock()
				if first {
					first = false
					close(firstUpdateRoute)
					select {
					case <-releaseReplay:
					case <-ctx.Done():
						return
					}
				}
				if err := mux.SendResult(ctx, req.ID, &rpc.UpdateRouteOutput{Announced: 1}); err != nil {
					t.Errorf("answer update-route: %v", err)
					return
				}
			case rpc.MethodDispatchCommand, rpc.MethodDispatchCommandArgs:
				if err := rpc.WriteDocumentAnswer(mux.AnswerWriter(ctx), req.ID, rpc.AnswerTail{}, nil); err != nil {
					t.Errorf("answer command: %v", err)
					return
				}
			default:
				t.Errorf("unexpected engine request %q", req.Method)
				return
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-handlerDone:
		case <-time.After(5 * time.Second):
			t.Error("engine request handler did not exit after cancellation")
		}
	})

	stateUpEvent := `{"type":"state","peer":{"address":"10.0.0.1","remote":{"address":"10.0.0.1","as":65001}},"state":"up","initial-replay":"1"}`
	probeEvent := `{"type":"sent","msg-id":2,"peer":{"address":"10.0.0.1","remote":{"address":"10.0.0.1","as":65001}},"route-meta":{"source-local":true},"origin":"igp","ipv4/unicast":[{"next-hop":"2.2.2.2","action":"add","nlri":["10.0.1.0/24"]}]}`
	stateUpDone := make(chan error, 1)
	go func() {
		stateUpDone <- deliverEvent(mux, stateUpEvent)
	}()
	select {
	case <-firstUpdateRoute:
	case <-ctx.Done():
		t.Fatal("state-up did not reach replay")
	}
	probeDone := make(chan error, 1)
	probeStarted := make(chan struct{})
	go func() {
		close(probeStarted)
		probeDone <- deliverEvent(mux, probeEvent)
	}()
	<-probeStarted
	close(releaseReplay)
	select {
	case err := <-stateUpDone:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("state-up delivery timed out")
	}
	select {
	case err := <-probeDone:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("sent-event delivery timed out")
	}

	// Start a separate consumer observation after both deliveries finish. Replay
	// cursor commands are not evidence that either route survived in Adj-RIB-Out.
	commandsMu.Lock()
	commands = nil
	commandsMu.Unlock()
	deliverEventSync(t, ctx, mux,
		`{"type":"refresh","peer":{"address":"10.0.0.1","remote":{"address":"10.0.0.1","as":65001}},"afi":"ipv4","safi":"unicast"}`)
	commandsMu.Lock()
	refreshCommands := slices.Clone(commands)
	commandsMu.Unlock()
	var routes []refreshRouteIdentity
	for _, command := range refreshCommands {
		routes = append(routes, consumedRefreshRoutes(t, parseRefreshCommand(t, command))...)
	}
	require.ElementsMatch(t, []refreshRouteIdentity{
		{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")},
		{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.1.0/24")},
	}, routes, "refresh must retain both the replayed route and the later sent event")
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
