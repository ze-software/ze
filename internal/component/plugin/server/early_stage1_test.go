package server

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// fakeDeclaredSpawner stands in for the plugin manager: it forks nothing and
// answers the processes and the start failures a test gave it.
type fakeDeclaredSpawner struct {
	manager  *process.ProcessManager
	failures map[string]error
	startErr error
	starts   int
	failed   map[string]error
}

func (f *fakeDeclaredSpawner) StartDeclared(context.Context, []plugin.PluginConfig) (map[string]error, error) {
	f.starts++
	return f.failures, f.startErr
}

func (f *fakeDeclaredSpawner) FailDeclared(_ context.Context, name string, cause error) {
	if f.failed == nil {
		f.failed = make(map[string]error)
	}
	f.failed[name] = cause
}

func (f *fakeDeclaredSpawner) GetProcessManager() any { return f.manager }

// heldPlugin builds a started external plugin as the early step finds it: a
// process in the manager whose connection the test holds the plugin end of.
func heldPlugin(t *testing.T, manager *process.ProcessManager, cfg plugin.PluginConfig) (proc *process.Process, pluginSide net.Conn) {
	t.Helper()

	engineSide, pluginSide := net.Pipe()
	t.Cleanup(func() { _ = pluginSide.Close() })
	t.Cleanup(func() { _ = engineSide.Close() })
	proc = process.NewProcess(cfg)
	proc.SetConn(muxPluginConn(t, engineSide))
	manager.AddProcess(cfg.Name, proc)
	return proc, pluginSide
}

// TestEarlyStage1HoldsAtBarrier proves a declared external plugin's Stage 1
// is read before the full parse and the plugin is then held at the barrier:
// the early step answers the module the declaration carries and sends the
// plugin nothing, and the request it kept is the one the plugin's startup
// tier reads next and answers. The plugin waits for its Stage 1 answer, with
// no stage timer on it, and gets that answer on the request it sent.
//
// Method: a real PluginConn over a pipe, the plugin end sending a real
// declare-registration call that blocks until it is answered.
func TestEarlyStage1HoldsAtBarrier(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := plugin.PluginConfig{Name: "acme-monitor", Run: "acme-monitor-plugin", StageTimeout: 2 * time.Second}
	manager := process.NewProcessManager(nil)
	proc, pluginSide := heldPlugin(t, manager, cfg)
	spawner := &fakeDeclaredSpawner{manager: manager}

	answered := make(chan error, 1)
	go func() {
		pluginConn := rpc.NewConn(pluginSide, pluginSide)
		_, err := pluginConn.CallRPC(ctx, rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationInput{
			Schema: &rpc.SchemaDecl{Module: "acme-monitor-conf", Namespace: "urn:acme:monitor", YANGText: "module acme-monitor-conf {}"},
		})
		answered <- err
	}()

	declarations := HoldDeclaredPlugins(ctx, spawner, []plugin.PluginConfig{cfg})

	require.Len(t, declarations, 1)
	require.NoError(t, declarations[0].Err)
	assert.Equal(t, "acme-monitor", declarations[0].Plugin)
	require.NotNil(t, declarations[0].Schema)
	assert.Equal(t, "acme-monitor-conf", declarations[0].Schema.Module)
	assert.Equal(t, "urn:acme:monitor", declarations[0].Schema.Namespace)
	assert.Equal(t, "module acme-monitor-conf {}", declarations[0].Schema.YANGText)
	assert.Empty(t, spawner.failed, "a plugin that declared is not failed")

	// Held: the early step answered nothing, so the plugin's call is still
	// waiting. The wait is longer than the plugin's stage timeout would be
	// if a timer measured the hold.
	select {
	case err := <-answered:
		t.Fatalf("the early step answered the plugin's Stage 1 (err=%v); the answer belongs to its startup tier", err)
	case <-time.After(150 * time.Millisecond):
	}

	// The tier's handshake reads the kept request, not a new one, and its
	// answer reaches the plugin.
	request, err := proc.Conn().ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, rpc.MethodDeclareRegistration, request.Method)
	require.NoError(t, proc.Conn().SendResult(ctx, request.ID, &rpc.DeclareRegistrationOutput{PluginName: "acme-monitor"}))

	select {
	case err := <-answered:
		require.NoError(t, err, "the plugin's Stage 1 call was answered by the tier")
	case <-time.After(5 * time.Second):
		t.Fatal("the plugin never received the answer the tier sent on the kept request")
	}
}

// TestEarlyStage1KeepsFailureOutcome proves a plugin that does not declare is
// an outcome with its reason, never the end of the step: each case answers an
// error naming the plugin, and only a plugin from which nothing was read is
// stopped (FailDeclared). A plugin that sent something else stays held, so its
// tier refuses it on the request it sent.
func TestEarlyStage1KeepsFailureOutcome(t *testing.T) {
	t.Run("could not be started", func(t *testing.T) {
		cfg := plugin.PluginConfig{Name: "absent-plugin", Run: "./no-such-program", StageTimeout: time.Second}
		cause := errors.New("plugin absent-plugin: start: no such file")
		spawner := &fakeDeclaredSpawner{
			manager:  process.NewProcessManager(nil),
			failures: map[string]error{"absent-plugin": cause},
		}

		declarations := HoldDeclaredPlugins(context.Background(), spawner, []plugin.PluginConfig{cfg})

		require.Len(t, declarations, 1)
		require.ErrorIs(t, declarations[0].Err, cause)
		assert.ErrorContains(t, declarations[0].Err, `external plugin "absent-plugin" could not be started`)
		assert.ErrorContains(t, declarations[0].Err, `fix: ze show plugin conformance run "./no-such-program"`)
		assert.Nil(t, declarations[0].Schema)
		assert.Empty(t, spawner.failed, "a plugin that never started holds nothing to stop")
	})

	t.Run("connected and silent", func(t *testing.T) {
		cfg := plugin.PluginConfig{Name: "silent", Run: "silent-plugin", StageTimeout: 50 * time.Millisecond}
		manager := process.NewProcessManager(nil)
		heldPlugin(t, manager, cfg)
		spawner := &fakeDeclaredSpawner{manager: manager}

		begun := time.Now()
		declarations := HoldDeclaredPlugins(context.Background(), spawner, []plugin.PluginConfig{cfg})

		require.Len(t, declarations, 1)
		require.Error(t, declarations[0].Err, "a silent plugin is bounded, not waited for")
		assert.ErrorContains(t, declarations[0].Err, `external plugin "silent" connected and sent no declaration within 50ms`)
		assert.ErrorContains(t, declarations[0].Err, "ze.plugin.stage.timeout")
		assert.Less(t, time.Since(begun), 5*time.Second, "the wait is the plugin's stage timeout")
		assert.Same(t, declarations[0].Err, spawner.failed["silent"], "the stopped plugin keeps the reason the load reports")
	})

	t.Run("connection ended", func(t *testing.T) {
		cfg := plugin.PluginConfig{Name: "gone", Run: "gone-plugin", StageTimeout: 5 * time.Second}
		manager := process.NewProcessManager(nil)
		_, pluginSide := heldPlugin(t, manager, cfg)
		require.NoError(t, pluginSide.Close())
		spawner := &fakeDeclaredSpawner{manager: manager}

		declarations := HoldDeclaredPlugins(context.Background(), spawner, []plugin.PluginConfig{cfg})

		require.Len(t, declarations, 1)
		require.Error(t, declarations[0].Err)
		assert.ErrorContains(t, declarations[0].Err, `external plugin "gone" sent no declaration: its connection ended`)
		assert.Contains(t, spawner.failed, "gone")
	})

	t.Run("first request is not a declaration", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cfg := plugin.PluginConfig{Name: "confused", Run: "confused-plugin", StageTimeout: 2 * time.Second}
		manager := process.NewProcessManager(nil)
		proc, pluginSide := heldPlugin(t, manager, cfg)
		spawner := &fakeDeclaredSpawner{manager: manager}
		go func() {
			pluginConn := rpc.NewConn(pluginSide, pluginSide)
			//nolint:errcheck // the test ends the call by canceling ctx
			pluginConn.CallRPC(ctx, "ze-plugin-engine:ready", nil)
		}()

		declarations := HoldDeclaredPlugins(ctx, spawner, []plugin.PluginConfig{cfg})

		require.Len(t, declarations, 1)
		require.Error(t, declarations[0].Err)
		assert.ErrorContains(t, declarations[0].Err, `external plugin "confused" sent no declaration: its first request was not `+rpc.MethodDeclareRegistration)
		assert.ErrorContains(t, declarations[0].Err, `first request "ze-plugin-engine:ready"`)
		assert.Empty(t, spawner.failed, "the plugin stays held: its tier refuses the request on the wire")
		request, err := proc.Conn().ReadRequest(ctx)
		require.NoError(t, err)
		assert.Equal(t, "ze-plugin-engine:ready", request.Method, "the tier reads the request the early step kept")
	})
}

// TestEarlyStage1SecondPluginKeepsItsDeclaration proves the step's one clock
// does not cost a plugin its declaration: a plugin read after a silent one,
// with its own bound already spent, still answers the declaration that was
// waiting in its connection.
func TestEarlyStage1SecondPluginKeepsItsDeclaration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	silent := plugin.PluginConfig{Name: "silent", Run: "silent-plugin", StageTimeout: 300 * time.Millisecond}
	prompt := plugin.PluginConfig{Name: "prompt", Run: "prompt-plugin", StageTimeout: 50 * time.Millisecond}
	manager := process.NewProcessManager(nil)
	heldPlugin(t, manager, silent)
	_, pluginSide := heldPlugin(t, manager, prompt)
	spawner := &fakeDeclaredSpawner{manager: manager}
	go func() {
		pluginConn := rpc.NewConn(pluginSide, pluginSide)
		//nolint:errcheck // the test ends the call by canceling ctx
		pluginConn.CallRPC(ctx, rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationInput{
			Schema: &rpc.SchemaDecl{Module: "prompt-conf", YANGText: "module prompt-conf {}"},
		})
	}()

	declarations := HoldDeclaredPlugins(ctx, spawner, []plugin.PluginConfig{silent, prompt})

	require.Len(t, declarations, 2)
	require.Error(t, declarations[0].Err)
	require.NoError(t, declarations[1].Err, "the declaration waited in the connection while the silent plugin was read")
	require.NotNil(t, declarations[1].Schema)
	assert.Equal(t, "prompt-conf", declarations[1].Schema.Module)
}

// TestEarlyStage1NoPluginStartsNothing proves the common case pays nothing:
// with no declared external plugin the step never calls the spawner.
func TestEarlyStage1NoPluginStartsNothing(t *testing.T) {
	spawner := &fakeDeclaredSpawner{}

	declarations := HoldDeclaredPlugins(context.Background(), spawner, nil)

	assert.Nil(t, declarations)
	assert.Zero(t, spawner.starts, "nothing is started when nothing is declared")
}

// TestEarlyStage1SpawnerRefusalIsEveryPluginsOutcome proves a start that
// forks nothing (no listener for the plugins to connect back to) is kept as
// the reason of every declared plugin.
func TestEarlyStage1SpawnerRefusalIsEveryPluginsOutcome(t *testing.T) {
	cause := errors.New("plugin TLS acceptor: no certificate authority")
	spawner := &fakeDeclaredSpawner{startErr: cause}
	configs := []plugin.PluginConfig{
		{Name: "one", Run: "one-plugin"},
		{Name: "two", Run: "two-plugin"},
	}

	declarations := HoldDeclaredPlugins(context.Background(), spawner, configs)

	require.Len(t, declarations, 2)
	for i, declaration := range declarations {
		assert.Equal(t, configs[i].Name, declaration.Plugin)
		require.ErrorIs(t, declaration.Err, cause)
	}
}
