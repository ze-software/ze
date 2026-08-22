package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// dispatchAnswerOverSocket runs one dispatch RPC from the plugin side of a
// socket and rebuilds the value the command answered with: the head's status,
// the document its records carry, and the command's own failure as an error.
//
// The engine writes the record sequence to every peer, so the reader here is
// the answer reader rather than the single-line CallRPC. It rebuilds the value
// exactly as a real plugin does (answerValue, pkg/plugin/sdk/sdk_engine.go),
// which is what keeps these tests asserting on what a plugin receives.
//
// The caller owns side and closes it; the mux reader ends when it does.
func dispatchAnswerOverSocket(ctx context.Context, side net.Conn, input any) (string, json.RawMessage, error) {
	answer, err := rpc.NewMuxConn(rpc.NewConn(side, side)).CallAnswer(ctx, rpc.MethodDispatchCommand, input)
	if err != nil {
		return "", nil, err
	}

	document, collapseErr := rpc.CollapseAnswer(answer)

	// Read after the range, never before: the range is what fills them.
	if answerErr := answer.Err(); answerErr != nil {
		return "", nil, answerErr
	}
	if collapseErr != nil {
		return "", nil, collapseErr
	}
	if message := answer.Message(); message != "" {
		return rpc.StatusError, nil, errors.New(message)
	}
	return rpc.StatusDone, document, nil
}

func registerExecuteCommandTarget(
	t *testing.T,
	d *Dispatcher,
	command string,
	calls int,
	handle func(int, *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error),
) <-chan error {
	t.Helper()

	engineSide, pluginSide := net.Pipe()
	t.Cleanup(func() { _ = engineSide.Close() })
	t.Cleanup(func() { _ = pluginSide.Close() })

	// Wired the way production wires a plugin (Process.attachConn): an answer
	// is many lines routed by id, and only MuxConn routes them.
	engineMux := rpc.NewMuxConn(rpc.NewConn(engineSide, engineSide))
	t.Cleanup(func() { _ = engineMux.Close() })

	proc := process.NewProcess(plugin.PluginConfig{Name: "target-plugin"})
	proc.SetConn(ipc.NewMuxPluginConn(engineMux))
	proc.SetRunning(true)

	results := d.Registry().Register(proc, []CommandDef{{Name: command, Description: "target command"}})
	require.Len(t, results, 1)
	require.True(t, results[0].OK, results[0].Error)

	pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginSide, pluginSide))
	t.Cleanup(func() { _ = pluginMux.Close() })

	done := make(chan error, 1)
	go func() {
		for i := range calls {
			readCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			req, err := readMuxRequest(readCtx, pluginMux)
			cancel()
			if err != nil {
				done <- err
				return
			}
			if req.Method != "ze-plugin-callback:execute-command" {
				done <- fmt.Errorf("unexpected method: %s", req.Method)
				return
			}

			var input rpc.ExecuteCommandInput
			if err := json.Unmarshal(req.Params, &input); err != nil {
				done <- err
				return
			}

			out, err := handle(i, &input)
			if err != nil {
				done <- err
				return
			}
			if out == nil {
				out = &rpc.ExecuteCommandOutput{Status: plugin.StatusDone}
			}

			// The target declares no wire shape and answers with the sequence
			// every peer answers with: the head, the one record its handler
			// built, and the terminator. A failure rides the terminator,
			// which is the one line an answer states its outcome on.
			writeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			head, payload := rpc.AnswerTail{}, out.Data
			if out.Status == plugin.StatusError {
				head.Message, payload = string(out.Data), nil
			}
			err = rpc.WriteDocumentAnswer(pluginMux.AnswerWriter(writeCtx), req.ID, head, payload)
			cancel()
			if err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	return done
}

// readMuxRequest takes the next request the engine sent over mux, or the
// context's error when none arrives.
func readMuxRequest(ctx context.Context, mux *rpc.MuxConn) (*rpc.Request, error) {
	select {
	case req, ok := <-mux.Requests():
		if !ok {
			return nil, rpc.ErrMuxConnClosed
		}
		return req, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-mux.Done():
		return nil, rpc.ErrMuxConnClosed
	}
}

type captureAuthorizer struct {
	allow      bool
	username   string
	remoteAddr string
	command    string
	readOnly   bool
}

func (a *captureAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	a.username = username
	a.remoteAddr = remoteAddr
	a.command = command
	a.readOnly = isReadOnly
	return a.allow
}

type captureCommandArgsAuthorizer struct {
	allow            bool
	username         string
	remoteAddr       string
	command          string
	args             []string
	peer             string
	readOnly         bool
	structuredCalled bool
	legacyCalled     bool
}

func (a *captureCommandArgsAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	a.legacyCalled = true
	a.username = username
	a.remoteAddr = remoteAddr
	a.command = command
	a.readOnly = isReadOnly
	return a.allow
}

func (a *captureCommandArgsAuthorizer) AuthorizeCommandArgs(username, remoteAddr, command string, args []string, peer string, isReadOnly bool) bool {
	a.structuredCalled = true
	a.username = username
	a.remoteAddr = remoteAddr
	a.command = command
	a.args = slices.Clone(args)
	a.peer = peer
	a.readOnly = isReadOnly
	return a.allow
}

// TestDispatchCommandToPlugin verifies that the dispatch-command RPC dispatches
// a command through the engine's dispatcher and returns the full {status, data} response.
//
// VALIDATES: AC-1 — Plugin A calls DispatchCommand with command registered by Plugin B,
//
//	Plugin B's handler invoked, response returned to A.
//
// VALIDATES: AC-2 — Plugin B returns status="done" with JSON data,
//
//	Plugin A receives both status and data string.
//
// PREVENTS: dispatch-command RPC failing to route through standard dispatcher.
func TestDispatchCommandToPlugin(t *testing.T) {
	t.Parallel()

	pluginSide, engineSide := net.Pipe()

	proc := process.NewProcess(plugin.PluginConfig{Name: "test-dispatch"})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))

	d := NewDispatcher()
	d.Register("test command", func(_ *CommandContext, args []string) (*plugin.Response, error) {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"last-index": float64(42)},
		}, nil
	}, "test command")

	s := &Server{
		subscriptions: newSubscriptionManager(),
		dispatcher:    d,
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	input := &rpc.DispatchCommandInput{Command: "test command"}
	status, data, err := dispatchAnswerOverSocket(ctx, pluginSide, input)
	require.NoError(t, err)

	assert.Equal(t, "done", status)
	assert.Contains(t, string(data), "last-index")

	if err := pluginSide.Close(); err != nil {
		t.Logf("close: %v", err)
	}
	if err := engineSide.Close(); err != nil {
		t.Logf("close: %v", err)
	}

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit")
	}
}

// VALIDATES: AC-6 -- plugin dispatch-command preserves plugin identity while inheriting server context.
// PREVENTS: plugin JSON or background-rooted dispatch from bypassing identity/accounting metadata.
func TestHandleDispatchCommandRPCPreservesPluginIdentity(t *testing.T) {
	t.Parallel()

	pluginSide, engineSide := net.Pipe()
	t.Cleanup(func() { _ = pluginSide.Close() })
	t.Cleanup(func() { _ = engineSide.Close() })

	proc := process.NewProcess(plugin.PluginConfig{Name: "identity-check"})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))

	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	var (
		gotUsername string
		gotContext  context.Context
	)

	d := NewDispatcher()
	d.Register("test command", func(ctx *CommandContext, _ []string) (*plugin.Response, error) {
		gotUsername = ctx.Username
		gotContext = ctx.Context()
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "test command")

	s := &Server{
		subscriptions: newSubscriptionManager(),
		dispatcher:    d,
		ctx:           serverCtx,
	}
	s.cancel = serverCancel

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()

	callCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	status, _, err := dispatchAnswerOverSocket(callCtx, pluginSide, &rpc.DispatchCommandInput{Command: "test command"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, status)
	// spec-fixit-authz-admin-fallthrough O-4: internal dispatch injects the
	// reserved trusted identity (un-typeable), keeping the plugin name after the
	// prefix for audit while authorizing as a trusted in-process caller.
	assert.Equal(t, internalPluginIdentity("identity-check"), gotUsername)
	assert.Same(t, serverCtx, gotContext)

	serverCancel()

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit")
	}
}

// TestDispatchCommandNotFound verifies that dispatching an unknown command
// returns an error through the dispatch-command RPC.
//
// VALIDATES: AC-3 — Command not found in registry returns error.
// PREVENTS: Unknown commands returning success or panicking.
func TestDispatchCommandNotFound(t *testing.T) {
	t.Parallel()

	pluginSide, engineSide := net.Pipe()

	proc := process.NewProcess(plugin.PluginConfig{Name: "test-dispatch-notfound"})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))

	s := &Server{
		subscriptions: newSubscriptionManager(),
		dispatcher:    NewDispatcher(),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()

	pluginConn := rpc.NewConn(pluginSide, pluginSide)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	input := &rpc.DispatchCommandInput{Command: "nonexistent command"}
	_, err := pluginConn.CallRPC(ctx, "ze-plugin-engine:dispatch-command", input)
	require.Error(t, err, "should return error for unknown command")

	if err := pluginSide.Close(); err != nil {
		t.Logf("close: %v", err)
	}
	if err := engineSide.Close(); err != nil {
		t.Logf("close: %v", err)
	}

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit")
	}
}

// TestDispatchCommandPluginError verifies that an error response from the command
// handler is propagated back through the dispatch-command RPC.
//
// VALIDATES: AC-4 — Plugin B returns error, DispatchCommand returns error with message.
// PREVENTS: Handler errors being silently swallowed.
func TestDispatchCommandPluginError(t *testing.T) {
	t.Parallel()

	pluginSide, engineSide := net.Pipe()

	proc := process.NewProcess(plugin.PluginConfig{Name: "test-dispatch-error"})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))

	d := NewDispatcher()
	d.Register("failing command", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "something went wrong",
		}, nil
	}, "failing command")

	s := &Server{
		subscriptions: newSubscriptionManager(),
		dispatcher:    d,
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	input := &rpc.DispatchCommandInput{Command: "failing command"}
	status, _, err := dispatchAnswerOverSocket(ctx, pluginSide, input)
	require.Error(t, err, "the command's own failure reaches the caller as an error")
	assert.Contains(t, err.Error(), "something went wrong")
	assert.Equal(t, "error", status)

	if err := pluginSide.Close(); err != nil {
		t.Logf("close: %v", err)
	}
	if err := engineSide.Close(); err != nil {
		t.Logf("close: %v", err)
	}

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit")
	}
}

// TestDispatchCommandEmptyCommand verifies boundary: empty command string returns error.
//
// VALIDATES: Boundary test — empty command input returns error.
// PREVENTS: Empty commands causing panics or silent no-ops.
func TestDispatchCommandEmptyCommand(t *testing.T) {
	t.Parallel()

	pluginSide, engineSide := net.Pipe()

	proc := process.NewProcess(plugin.PluginConfig{Name: "test-dispatch-empty"})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))

	s := &Server{
		subscriptions: newSubscriptionManager(),
		dispatcher:    NewDispatcher(),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()

	pluginConn := rpc.NewConn(pluginSide, pluginSide)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	input := &rpc.DispatchCommandInput{Command: ""}
	_, err := pluginConn.CallRPC(ctx, "ze-plugin-engine:dispatch-command", input)
	require.Error(t, err, "empty command should return error")

	if err := pluginSide.Close(); err != nil {
		t.Logf("close: %v", err)
	}
	if err := engineSide.Close(); err != nil {
		t.Logf("close: %v", err)
	}

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit")
	}
}

// TestDispatchCommandDirectBridge verifies dispatch-command works through the
// DirectBridge path (internal plugins).
//
// VALIDATES: AC-5 — DirectBridge path has same behavior as socket path.
// PREVENTS: Internal plugins unable to use dispatch-command.
func TestDispatchCommandDirectBridge(t *testing.T) {
	t.Parallel()

	d := NewDispatcher()
	handlerReturned := false
	completed := false
	d.Register("bridge test", func(_ *CommandContext, _ []string) (resp *plugin.Response, err error) {
		defer func() { handlerReturned = true }()
		resp = &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"result": "bridge-ok"},
		}
		resp.OnTransportComplete(func() {
			assert.True(t, handlerReturned, "direct completion ran before the handler delivered its result")
			completed = true
		})
		return resp, nil
	}, "bridge test")

	s := &Server{
		subscriptions: newSubscriptionManager(),
		dispatcher:    d,
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	proc := process.NewProcess(plugin.PluginConfig{Name: "test-direct-bridge"})

	input := &rpc.DispatchCommandInput{Command: "bridge test"}
	params, err := json.Marshal(input)
	require.NoError(t, err)

	raw, err := s.dispatchPluginRPCDirect(proc, "ze-plugin-engine:dispatch-command", params)
	require.NoError(t, err)

	// DirectBridge returns marshaled result directly (no envelope).
	var output rpc.DispatchCommandOutput
	require.NoError(t, json.Unmarshal(raw, &output))

	assert.Equal(t, "done", output.Status)
	assert.Contains(t, string(output.Data), "bridge-ok")
	assert.True(t, completed, "direct transport did not complete the accepted action")
}

// TestDispatchCommandArgsRoutesSameHandlerAsDispatchCommand verifies that the
// typed exact-command path reaches the same plugin command handler as the
// external string dispatch path for ordinary token values.
//
// VALIDATES: dispatch-command-args routes exact command plus args to the registered plugin handler.
// PREVENTS: typed dispatch using a parallel handler path that diverges from dispatch-command routing.
func TestDispatchCommandArgsRoutesSameHandlerAsDispatchCommand(t *testing.T) {
	t.Parallel()

	d := NewDispatcher()
	done := registerExecuteCommandTarget(t, d, "request target echo", 2, func(_ int, input *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error) {
		if input.Command != "request target echo" {
			return nil, fmt.Errorf("unexpected command: %s", input.Command)
		}
		if !slices.Equal(input.Args, []string{"alpha", "beta"}) {
			return nil, fmt.Errorf("unexpected args: %#v", input.Args)
		}
		if input.Peer != "*" {
			return nil, fmt.Errorf("unexpected peer: %s", input.Peer)
		}
		return &rpc.ExecuteCommandOutput{
			Status: plugin.StatusDone,
			Data:   json.RawMessage(`{"handler":"target"}`),
		}, nil
	})

	s := &Server{dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	caller := process.NewProcess(plugin.PluginConfig{Name: "caller-plugin"})
	stringOut, err := s.dispatchCommand(caller, "request target echo alpha beta")
	require.NoError(t, err)

	argsOut, err := s.dispatchCommandArgs(caller, "request target echo", []string{"alpha", "beta"}, "")
	require.NoError(t, err)

	assert.Equal(t, stringOut.Status, argsOut.Status)
	assert.JSONEq(t, string(stringOut.Data), string(argsOut.Data))
	require.NoError(t, <-done)
}

// TestDispatchCommandArgsPreservesOddArguments verifies args that the command
// tokenizer would split or reject arrive at the plugin as single args.
//
// VALIDATES: dispatch-command-args delivers spaces, quotes, tabs, and backslashes without tokenization.
// PREVENTS: internal runtime data being reinterpreted as command syntax.
func TestDispatchCommandArgsPreservesOddArguments(t *testing.T) {
	t.Parallel()

	d := NewDispatcher()
	wantArgs := []string{"peer key with spaces", `quote"inside`, "tab\tinside", `slash\inside`}
	wantPeer := "peer selector with spaces"
	done := registerExecuteCommandTarget(t, d, "request target odd", 1, func(_ int, input *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error) {
		if input.Command != "request target odd" {
			return nil, fmt.Errorf("unexpected command: %s", input.Command)
		}
		if !slices.Equal(input.Args, wantArgs) {
			return nil, fmt.Errorf("unexpected args: %#v", input.Args)
		}
		if input.Peer != wantPeer {
			return nil, fmt.Errorf("unexpected peer: %s", input.Peer)
		}
		return &rpc.ExecuteCommandOutput{Status: plugin.StatusDone}, nil
	})

	s := &Server{dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	caller := process.NewProcess(plugin.PluginConfig{Name: "caller-plugin"})
	out, err := s.dispatchCommandArgs(caller, "request target odd", wantArgs, wantPeer)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, plugin.StatusDone, out.Status)
	require.NoError(t, <-done)
}

// TestDispatchCommandArgsErrorsMatchDispatchCommand verifies unknown command
// and handler error semantics stay compatible with dispatch-command.
//
// VALIDATES: dispatch-command-args preserves unknown-command errors and handler status/error output.
// PREVENTS: typed dispatch returning different status or error shapes from the string API.
func TestDispatchCommandArgsErrorsMatchDispatchCommand(t *testing.T) {
	t.Parallel()

	d := NewDispatcher()
	done := registerExecuteCommandTarget(t, d, "request target fails", 2, func(_ int, input *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error) {
		if input.Command != "request target fails" {
			return nil, fmt.Errorf("unexpected command: %s", input.Command)
		}
		return &rpc.ExecuteCommandOutput{
			Status: plugin.StatusError,
			Data:   json.RawMessage(`"target broke"`),
		}, nil
	})

	s := &Server{dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	caller := process.NewProcess(plugin.PluginConfig{Name: "caller-plugin"})

	stringUnknownOut, stringUnknownErr := s.dispatchCommand(caller, "missing command")
	argsUnknownOut, argsUnknownErr := s.dispatchCommandArgs(caller, "missing command", nil, "")
	require.Error(t, stringUnknownErr)
	require.Error(t, argsUnknownErr)
	assert.True(t, errors.Is(stringUnknownErr, ErrUnknownCommand))
	assert.True(t, errors.Is(argsUnknownErr, ErrUnknownCommand))
	assert.Nil(t, stringUnknownOut)
	assert.Nil(t, argsUnknownOut)

	stringOut, err := s.dispatchCommand(caller, "request target fails")
	require.NoError(t, err)
	argsOut, err := s.dispatchCommandArgs(caller, "request target fails", nil, "")
	require.NoError(t, err)

	assert.Equal(t, stringOut.Status, argsOut.Status)
	assert.Equal(t, stringOut.Error, argsOut.Error)
	assert.Equal(t, plugin.StatusError, argsOut.Status)
	assert.Contains(t, argsOut.Error, "target broke")
	require.NoError(t, <-done)
}

// TestDispatchCommandArgsPreservesPluginIdentity verifies typed dispatch uses
// the caller plugin identity and structured auth path for authorization.
//
// VALIDATES: dispatch-command-args authorizes as the reserved internal identity
// carrying the caller plugin name, with exact command, args, and peer.
// PREVENTS: typed dispatch flattening auth inputs before built-in authorizers can inspect them.
func TestDispatchCommandArgsPreservesPluginIdentity(t *testing.T) {
	t.Parallel()

	d := NewDispatcher()
	auth := &captureCommandArgsAuthorizer{allow: false}
	d.SetAuthorizer(auth)

	s := &Server{dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	caller := process.NewProcess(plugin.PluginConfig{Name: "caller-plugin"})
	wantArgs := []string{"peer key with spaces", `quote"inside`, `slash\inside`}
	out, err := s.dispatchCommandArgs(caller, "request target echo", wantArgs, "10.0.0.1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnauthorized))
	assert.NotNil(t, out)
	assert.Equal(t, plugin.StatusError, out.Status)
	assert.Equal(t, internalPluginIdentity("caller-plugin"), auth.username)
	assert.Equal(t, "request target echo", auth.command)
	assert.Equal(t, wantArgs, auth.args)
	assert.Equal(t, "10.0.0.1", auth.peer)
	assert.False(t, auth.readOnly)
	assert.True(t, auth.structuredCalled)
	assert.False(t, auth.legacyCalled)
}

// TestDispatchCommandArgsLegacyAuthorizationCanonicalizesPeerScope verifies the
// fallback string path preserves peer scope and quoted arg boundaries for
// legacy aaa.Authorizer implementations.
//
// VALIDATES: legacy authorizers receive aaa.CanonicalCommand(command, args, peer).
// PREVENTS: peer-scoped typed dispatch being authorized as an unscoped flat string.
func TestDispatchCommandArgsLegacyAuthorizationCanonicalizesPeerScope(t *testing.T) {
	t.Parallel()

	d := NewDispatcher()
	auth := &captureAuthorizer{allow: false}
	d.SetAuthorizer(auth)

	s := &Server{dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	caller := process.NewProcess(plugin.PluginConfig{Name: "caller-plugin"})
	wantArgs := []string{"peer key with spaces", `quote"inside`, `slash\inside`}
	out, err := s.dispatchCommandArgs(caller, "request target echo", wantArgs, "10.0.0.1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnauthorized))
	assert.NotNil(t, out)
	assert.Equal(t, plugin.StatusError, out.Status)
	assert.Equal(t, internalPluginIdentity("caller-plugin"), auth.username)
	assert.Equal(t, aaa.CanonicalCommand("request target echo", wantArgs, "10.0.0.1"), auth.command)
	assert.False(t, auth.readOnly)
}

func TestResponseToDispatchOutputMarshalFail(t *testing.T) {
	t.Parallel()

	resp := &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"bad": make(chan int)}, // channels are not JSON-serializable
	}

	output := responseToDispatchOutput(resp)

	assert.Equal(t, plugin.StatusError, output.Status)
	assert.Contains(t, output.Error, "marshal response data")
	assert.Empty(t, output.Data)
}

func TestResponseToDispatchOutputErrorField(t *testing.T) {
	t.Parallel()

	resp := &plugin.Response{
		Status: plugin.StatusError,
		Error:  "something went wrong",
	}

	output := responseToDispatchOutput(resp)

	assert.Equal(t, plugin.StatusError, output.Status)
	assert.Equal(t, "something went wrong", output.Error)
	assert.Empty(t, output.Data)
}

func TestDispatchCommandOutputRoundTrip(t *testing.T) {
	t.Parallel()

	original := rpc.DispatchCommandOutput{
		Status: "done",
		Data:   json.RawMessage(`{"peers":[{"address":"192.0.2.1"}]}`),
	}

	wire, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded rpc.DispatchCommandOutput
	require.NoError(t, json.Unmarshal(wire, &decoded))

	assert.Equal(t, original.Status, decoded.Status)
	assert.Equal(t, string(original.Data), string(decoded.Data))
	assert.Empty(t, decoded.Error)

	var peers map[string]any
	require.NoError(t, json.Unmarshal(decoded.Data, &peers))
	assert.NotNil(t, peers["peers"])
}

func TestDispatchCommandOutputRoundTripError(t *testing.T) {
	t.Parallel()

	original := rpc.DispatchCommandOutput{
		Status: "error",
		Error:  "command not found",
	}

	wire, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded rpc.DispatchCommandOutput
	require.NoError(t, json.Unmarshal(wire, &decoded))

	assert.Equal(t, "error", decoded.Status)
	assert.Equal(t, "command not found", decoded.Error)
	assert.Empty(t, decoded.Data)
}

func TestResponseToDispatchOutputSingleDecode(t *testing.T) {
	t.Parallel()

	resp := &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"count": float64(7)},
	}

	output := responseToDispatchOutput(resp)

	assert.Equal(t, plugin.StatusDone, output.Status)
	assert.Empty(t, output.Error)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(output.Data, &decoded))
	assert.Equal(t, float64(7), decoded["count"])
}
