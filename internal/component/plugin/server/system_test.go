package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
	assert.False(t, called, "shutdown must wait for transport completion")
	resp.TransportComplete()
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
	assert.False(t, reactor.stopped, "shutdown must wait for transport completion")
	resp.TransportComplete()
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

type gatedStopReactor struct {
	*mockReactor
	entered     chan struct{}
	release     chan struct{}
	enterOnce   sync.Once
	releaseOnce sync.Once
}

func newGatedStopReactor() *gatedStopReactor {
	return &gatedStopReactor{
		mockReactor: &mockReactor{},
		entered:     make(chan struct{}),
		release:     make(chan struct{}),
	}
}

func (r *gatedStopReactor) Stop() {
	r.enterOnce.Do(func() { close(r.entered) })
	<-r.release
	r.mockReactor.Stop()
}

func (r *gatedStopReactor) releaseStop() {
	r.releaseOnce.Do(func() { close(r.release) })
}

// TestDaemonTerminationSocketResponsePrecedesWait verifies every accepted
// daemon-termination command answers the admitted socket RPC before it releases
// the daemon wait loop.
//
// VALIDATES: shutdown, reboot, and quit return their success response before
// Server.Wait reports the explicit termination request.
// PREVENTS: Server.Stop closing the requesting process connection while its
// successful response is still waiting to be written.
func TestDaemonTerminationSocketResponsePrecedesWait(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wire    func(*Server, *gatedStopReactor)
	}{
		{name: "shutdown", command: "request shutdown"},
		{
			name:    "reboot",
			command: "request reboot",
			wire: func(s *Server, r *gatedStopReactor) {
				s.SetRebootFunc(r.Stop)
			},
		},
		{name: "quit", command: "request halt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			reactor := newGatedStopReactor()
			t.Cleanup(reactor.releaseStop)

			s, err := NewServer(&ServerConfig{}, reactor)
			require.NoError(t, err)
			require.NoError(t, s.StartWithContext(context.Background()))
			if tt.wire != nil {
				tt.wire(s, reactor)
			}

			pluginSide, engineSide := net.Pipe()
			t.Cleanup(func() {
				_ = pluginSide.Close()
				_ = engineSide.Close()
			})

			proc := process.NewProcess(plugin.PluginConfig{Name: "daemon-termination-caller"})
			proc.SetConn(plugipc.NewPluginConn(engineSide, engineSide))
			pm := process.NewProcessManager(nil)
			pm.AddProcess(proc.Name(), proc)
			s.procManager.Store(pm)
			s.wg.Go(func() {
				s.handleSingleProcessCommandsRPC(proc)
			})

			waitDone := make(chan error, 1)
			go func() {
				waitDone <- s.Wait(testCtx)
			}()

			type callResult struct {
				status string
				err    error
			}
			callDone := make(chan callResult, 1)
			go func() {
				status, _, callErr := dispatchAnswerOverSocket(testCtx, pluginSide, &rpc.DispatchCommandInput{
					Command: tt.command,
				})
				callDone <- callResult{status: status, err: callErr}
			}()

			var result callResult
			select {
			case result = <-callDone:
			case <-testCtx.Done():
				t.Fatal("daemon command did not return its socket response")
			}
			require.NoError(t, result.err)
			assert.Equal(t, plugin.StatusDone, result.status)

			select {
			case <-reactor.entered:
			case <-testCtx.Done():
				t.Fatal("daemon command did not begin reactor shutdown after writing its response")
			}
			select {
			case err := <-waitDone:
				t.Fatalf("Server.Wait unblocked before the accepted response completed: %v", err)
			default:
			}

			reactor.releaseStop()
			select {
			case err := <-waitDone:
				require.NoError(t, err)
			case <-testCtx.Done():
				t.Fatal("daemon command did not unblock Server.Wait")
			}

			s.Stop()
			require.NoError(t, s.Wait(testCtx))
		})
	}
}

// TestCommandRowsCarryLongHelp: `system command list` is the only answer the
// attached console of `ze start --cli` builds its command tree from. The long
// explanation therefore travels on it, beside the summary. A command that
// declares no explanation yields a row without the key. That is how the console
// tells an unwritten explanation from a written one.
//
// VALIDATES: the command list carries the explanation of a builtin and of a
// registered plugin command.
// PREVENTS: `?` in the attached console answering "no explanation is declared"
// for every command.
func TestCommandRowsCarryLongHelp(t *testing.T) {
	d := NewDispatcher()
	handler := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	d.RegisterWithOptions("show explained", handler, "Show the explained thing",
		RegisterOptions{LongHelp: "The explanation a builtin declares."})
	d.Register("show bare", handler, "Show the bare thing")

	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})
	for _, result := range d.Registry().Register(proc, []CommandDef{
		{Name: "request explained", Description: "Explained plugin command", LongHelp: "The explanation a plugin declares."},
		{Name: "request bare", Description: "Bare plugin command"},
	}) {
		require.True(t, result.OK, "register %s: %s", result.Name, result.Error)
	}

	rows := make(map[string]map[string]any)
	for record := range rpc.HeldRecords(commandRows(d, false)) {
		require.Nil(t, record.Fault, "commandRows yielded a rejected row")
		var row map[string]any
		require.NoError(t, json.Unmarshal(record.Item, &row))
		value, _ := row["value"].(string)
		rows[value] = row
	}

	declared := map[string]string{
		"show explained":    "The explanation a builtin declares.",
		"request explained": "The explanation a plugin declares.",
	}
	for name, want := range declared {
		row, ok := rows[name]
		require.True(t, ok, "no row for %q", name)
		assert.Equal(t, want, row["long-help"], "long-help of %q", name)
	}

	for _, name := range []string{"show bare", "request bare"} {
		row, ok := rows[name]
		require.True(t, ok, "no row for %q", name)
		_, present := row["long-help"]
		assert.False(t, present, "row for %q carries long-help, want the key absent", name)
	}
}
