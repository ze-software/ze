// Design: docs/architecture/api/process-protocol.md -- a request the end of the engine session cancelled is answered by the close.
// Related: dispatch_registry.go -- serveEngineOpJSON, engineSessionEnded.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	sessionEndMethod = "ze-plugin:session-end-test"
	sessionEndPlugin = "session-end"
)

// sessionEndCall is one plugin-to-engine runtime call on its own pipe: the
// plugin end is a MuxConn, as a forked plugin's SDK holds, and the engine end
// is served once by serveEngineOpJSON.
type sessionEndCall struct {
	ctx    context.Context
	engine net.Conn
	answer chan error
	served chan struct{}
}

// startSessionEndCall sends one request and serves it with handle. The test
// decides when the stop happens; it MUST call finish after it.
func startSessionEndCall(t *testing.T, s *Server, proc *process.Process, handle func(*Server, context.Context, *process.Process, json.RawMessage) (any, error)) *sessionEndCall {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	pluginEnd, engineEnd := net.Pipe()
	remote := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
	t.Cleanup(func() {
		_ = remote.Close()    //nolint:errcheck // Owned fixture cleanup; closing twice is safe.
		_ = engineEnd.Close() //nolint:errcheck // Owned fixture cleanup; closing twice is safe.
	})
	call := &sessionEndCall{ctx: ctx, engine: engineEnd, answer: make(chan error, 1), served: make(chan struct{})}
	go func() {
		_, err := remote.CallRPC(ctx, sessionEndMethod, nil)
		call.answer <- err
	}()
	conn := plugipc.NewPluginConn(engineEnd, engineEnd)
	req, err := conn.ReadRequest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	op := &engineOp{method: sessionEndMethod, handle: handle}
	go func() {
		defer close(call.served)
		s.serveEngineOpJSON(proc, conn, req, op)
	}()
	return call
}

// finish waits until the engine served the request, closes the engine end the
// way a stop does, and returns what the plugin's call received. A response
// written before the close is consumed before the reader's terminal error.
func (call *sessionEndCall) finish(t *testing.T) error {
	t.Helper()
	select {
	case <-call.served:
	case <-call.ctx.Done():
		t.Fatal("engine never finished serving the request")
	}
	if err := call.engine.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-call.answer:
		return err
	case <-call.ctx.Done():
		t.Fatal("the plugin's call never ended after the engine closed")
		return nil
	}
}

func requireAnsweredByClose(t *testing.T, err error) {
	t.Helper()
	var refusal *rpc.RPCCallError
	if errors.As(err, &refusal) {
		t.Fatalf("the engine answered a request its own stop cancelled with a refusal, which the plugin cannot tell from a live refusal: %q", refusal.Message)
	}
	if errors.Is(err, io.EOF) {
		return
	}
	if errors.Is(err, rpc.ErrMuxConnClosed) {
		return
	}
	if errors.Is(err, io.ErrClosedPipe) {
		return
	}
	if errors.Is(err, net.ErrClosed) {
		return
	}
	t.Fatalf("the plugin's pending call ended with %T %v, want the connection close", err, err)
}

func requireAnsweredRefusal(t *testing.T, err error, problem string) {
	t.Helper()
	var refusal *rpc.RPCCallError
	if !errors.As(err, &refusal) {
		t.Fatalf("the engine withheld the answer to a failure that is not the end of the session: %T %v", err, err)
	}
	if refusal.Message != problem {
		t.Fatalf("answered refusal=%q, want %q", refusal.Message, problem)
	}
}

func sessionEndStopped(cause error) error {
	return opererr.Wrap(cause, "Test request stopped before completion", "The request did not return a complete result", "ze request plugin restart "+sessionEndPlugin)
}

// Goal: at daemon stop the engine does not answer a runtime request that the
// stop itself cancelled; the connection close answers it. This is the forked
// system RIB's long-poll read at stop, which the daemon answered with a
// refusal before the close, so the consumer logged a plugin failure. Method:
// the handler waits on the server context, as the routing-view read waits,
// and returns that cancellation wrapped in an operator error, as
// routeViewStopped does. The server context is cancelled with the connection
// still open (Server.Stop cancels before ProcessManager.Stop closes), and the
// engine end is closed only after the handler was served.
func TestRuntimeRequestCancelledByServerStopIsLeftToConnectionClose(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	proc := process.NewProcess(plugin.PluginConfig{Name: sessionEndPlugin})
	entered := make(chan struct{})
	call := startSessionEndCall(t, s, proc, func(_ *Server, ctx context.Context, _ *process.Process, _ json.RawMessage) (any, error) {
		close(entered)
		<-ctx.Done()
		return nil, sessionEndStopped(ctx.Err())
	})
	<-entered
	s.cancel()
	requireAnsweredByClose(t, call.finish(t))
}

// Goal: a stop of only this plugin's process generation (a plugin restart, a
// removal after bye) is the end of its engine session too, with the server
// live. Method: a real internal process generation; the handler waits on its
// Done channel, as the routing-view read waits on processDone, and
// Process.Stop cancels that context before it closes the connections.
func TestRuntimeRequestCancelledByProcessStopIsLeftToConnectionClose(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	proc := nativeOwnerProcess(t, s)
	entered := make(chan struct{})
	call := startSessionEndCall(t, s, proc, func(_ *Server, _ context.Context, owner *process.Process, _ json.RawMessage) (any, error) {
		close(entered)
		<-owner.Done()
		return nil, sessionEndStopped(context.Canceled)
	})
	<-entered
	proc.Stop()
	if err := s.ctx.Err(); err != nil {
		t.Fatalf("fixture stopped the server, not only the process: %v", err)
	}
	requireAnsweredByClose(t, call.finish(t))
}

// Goal: a cancellation the handler met while the server and the process
// generation both live is a refusal, and it is answered. Method: the handler
// returns an operator error wrapping context.Canceled at once; nothing stops.
func TestRuntimeRequestCancelledUnderLiveSessionIsAnswered(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	proc := process.NewProcess(plugin.PluginConfig{Name: sessionEndPlugin})
	refusal := sessionEndStopped(context.Canceled)
	call := startSessionEndCall(t, s, proc, func(*Server, context.Context, *process.Process, json.RawMessage) (any, error) {
		return nil, refusal
	})
	requireAnsweredRefusal(t, call.finish(t), refusal.Error())
}

// Goal: during a stop only the stop's own cancellation goes unanswered; a
// handler's refusal for another reason is still answered. Method: the server
// context is cancelled before the request is served, and the handler returns
// an operator refusal that wraps no cancellation.
func TestRuntimeRefusalDuringServerStopIsAnswered(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	proc := process.NewProcess(plugin.PluginConfig{Name: sessionEndPlugin})
	refusal := opererr.New("Test request refused", "The request named no open session", "ze request plugin restart "+sessionEndPlugin)
	s.cancel()
	call := startSessionEndCall(t, s, proc, func(*Server, context.Context, *process.Process, json.RawMessage) (any, error) {
		return nil, refusal
	})
	requireAnsweredRefusal(t, call.finish(t), refusal.Error())
}
