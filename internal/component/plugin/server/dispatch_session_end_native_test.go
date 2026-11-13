// Design: docs/architecture/api/process-protocol.md -- a request the end of the engine session canceled is answered by the fenced bridge in process.
// Related: dispatch_native.go -- directCallError; dispatch_session_end_test.go -- the socket path's cases.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const sessionEndNativeMethod = "ze-plugin:session-end-native-test"

// sessionEndDirectCall is one in-process plugin-to-engine call through the
// real bridge and the server's production adapters of a real internal
// process generation.
type sessionEndDirectCall struct {
	proc    *process.Process
	entered chan struct{}
	answer  chan error
}

// startSessionEndDirectCall registers one runtime operation whose raw and
// native handlers both fail with fail, then calls it from the plugin side:
// through DispatchNative when native, else through the JSON-shaped
// DispatchRPC. The caller's context stays live, as the worker's owner
// context does at a stop.
func startSessionEndDirectCall(t *testing.T, s *Server, native bool, fail func(*Server, *process.Process) error) *sessionEndDirectCall {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	call := &sessionEndDirectCall{entered: make(chan struct{}), answer: make(chan error, 1)}
	var once sync.Once
	evaluate := func(srv *Server, proc *process.Process) error {
		once.Do(func() { close(call.entered) })
		return fail(srv, proc)
	}
	registerTestEngineOperation(t, EngineOperation{
		Method: sessionEndNativeMethod, Startup: EngineOperationRuntimeOnly,
		Handler: func(srv *Server, _ context.Context, proc *process.Process, _ json.RawMessage) (any, error) {
			return nil, evaluate(srv, proc)
		},
		Native: func(srv *Server, _ context.Context, proc *process.Process, _ any) (any, error) {
			return nil, evaluate(srv, proc)
		},
	})
	call.proc = nativeOwnerProcess(t, s)
	bridge := call.proc.Bridge()
	bridge.SetReady()
	go func() {
		if native {
			_, handled, err := bridge.DispatchNative(ctx, sessionEndNativeMethod, nil)
			if err == nil {
				if !handled {
					err = errors.New("registered native operation was not handled")
				}
			}
			call.answer <- err
			return
		}
		_, err := bridge.DispatchRPC(ctx, sessionEndNativeMethod, nil)
		call.answer <- err
	}()
	return call
}

func (call *sessionEndDirectCall) awaitEntered(t *testing.T) {
	t.Helper()
	select {
	case <-call.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the in-process call never reached its handler")
	}
}

func (call *sessionEndDirectCall) result(t *testing.T) error {
	t.Helper()
	select {
	case err := <-call.answer:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("the in-process call never returned")
		return nil
	}
}

func requireBridgeClosedAnswer(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, rpc.ErrBridgeClosed) {
		return
	}
	t.Fatalf("an in-process request the stop canceled came back as %T %v, which a live owner reads as a refusal; want rpc.ErrBridgeClosed, the fenced bridge's own answer", err, err)
}

func waitServerStop(srv *Server, _ *process.Process) error {
	<-srv.Context().Done()
	return sessionEndStopped(srv.Context().Err())
}

func waitProcessStop(_ *Server, proc *process.Process) error {
	<-proc.Done()
	return sessionEndStopped(context.Canceled)
}

// Goal: at daemon stop an in-process native request the stop canceled comes
// back as the bridge's closed answer, never as the stop refusal. This is the
// internal system RIB's native publication at stop, which the worker retained
// as a failure. Method: the native handler waits on the server's lifetime
// context and returns its cancellation wrapped as routeViewStopped wraps it;
// the server context is canceled while the caller's context stays live and
// before any Process.Stop fences the bridge.
func TestNativeRequestCancelledByServerStopAnswersBridgeClosed(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	call := startSessionEndDirectCall(t, s, true, waitServerStop)
	call.awaitEntered(t)
	s.cancel()
	requireBridgeClosedAnswer(t, call.result(t))
}

// Goal: a stop of only this internal process generation is the end of its
// engine session too. Method: the native handler waits on the generation's
// Done channel, as the routing-view handlers wait on processDone, and
// Process.Stop fences dispatch and cancels that context with the server live.
func TestNativeRequestCancelledByProcessStopAnswersBridgeClosed(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	call := startSessionEndDirectCall(t, s, true, waitProcessStop)
	call.awaitEntered(t)
	call.proc.Stop()
	if err := s.ctx.Err(); err != nil {
		t.Fatalf("fixture stopped the server, not only the process: %v", err)
	}
	requireBridgeClosedAnswer(t, call.result(t))
}

// Goal: the JSON-shaped in-process path states the same rule as the native
// one. Method: the raw handler waits on the server context, the plugin calls
// it through DispatchRPC, and the server context is canceled.
func TestDirectRequestCancelledByServerStopAnswersBridgeClosed(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	call := startSessionEndDirectCall(t, s, false, waitServerStop)
	call.awaitEntered(t)
	s.cancel()
	requireBridgeClosedAnswer(t, call.result(t))
}

// Goal: a cancellation a native handler met while the server and the process
// generation both live still comes back as the handler's own failure, never
// as the closed bridge. Method: the handler returns the stop-shaped operator
// error at once; nothing stops.
func TestNativeRequestCancelledUnderLiveSessionKeepsItsFailure(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	call := startSessionEndDirectCall(t, s, true, func(*Server, *process.Process) error {
		return sessionEndStopped(context.Canceled)
	})
	err := call.result(t)
	if errors.Is(err, rpc.ErrBridgeClosed) {
		t.Fatalf("a cancellation under a live session was answered as a stopped session: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancellation under a live session lost its failure: %T %v", err, err)
	}
	if _, stopped := errors.AsType[*opererr.Error](err); !stopped {
		t.Fatalf("a cancellation under a live session lost its operator refusal: %T %v", err, err)
	}
}

// Goal: during a stop only the stop's own cancellation becomes the closed
// bridge; a native refusal for another reason is still an RPC refusal.
// Method: the server context is canceled before the call, and the handler
// returns an operator refusal that wraps no cancellation.
func TestNativeRefusalDuringServerStopIsRefusal(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	refusal := opererr.New("Test request refused", "The request named no open session", "ze request plugin restart "+sessionEndPlugin)
	s.cancel()
	call := startSessionEndDirectCall(t, s, true, func(*Server, *process.Process) error {
		return refusal
	})
	err := call.result(t)
	var answered *rpc.RPCCallError
	if !errors.As(err, &answered) {
		t.Fatalf("a refusal during the stop was not answered as a refusal: %T %v", err, err)
	}
	if answered.Message != refusal.Error() {
		t.Fatalf("answered refusal=%q, want %q", answered.Message, refusal.Error())
	}
}
