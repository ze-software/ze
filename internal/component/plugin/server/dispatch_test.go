package server

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

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
		subscriptions: NewSubscriptionManager(),
		dispatcher:    d,
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

	input := &rpc.DispatchCommandInput{Command: "test command"}
	result, err := pluginConn.CallRPC(ctx, "ze-plugin-engine:dispatch-command", input)
	require.NoError(t, err)

	var output rpc.DispatchCommandOutput
	require.NoError(t, json.Unmarshal(result, &output))

	assert.Equal(t, "done", output.Status)
	assert.Contains(t, string(output.Data), "last-index")

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
		subscriptions: NewSubscriptionManager(),
		dispatcher:    d,
		ctx:           serverCtx,
	}
	s.cancel = serverCancel

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()

	pluginConn := rpc.NewConn(pluginSide, pluginSide)

	callCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := pluginConn.CallRPC(callCtx, "ze-plugin-engine:dispatch-command", &rpc.DispatchCommandInput{Command: "test command"})
	require.NoError(t, err)

	var output rpc.DispatchCommandOutput
	require.NoError(t, json.Unmarshal(result, &output))
	assert.Equal(t, plugin.StatusDone, output.Status)
	assert.Equal(t, "plugin:identity-check", gotUsername)
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
		subscriptions: NewSubscriptionManager(),
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
		subscriptions: NewSubscriptionManager(),
		dispatcher:    d,
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

	input := &rpc.DispatchCommandInput{Command: "failing command"}
	result, err := pluginConn.CallRPC(ctx, "ze-plugin-engine:dispatch-command", input)
	require.NoError(t, err)

	var output rpc.DispatchCommandOutput
	require.NoError(t, json.Unmarshal(result, &output))
	assert.Equal(t, "error", output.Status)
	assert.Contains(t, output.Error, "something went wrong")

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
		subscriptions: NewSubscriptionManager(),
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
	d.Register("bridge test", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"result": "bridge-ok"},
		}, nil
	}, "bridge test")

	s := &Server{
		subscriptions: NewSubscriptionManager(),
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
