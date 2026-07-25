package server

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestHandleSystemDispatch verifies ze-system:dispatch routes text commands
// through the standard dispatcher, enabling API socket clients to reach
// plugin-registered commands.
//
// VALIDATES: Text commands from API socket clients reach the dispatcher.
// PREVENTS: API socket clients unable to invoke plugin commands.
func TestHandleSystemDispatch(t *testing.T) {
	d := NewDispatcher()

	var receivedArgs []string
	handler := func(_ *CommandContext, args []string) (*plugin.Response, error) {
		receivedArgs = args
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"result": "ok"}}, nil
	}
	d.Register("watchdog announce", handler, "Announce watchdog")

	srv := &Server{dispatcher: d}
	ctx := &CommandContext{Server: srv, Peer: "*"}

	resp, err := handleSystemDispatch(ctx, []string{"watchdog announce dnsr"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, []string{"dnsr"}, receivedArgs)
}

// TestHandleDaemonReloadUsesFullReload verifies daemon-reload prefers the hub-level reload hook.
//
// VALIDATES: CLI reload RPC reaches the full daemon reload path when it is wired.
// PREVENTS: CLI commit refreshing plugins but skipping provider, engine, and subsystem reloads.
func TestHandleDaemonReloadUsesFullReload(t *testing.T) {
	reactor := &mockReactor{}
	server, err := NewServer(&ServerConfig{}, reactor)
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	called := false
	server.SetFullReloadFunc(func(ctx context.Context) error {
		called = true
		assert.NotNil(t, ctx)
		return nil
	})
	server.SetConfigLoader(func() (map[string]any, error) {
		return nil, fmt.Errorf("plugin-only reload should not be used")
	})

	resp, err := handleDaemonReload(&CommandContext{Server: server}, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, called, "full reload hook should be called")
}

// TestHandleSystemDispatchMissingCommand verifies error on empty args.
//
// VALIDATES: Missing command returns error.
// PREVENTS: Panic on nil/empty args.
func TestHandleSystemDispatchMissingCommand(t *testing.T) {
	ctx := &CommandContext{}

	resp, err := handleSystemDispatch(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandleDaemonShutdownReactorlessUsesShutdownFunc verifies that a daemon
// without a BGP reactor stops via the daemon-provided shutdownFunc.
//
// VALIDATES: `request shutdown` on an OSPF-only (reactorless) daemon invokes the
// fallback shutdownFunc and reports done, instead of failing RequireReactor.
// PREVENTS: the regression where a reactorless daemon could not be stopped by
// command and hung until the test timeout (ospf ldp-sync tests).
func TestHandleDaemonShutdownReactorlessUsesShutdownFunc(t *testing.T) {
	called := false
	// A reactorless daemon carries the Coordinator as its reactor; its
	// FullReactor() returns the coordinator itself (a no-op Stop), so ctx.Reactor()
	// is non-nil. The handler must recognize this fallback and use shutdownFunc,
	// not silently no-op on coordinator.Stop() (the ospf ldp-sync hang).
	srv := &Server{reactor: plugin.NewCoordinator(nil)}
	srv.SetShutdownFunc(func() { called = true })

	resp, err := handleDaemonShutdown(&CommandContext{Server: srv}, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, called, "reactorless shutdown must invoke the daemon shutdownFunc, not coordinator.Stop()")
}

// TestHandleDaemonShutdownRealReactorStops verifies a BGP daemon (real reactor,
// not the coordinator fallback) still stops via the reactor and does NOT use the
// shutdownFunc, preserving graceful BGP shutdown.
func TestHandleDaemonShutdownRealReactorStops(t *testing.T) {
	reactor := &mockReactor{}
	funcCalled := false
	srv := &Server{reactor: reactor}
	srv.SetShutdownFunc(func() { funcCalled = true })

	resp, err := handleDaemonShutdown(&CommandContext{Server: srv}, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, reactor.stopped, "a real reactor must be stopped directly")
	assert.False(t, funcCalled, "a real reactor must not fall back to shutdownFunc")
}

// TestHandleDaemonShutdownNoReactorNoFuncErrors verifies a clean error when
// neither a reactor nor a shutdownFunc is available (fail closed, no panic).
func TestHandleDaemonShutdownNoReactorNoFuncErrors(t *testing.T) {
	resp, err := handleDaemonShutdown(&CommandContext{Server: &Server{}}, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandleSystemDispatchNoDispatcher verifies error when dispatcher unavailable.
//
// VALIDATES: Nil dispatcher returns error.
// PREVENTS: Panic when server has no dispatcher.
func TestHandleSystemDispatchNoDispatcher(t *testing.T) {
	ctx := &CommandContext{Server: &Server{}}

	resp, err := handleSystemDispatch(ctx, []string{"test"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandleSystemDispatchJoinsArgs verifies multiple args are joined.
//
// VALIDATES: Multiple args elements joined into single command string.
// PREVENTS: Only first arg used when multiple provided.
func TestHandleSystemDispatchJoinsArgs(t *testing.T) {
	d := NewDispatcher()

	var receivedArgs []string
	handler := func(_ *CommandContext, args []string) (*plugin.Response, error) {
		receivedArgs = args
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	d.Register("watchdog withdraw", handler, "Withdraw watchdog")

	srv := &Server{dispatcher: d}
	ctx := &CommandContext{Server: srv, Peer: "*"}

	resp, err := handleSystemDispatch(ctx, []string{"watchdog", "withdraw", "dnsr"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, []string{"dnsr"}, receivedArgs)
}
