package dnsresolver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// snapshotRequestEngine is the engine side of one resolver plugin connection.
// It answers every request the plugin sends, reports each method on seen so
// the test can follow the five-stage startup, and reports each system-dns
// snapshot-request emit on requests after answering it. With refuseRequest
// set it answers that emit with an error, as an engine that cannot route it
// would.
type snapshotRequestEngine struct {
	mux           *rpc.MuxConn
	refuseRequest bool
	seen          chan string
	requests      chan struct{}
}

// serve answers each plugin request until the connection closes or ctx ends:
// runtime state is absent, a boot count increments to one, Stage 1 is
// answered with the protocol version, an emit-event with one delivery (or the
// refusal for the snapshot-request), and every other request is acknowledged.
func (e *snapshotRequestEngine) serve(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-e.mux.Requests():
			if !ok || req == nil {
				return
			}
			e.answer(ctx, req)
			select {
			case e.seen <- req.Method:
			default:
			}
		}
	}
}

// answer sends the engine's answer to one plugin request.
func (e *snapshotRequestEngine) answer(ctx context.Context, req *rpc.Request) {
	switch req.Method {
	case rpc.MethodStateIncrement:
		_ = e.mux.SendResult(ctx, req.ID, rpc.StateOutput{Status: rpc.StateOK, Value: 1})
	case rpc.MethodStateGet:
		_ = e.mux.SendResult(ctx, req.ID, rpc.StateOutput{Status: rpc.StateAbsent})
	case rpc.MethodDeclareRegistration:
		_ = e.mux.SendResult(ctx, req.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion})
	case rpc.MethodEmitEvent:
		var in rpc.EmitEventInput
		_ = json.Unmarshal(req.Params, &in)
		isRequest := in.Namespace == dnssource.Namespace && in.EventType == dnssource.SnapshotRequestEventType
		if isRequest && e.refuseRequest {
			_ = e.mux.SendError(ctx, req.ID, "no subscriber routes the system-dns snapshot-request")
		} else {
			_ = e.mux.SendResult(ctx, req.ID, rpc.EmitEventOutput{Delivered: 1})
		}
		if isRequest {
			e.requests <- struct{}{}
		}
	default:
		_ = e.mux.SendOK(ctx, req.ID)
	}
}

// waitFor blocks until the plugin has sent method.
func (e *snapshotRequestEngine) waitFor(t *testing.T, ctx context.Context, method string) {
	t.Helper()
	for {
		select {
		case got := <-e.seen:
			if got == method {
				return
			}
		case <-ctx.Done():
			t.Fatalf("waiting for %s: %v", method, ctx.Err())
		}
	}
}

// callback sends one engine-to-plugin callback and fails on an error answer.
func (e *snapshotRequestEngine) callback(t *testing.T, ctx context.Context, method string, input any) {
	t.Helper()
	raw, err := e.mux.CallRPC(ctx, method, input)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	if len(raw) == 0 {
		return
	}
	var status struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatalf("%s answer %s: %v", method, raw, err)
	}
	if status.Status == rpc.StatusError {
		t.Fatalf("%s refused: %s", method, status.Error)
	}
}

// startFollowingResolver runs a resolver's learned-resolver wiring
// (followLearnedResolvers, the call runResolverPlugin makes) on a real SDK
// plugin over a pipe, drives the five-stage startup, and returns once the
// plugin sent ready, so OnStarted runs next. The plugin stops at cleanup.
func startFollowingResolver(t *testing.T, refuseRequest bool) (*resolver, *snapshotRequestEngine) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	pluginEnd, engineEnd := net.Pipe()
	eng := &snapshotRequestEngine{
		mux:           rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)),
		refuseRequest: refuseRequest,
		seen:          make(chan string, 64),
		requests:      make(chan struct{}, 8),
	}
	go eng.serve(ctx)

	res := newResolver(time.Now)
	p := sdk.NewWithConn("dnsresolver", pluginEnd)
	followLearnedResolvers(p, res, slog.New(slog.NewTextHandler(io.Discard, nil)))
	done := make(chan struct{})
	go func() {
		_ = p.Run(ctx, &sdk.Registration{})
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		_ = eng.mux.Close()
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the resolver plugin did not exit after its transport closed")
		}
		_ = p.Close()
	})

	eng.waitFor(t, ctx, rpc.MethodDeclareRegistration)
	eng.callback(t, ctx, "ze-plugin-callback:configure", rpc.ConfigureInput{})
	eng.waitFor(t, ctx, "ze-plugin-engine:declare-capabilities")
	eng.callback(t, ctx, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	eng.waitFor(t, ctx, "ze-plugin-engine:ready")
	return res, eng
}

// awaitOneSnapshotRequest fails unless the plugin emits exactly one
// system-dns snapshot-request: one within a second of ready, and no second
// one in the half second after it.
func awaitOneSnapshotRequest(t *testing.T, eng *snapshotRequestEngine) {
	t.Helper()
	select {
	case <-eng.requests:
	case <-time.After(time.Second):
		t.Fatal("the resolver sent no system-dns snapshot-request when it started: a resolver restarted alone forwards to no learned resolver until the next lease change")
	}
	select {
	case <-eng.requests:
		t.Fatal("the resolver sent a second system-dns snapshot-request at start")
	case <-time.After(500 * time.Millisecond):
	}
}

// TestStartRequestsSnapshot: spec-home-cpe-23b AC-13. A resolver that starts
// alone (the failure-policy respawn) asks the engine for the current
// system-dns snapshot itself, once, and holds queries for the replay that
// request earns. Method: run followLearnedResolvers on a real SDK plugin
// against a fake engine that answers the request; count the snapshot-request
// emits, then check a query (learned.AwaitReplay, the first step of admit) is
// still held 300 ms later, since no snapshot was delivered.
func TestStartRequestsSnapshot(t *testing.T) {
	res, eng := startFollowingResolver(t, false)
	awaitOneSnapshotRequest(t, eng)

	released := make(chan struct{})
	go func() {
		res.learned.AwaitReplay()
		close(released)
	}()
	select {
	case <-released:
		t.Fatal("a query was released after a sent snapshot-request and before any snapshot arrived: the replay hold is not armed")
	case <-time.After(300 * time.Millisecond):
	}
}

// TestStartRequestRefusedReleasesHold: spec-home-cpe-23b AC-13. When the
// engine refuses the start's snapshot-request, no replay can answer it, so
// the resolver stops holding queries at once rather than for the whole
// dnssource.ReplayWaitMax bound. Method: the fake engine answers the request
// with an error; after the single request, learned.AwaitReplay must return
// well inside the bound.
func TestStartRequestRefusedReleasesHold(t *testing.T) {
	res, eng := startFollowingResolver(t, true)
	awaitOneSnapshotRequest(t, eng)

	start := time.Now()
	res.learned.AwaitReplay()
	if waited := time.Since(start); waited > dnssource.ReplayWaitMax/2 {
		t.Fatalf("a query waited %v after the snapshot-request was refused: the replay hold stayed armed for a replay that cannot arrive", waited)
	}
}
