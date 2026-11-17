// Design: docs/architecture/api/process-protocol.md -- selected socket cancellation.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type selectedCancellationReactor struct {
	*mockReactor
	started  chan struct{}
	canceled chan struct{}
	release  <-chan struct{}
	parent   context.Context
}

func (reactor *selectedCancellationReactor) SelectRoutes(ctx context.Context, _ *plugin.SelectedBatch, _ plugin.SelectedOwner) (*rpc.SelectRoutesOutput, error) {
	close(reactor.started)
	<-ctx.Done()
	close(reactor.canceled)
	if reactor.release != nil {
		select {
		case <-reactor.release:
		case <-reactor.parent.Done():
		}
	}
	return nil, ctx.Err()
}

// TestSelectedSocketCancellationJoinsWriter runs the real socket dispatcher
// against a held writer seam. The RPC may return its caller's cancellation
// only after that writer releases its final obligation, without ending another
// process or the daemon context.
func TestSelectedSocketCancellationJoinsWriter(t *testing.T) {
	work, stop := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
	t.Cleanup(stop)
	release := make(chan struct{})
	reactor := &selectedCancellationReactor{mockReactor: &mockReactor{}, parent: work,
		started: make(chan struct{}), canceled: make(chan struct{}), release: release}
	host := &Server{ctx: work, reactor: reactor}
	proc := nativeOwnerProcess(t, host)
	host.UpdateDeliveryGraph(graphNS, []DeliveryPeer{{Addr: "192.0.2.7", Bindings: []plugin.PeerProcessBinding{{
		PluginName: proc.Name(), Send: map[string]bool{"update": true}}}}})
	client, engine := net.Pipe()
	peer := sdk.NewWithConn(proc.Name(), client)
	conn := plugipc.NewPluginConn(engine, engine)
	serverDone := make(chan struct{})
	go func() { host.serveRuntimeRequests(proc, conn); close(serverDone) }()
	t.Cleanup(func() {
		if err := peer.Close(); err != nil {
			t.Error(err)
		}
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		select {
		case <-serverDone:
		case <-work.Done():
			t.Error("socket dispatcher did not join")
		}
	})
	ctx, cancel := context.WithCancel(work)
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := peer.SelectRoutes(ctx, selectedServerInput()); finished <- err }()
	select {
	case <-reactor.started:
	case <-work.Done():
		t.Fatal(work.Err())
	}
	cancel()
	select {
	case <-reactor.canceled:
	case <-work.Done():
		t.Fatal("socket cancellation did not reach the writer")
	}
	select {
	case err := <-finished:
		t.Fatalf("writer is still held, RPC returned %v", err)
	default:
	}
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("caller cancellation lost: %v", err)
		}
		if _, unsettled := errors.AsType[*rpc.CancellationUnsettledError](err); unsettled {
			t.Fatalf("writer joined but cancellation was not acknowledged: %v", err)
		}
	case <-work.Done():
		t.Fatal(work.Err())
	}
	if host.Context().Err() != nil {
		t.Fatal("one request cancellation ended the daemon context")
	}
}

// TestRuntimeCancellationBeforeDispatch proves the original request is live
// before any goroutine can start its handler. A later cancel on the same
// connection sees and ends it, then waits for the exact admission to finish.
func TestRuntimeCancellationBeforeDispatch(t *testing.T) {
	work, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	requests := testRuntimeRequests(work, func(netip.Addr) bool { return true })
	defer func() { requests.stop(); requests.cancelAll() }()
	input := selectedServerInput()
	input.DeadlineMS = time.Now().Add(time.Second).UnixMilli()
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := requests.admit(&rpc.Request{ID: 1, Method: rpc.MethodSelectRoutes, Params: encoded})
	if err != nil {
		t.Fatal(err)
	}
	control, err := requests.admit(&rpc.Request{ID: 2, Method: rpc.MethodCancelRequest})
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		_, err := (&Server{}).opCancelRequest(control.ctx, nil, json.RawMessage(`{"request-id":1,"method":"ze-plugin-engine:select-routes"}`))
		control.finish()
		finished <- err
	}()
	select {
	case <-admission.ctx.Done():
	case <-work.Done():
		t.Fatal("cancel missed an admitted request before dispatch")
	}
	select {
	case err := <-finished:
		t.Fatalf("unfinished admission was acknowledged: %v", err)
	default:
	}
	admission.finish()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-work.Done():
		t.Fatal(work.Err())
	}
	requests.mu.Lock()
	held := len(requests.active) + len(requests.resources)
	requests.mu.Unlock()
	if held != 0 {
		t.Fatalf("completed requests retained %d map entries", held)
	}
	completedControl, err := requests.admit(&rpc.Request{ID: 3, Method: rpc.MethodCancelRequest})
	if err != nil {
		t.Fatal(err)
	}
	defer completedControl.finish()
	result, err := (&Server{}).opCancelRequest(completedControl.ctx, nil, json.RawMessage(`{"request-id":1,"method":"ze-plugin-engine:select-routes"}`))
	if err != nil {
		t.Fatal(err)
	}
	output, ok := result.(*rpc.CancelRequestOutput)
	if !ok {
		t.Fatalf("cancel-request returned %T, want *rpc.CancelRequestOutput", result)
	}
	if output.Outcome != rpc.CancelRequestNotActive {
		t.Fatal("finished request lost its explicit not-active answer")
	}
	for _, raw := range []string{`{"request-id":0}`, `{"request-id":3}`, `{"request-id":4}`,
		`{"request-id":1,"method":"unknown"}`, `{"request-id":1,"method":"ze-plugin-engine:update-route"}`} {
		if _, err := (&Server{}).opCancelRequest(completedControl.ctx, nil, json.RawMessage(raw)); err == nil {
			t.Fatalf("invalid cancellation accepted: %s", raw)
		}
	}
}

// TestRuntimeCancellationConnectionIsolation proves another connection cannot
// revoke a request with the same numeric ID. It also pins one admitted batch
// per authorized destination and the read-loop stop's cancellation signal.
func TestRuntimeCancellationConnectionIsolation(t *testing.T) {
	first := testRuntimeRequests(t.Context(), func(destination netip.Addr) bool { return destination == netip.MustParseAddr("192.0.2.7") })
	second := testRuntimeRequests(t.Context(), func(netip.Addr) bool { return true })
	defer func() { first.stop(); first.cancelAll() }()
	defer func() { second.stop(); second.cancelAll() }()
	input := selectedServerInput()
	input.DeadlineMS = time.Now().Add(time.Second).UnixMilli()
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := first.admit(&rpc.Request{ID: 1, Method: rpc.MethodSelectRoutes, Params: encoded})
	if err != nil {
		t.Fatal(err)
	}
	defer admitted.finish()
	rejected, err := first.admit(&rpc.Request{ID: 2, Method: rpc.MethodSelectRoutes, Params: encoded})
	if err == nil {
		t.Fatal("second batch at the same destination was admitted")
	}
	if rejected != nil {
		rejected.finish()
	}
	control, err := second.admit(&rpc.Request{ID: 2, Method: rpc.MethodCancelRequest})
	if err != nil {
		t.Fatal(err)
	}
	defer control.finish()
	if _, err := (&Server{}).opCancelRequest(control.ctx, nil, json.RawMessage(`{"request-id":1,"method":"ze-plugin-engine:select-routes"}`)); err != nil {
		t.Fatal(err)
	}
	if admitted.ctx.Err() != nil {
		t.Fatal("another connection canceled the same numeric request ID")
	}
	first.stop()
	if !errors.Is(admitted.ctx.Err(), context.Canceled) {
		t.Fatal("connection stop did not cancel admitted contexts before join")
	}
}

// testRuntimeRequests controls only the owner's resource validator. The live
// socket test above uses the real DeliveryGraph grant validator instead.
func testRuntimeRequests(ctx context.Context, allowed func(netip.Addr) bool) *runtimeRequests {
	return newRuntimeRequests(ctx, func(_ *engineOp, params json.RawMessage) (requestScope, error) {
		destination, deadline, err := selectedCancellationHeader(params)
		if err != nil {
			return requestScope{}, err
		}
		if !allowed(destination) {
			return requestScope{}, errors.New("test resource is not authorized")
		}
		return requestScopeFor(rpc.MethodSelectRoutes, destination, deadline), nil
	})
}

// TestRuntimeCancellationConcurrentIDsKeepAllocationOrderIndependent proves
// concurrent writers need not acquire their mutex in ID allocation order.
// Method: admit 10 then 9 at different resources, but refuse an active ID reuse.
func TestRuntimeCancellationConcurrentIDsKeepAllocationOrderIndependent(t *testing.T) {
	requests := testRuntimeRequests(t.Context(), func(netip.Addr) bool { return true })
	completions := make([]func(), 0, 2)
	defer func() {
		for _, finish := range slices.Backward(completions) {
			finish()
		}
		requests.stop()
		requests.cancelAll()
	}()
	for i, id := range []uint64{10, 9} {
		input := selectedServerInput()
		if i == 1 {
			input.Destination = "192.0.2.8"
		}
		input.DeadlineMS = time.Now().Add(rpc.CancellableRequestTimeout).UnixMilli()
		encoded, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		admission, err := requests.admit(&rpc.Request{ID: id, Method: rpc.MethodSelectRoutes, Params: encoded})
		if err != nil {
			t.Fatalf("valid concurrent request ID %d refused: %v", id, err)
		}
		completions = append(completions, admission.finish)
	}
	input := selectedServerInput()
	input.Destination = "192.0.2.9"
	input.DeadlineMS = time.Now().Add(rpc.CancellableRequestTimeout).UnixMilli()
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := requests.admit(&rpc.Request{ID: 10, Method: rpc.MethodSelectRoutes, Params: encoded}); err == nil {
		t.Fatal("active correlation ID was reused for another resource")
	}
}

// TestSelectedSocketDisconnectCancelsBeforeDispatchJoin proves EOF ends an
// admitted writer even while its process and daemon contexts stay alive.
// Method: close only the socket while the writer waits on its request context.
func TestSelectedSocketDisconnectCancelsBeforeDispatchJoin(t *testing.T) {
	work, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	reactor := &selectedCancellationReactor{mockReactor: &mockReactor{}, parent: work,
		started: make(chan struct{}), canceled: make(chan struct{})}
	host := &Server{ctx: work, reactor: reactor}
	proc := nativeOwnerProcess(t, host)
	host.UpdateDeliveryGraph(graphNS, []DeliveryPeer{{Addr: "192.0.2.7", Bindings: []plugin.PeerProcessBinding{{
		PluginName: proc.Name(), Send: map[string]bool{"update": true}}}}})
	client, engine := net.Pipe()
	peer := sdk.NewWithConn(proc.Name(), client)
	conn := plugipc.NewPluginConn(engine, engine)
	serverDone := make(chan struct{})
	go func() { host.serveRuntimeRequests(proc, conn); close(serverDone) }()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	finished := make(chan error, 1)
	go func() { _, err := peer.SelectRoutes(work, selectedServerInput()); finished <- err }()
	select {
	case <-reactor.started:
	case <-work.Done():
		t.Fatal(work.Err())
	}
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reactor.canceled:
	case <-work.Done():
		t.Fatal("reader exit waited before canceling its writer")
	}
	select {
	case <-serverDone:
	case <-work.Done():
		t.Fatal("canceled writer did not join its socket dispatcher")
	}
	select {
	case err := <-finished:
		if _, unsettled := errors.AsType[*rpc.CancellationUnsettledError](err); !unsettled {
			t.Fatalf("disconnect claimed a remote receipt: %v", err)
		}
	case <-work.Done():
		t.Fatal(work.Err())
	}
	if host.Context().Err() != nil {
		t.Fatal("socket EOF ended the daemon context")
	}
}

// TestSelectedRuntimeEnrollmentRequiresExistingGrant proves the cancellation
// resource inventory cannot be grown with invented destinations. Method: refuse the
// same shaped request before a real send grant exists, then admit it after.
func TestSelectedRuntimeEnrollmentRequiresExistingGrant(t *testing.T) {
	host := &Server{ctx: t.Context()}
	proc := nativeOwnerProcess(t, host)
	requests := newRuntimeRequests(t.Context(), func(operation *engineOp, params json.RawMessage) (requestScope, error) {
		return operation.cancellationScope(host, proc, params)
	})
	defer func() { requests.stop(); requests.cancelAll() }()
	input := selectedServerInput()
	input.DeadlineMS = time.Now().Add(rpc.CancellableRequestTimeout).UnixMilli()
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := requests.admit(&rpc.Request{ID: 1, Method: rpc.MethodSelectRoutes, Params: encoded})
	if err == nil {
		t.Fatal("an ungranted resource was admitted into the live inventory")
	}
	if rejected != nil {
		rejected.finish()
	}
	host.UpdateDeliveryGraph(graphNS, []DeliveryPeer{{Addr: input.Destination, Bindings: []plugin.PeerProcessBinding{{
		PluginName: proc.Name(), Send: map[string]bool{"update": true}}}}})
	admitted, err := requests.admit(&rpc.Request{ID: 2, Method: rpc.MethodSelectRoutes, Params: encoded})
	if err != nil {
		t.Fatal(err)
	}
	admitted.finish()
}
