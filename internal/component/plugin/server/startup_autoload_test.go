package server

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestCollectOrphanCandidates_HardAndOptional verifies the orphan-candidate
// collector walks BOTH Dependencies and OptionalDependencies when computing
// which plugins might be orphaned by a config-reload stop.
//
// VALIDATES: spec-rs-fastpath-2-adjrib orphan-stop sibling audit -- an
// optional dep is orphan-eligible identically to a hard dep. Without this,
// moving bgp-rs -> bgp-adj-rib-in from Dependencies to OptionalDependencies
// would silently break config-reload teardown of adj-rib-in.
// PREVENTS: adj-rib-in leaking across config reloads when its last optional
// user is removed.
func TestCollectOrphanCandidates_HardAndOptional(t *testing.T) {
	lookup := func(name string) *registry.Registration {
		switch name {
		case "plugin-hard-only":
			return &registry.Registration{Name: name, Dependencies: []string{"dep-a"}}
		case "plugin-optional-only":
			return &registry.Registration{Name: name, OptionalDependencies: []string{"dep-b"}}
		case "plugin-mixed":
			return &registry.Registration{
				Name:                 name,
				Dependencies:         []string{"dep-c"},
				OptionalDependencies: []string{"dep-d"},
			}
		case "plugin-no-deps":
			return &registry.Registration{Name: name}
		}
		return nil
	}

	tests := []struct {
		name    string
		stopped []string
		want    []string
	}{
		{
			name:    "hard dep only",
			stopped: []string{"plugin-hard-only"},
			want:    []string{"dep-a"},
		},
		{
			name:    "optional dep only",
			stopped: []string{"plugin-optional-only"},
			want:    []string{"dep-b"},
		},
		{
			name:    "mixed hard and optional",
			stopped: []string{"plugin-mixed"},
			want:    []string{"dep-c", "dep-d"},
		},
		{
			name:    "multiple plugins, union of deps",
			stopped: []string{"plugin-hard-only", "plugin-optional-only"},
			want:    []string{"dep-a", "dep-b"},
		},
		{
			name:    "plugin with no deps",
			stopped: []string{"plugin-no-deps"},
			want:    []string{},
		},
		{
			name:    "unknown plugin is skipped",
			stopped: []string{"not-registered"},
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stoppedSet := make(map[string]bool, len(tt.stopped))
			for _, n := range tt.stopped {
				stoppedSet[n] = true
			}
			got := collectOrphanCandidates(stoppedSet, lookup)
			assert.Equal(t, len(tt.want), len(got), "candidate count")
			for _, w := range tt.want {
				assert.True(t, got[w], "expected %q in candidate set, got %v", w, got)
			}
		})
	}
}

// TestPluginDependsOn verifies the dependency-check helper treats hard and
// optional deps identically when asking "does plugin X still need Y?"
//
// VALIDATES: orphan-stop's "has any other plugin still got this as a dep?"
// check covers both Dependencies and OptionalDependencies. Without this, a
// plugin declaring only OptionalDependencies on a shared resource would let
// the resource be orphan-stopped while still in use.
// PREVENTS: premature stop of a plugin that a remaining optional user needs.
func TestPluginDependsOn(t *testing.T) {
	tests := []struct {
		name      string
		reg       *registry.Registration
		candidate string
		want      bool
	}{
		{"nil registration", nil, "anything", false},
		{"empty registration", &registry.Registration{}, "anything", false},
		{"hard dep match", &registry.Registration{Dependencies: []string{"X"}}, "X", true},
		{"optional dep match", &registry.Registration{OptionalDependencies: []string{"X"}}, "X", true},
		{"hard dep no match", &registry.Registration{Dependencies: []string{"Y"}}, "X", false},
		{"optional dep no match", &registry.Registration{OptionalDependencies: []string{"Y"}}, "X", false},
		{
			"mixed, hard hit",
			&registry.Registration{Dependencies: []string{"X"}, OptionalDependencies: []string{"Y"}},
			"X", true,
		},
		{
			"mixed, optional hit",
			&registry.Registration{Dependencies: []string{"Y"}, OptionalDependencies: []string{"X"}},
			"X", true,
		},
		{
			"mixed, neither hit",
			&registry.Registration{Dependencies: []string{"Y"}, OptionalDependencies: []string{"Z"}},
			"X", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pluginDependsOn(tt.reg, tt.candidate))
		})
	}
}

// TestGetUnclaimedEventTypePlugins verifies auto-loading plugins for custom event types.
//
// VALIDATES: Plugins producing custom event types are auto-loaded when not explicitly configured.
// PREVENTS: Custom event types silently ignored because producing plugin was not loaded.
func TestGetUnclaimedEventTypePlugins(t *testing.T) {
	tests := []struct {
		name              string
		customEvents      []string
		configuredPlugins []plugin.PluginConfig
		wantPluginNames   []string
		wantNil           bool
	}{
		{
			name:         "no_custom_events",
			customEvents: nil,
			wantNil:      true,
		},
		{
			name:         "unknown_event_type_returns_nil",
			customEvents: []string{"nonexistent-event"},
			wantNil:      true,
		},
		{
			name:         "known_event_type_auto_loads_plugin_and_deps",
			customEvents: []string{"update-rpki"},
			// bgp-rpki-decorator produces update-rpki, depends on bgp-rpki,
			// which depends on bgp-adj-rib-in. ResolveDependencies returns
			// all transitive dependencies in dependency-first order.
			wantPluginNames: []string{"bgp-rpki-decorator", "bgp", "bgp-rpki", "bgp-adj-rib-in"},
		},
		{
			name:         "already_configured_plugin_skipped",
			customEvents: []string{"update-rpki"},
			configuredPlugins: []plugin.PluginConfig{
				{Name: "bgp-rpki-decorator"},
			},
			// The producing plugin is already configured, so nothing to auto-load.
			// But bgp-rpki (dependency) is not configured -- however the producing
			// plugin itself is skipped, so no dependency resolution happens.
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				config: &ServerConfig{
					ConfiguredCustomEvents: tt.customEvents,
					Plugins:                tt.configuredPlugins,
				},
				registry: plugin.NewPluginRegistry(),
			}

			got := s.getUnclaimedEventTypePlugins()

			if tt.wantNil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)

			var names []string
			for _, p := range got {
				names = append(names, p.Name)
				assert.Equal(t, "json", p.Encoder, "auto-loaded plugin should use json encoder")
				assert.True(t, p.Internal, "auto-loaded plugin should be internal")
			}

			assert.Equal(t, tt.wantPluginNames, names)
		})
	}
}

// TestBGPPluginAutoLoads verifies that ConfigRoots "bgp" triggers BGP plugin
// auto-loading when the config contains a bgp { } section.
//
// VALIDATES: AC-1 -- Config with bgp { } auto-loads BGP plugin via ConfigRoots.
// PREVENTS: BGP plugin not loaded when bgp section is present in config.
func TestBGPPluginAutoLoads(t *testing.T) {
	s := &Server{
		config: &ServerConfig{
			ConfiguredPaths: []string{"bgp"},
		},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}

	got := s.getConfigPathPlugins()
	require.NotNil(t, got, "should auto-load plugins for bgp config path")

	var names []string
	for _, p := range got {
		names = append(names, p.Name)
	}
	assert.Contains(t, names, "bgp", "bgp plugin should be in the auto-load list")

	for _, p := range got {
		assert.True(t, p.Internal, "plugin %s should be internal", p.Name)
		assert.Equal(t, "json", p.Encoder, "plugin %s should use json encoder", p.Name)
	}
}

// TestEngineStartsWithoutBGP verifies that no BGP plugins are auto-loaded when
// the config has no bgp section.
//
// VALIDATES: AC-2/AC-5 -- Config without bgp section does not load BGP.
// PREVENTS: BGP plugin loading unconditionally regardless of config.
func TestEngineStartsWithoutBGP(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
	}{
		{name: "empty_paths", paths: nil},
		{name: "interface_only", paths: []string{"interface"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				config: &ServerConfig{
					ConfiguredPaths: tt.paths,
				},
				registry:      plugin.NewPluginRegistry(),
				loadedPlugins: make(map[string]bool),
			}

			got := s.getConfigPathPlugins()

			for _, p := range got {
				assert.NotEqual(t, "bgp", p.Name,
					"bgp plugin should not auto-load without bgp config path")
			}
		})
	}
}

func newAutoloadTeardownTestServer() (*Server, *process.ProcessManager) {
	s := &Server{
		config:        &ServerConfig{},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}
	pm := process.NewProcessManager(nil)
	s.procManager.Store(pm)
	return s, pm
}

func installAutoloadTeardownClaims(
	t *testing.T,
	s *Server,
	pm *process.ProcessManager,
	pluginName string,
	commands ...string,
) {
	t.Helper()
	proc := process.NewProcess(plugin.PluginConfig{Name: pluginName})
	proc.SetRegistration(&plugin.PluginRegistration{Name: pluginName, Commands: commands})
	pm.AddProcess(pluginName, proc)
	require.NoError(t, s.registry.Register(proc.Registration()))
	s.markPluginLoaded(pluginName)
}

func registerAutoloadReplacement(
	t *testing.T,
	s *Server,
	pm *process.ProcessManager,
	pluginName string,
	commands ...string,
) {
	t.Helper()
	proc := process.NewProcess(plugin.PluginConfig{Name: pluginName})
	proc.SetRegistration(&plugin.PluginRegistration{Name: pluginName, Commands: commands})
	pm.AddProcess(pluginName, proc)
	require.NoError(t, s.registry.Register(proc.Registration()))
}

// TestAutoStopForRemovedConfigPathsRollsBackRuntimeClaims verifies config-root
// removal uses the complete startup rollback path.
//
// VALIDATES: Removing a config root removes its process, command, and loaded-plugin marker.
// PREVENTS: A later reload failing to restart the plugin because its command remains registered.
func TestAutoStopForRemovedConfigPathsRollsBackRuntimeClaims(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const (
		pluginName      = "autoload-config-owner"
		replacementName = "autoload-config-owner-replacement"
		commandName     = "show autoload config owner"
		configRoot      = "autoload/config-owner"
	)
	require.NoError(t, registry.Register(registry.Registration{
		Name:        pluginName,
		Description: "config-root teardown test plugin",
		ConfigRoots: []string{configRoot},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))

	s, pm := newAutoloadTeardownTestServer()
	installAutoloadTeardownClaims(t, s, pm, pluginName, commandName)

	s.autoStopForRemovedConfigPaths([]string{configRoot})

	assert.Nil(t, pm.GetProcess(pluginName))
	assert.Empty(t, s.registry.LookupCommand(commandName))
	assert.False(t, s.isPluginLoaded(pluginName))

	registerAutoloadReplacement(t, s, pm, replacementName, commandName)
	assert.Equal(t, replacementName, s.registry.LookupCommand(commandName))
}

// TestStopOrphanedDependenciesRollsBackRuntimeClaims verifies orphan cleanup
// uses the complete startup rollback path across a transitive dependency chain.
//
// VALIDATES: Orphan cleanup removes each process, command, and loaded-plugin marker.
// PREVENTS: A later reload failing to restart an orphan because its command remains registered.
func TestStopOrphanedDependenciesRollsBackRuntimeClaims(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const (
		parentName        = "autoload-orphan-parent"
		orphanName        = "autoload-orphan"
		transitiveName    = "autoload-orphan-transitive"
		replacementName   = "autoload-orphan-replacement"
		orphanCommand     = "show autoload orphan"
		transitiveCommand = "show autoload orphan transitive"
	)
	require.NoError(t, registry.Register(registry.Registration{
		Name:         parentName,
		Description:  "orphan teardown parent",
		Dependencies: []string{orphanName},
		RunEngine:    func(net.Conn) int { return 0 },
		CLIHandler:   func([]string) int { return 0 },
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:         orphanName,
		Description:  "orphan teardown dependency",
		Dependencies: []string{transitiveName},
		RunEngine:    func(net.Conn) int { return 0 },
		CLIHandler:   func([]string) int { return 0 },
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:        transitiveName,
		Description: "transitive orphan teardown dependency",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))

	s, pm := newAutoloadTeardownTestServer()
	installAutoloadTeardownClaims(t, s, pm, orphanName, orphanCommand)
	installAutoloadTeardownClaims(t, s, pm, transitiveName, transitiveCommand)
	stopped := map[string]bool{parentName: true}

	s.stopOrphanedDependencies(pm, stopped)

	assert.True(t, stopped[orphanName])
	assert.True(t, stopped[transitiveName])
	assert.Nil(t, pm.GetProcess(orphanName))
	assert.Nil(t, pm.GetProcess(transitiveName))
	assert.Empty(t, s.registry.LookupCommand(orphanCommand))
	assert.Empty(t, s.registry.LookupCommand(transitiveCommand))
	assert.False(t, s.isPluginLoaded(orphanName))
	assert.False(t, s.isPluginLoaded(transitiveName))

	registerAutoloadReplacement(t, s, pm, replacementName, orphanCommand, transitiveCommand)
	assert.Equal(t, replacementName, s.registry.LookupCommand(orphanCommand))
	assert.Equal(t, replacementName, s.registry.LookupCommand(transitiveCommand))
}

// TestRollbackStartupProcessCleansRuntimeStateOnce verifies an old process
// cannot repeat name-scoped cleanup after a replacement generation starts.
//
// VALIDATES: Startup rollback runs runtime cleanup once for each process generation.
// PREVENTS: An old command-loop defer removing route state owned by its replacement.
func TestRollbackStartupProcessCleansRuntimeStateOnce(t *testing.T) {
	const (
		pluginName  = "autoload-generation-owner"
		commandName = "show autoload generation owner"
	)

	s, pm := newAutoloadTeardownTestServer()
	installAutoloadTeardownClaims(t, s, pm, pluginName, commandName)
	oldProc := pm.GetProcess(pluginName)

	s.rollbackStartupProcess(oldProc)

	replacementRoute := routeKey{
		fam:    v4u(),
		prefix: mustPfx(t, "192.0.2.0/24"),
	}
	s.recordInstalled(pluginName, []routeKey{replacementRoute})
	replacement := process.NewProcess(plugin.PluginConfig{Name: pluginName})
	pm.AddProcess(pluginName, replacement)

	s.cleanupProcess(oldProc)

	s.routeMu.Lock()
	_, tracked := s.installedByPlugin[pluginName][replacementRoute]
	s.routeMu.Unlock()
	assert.True(t, tracked)
	assert.Same(t, replacement, pm.GetProcess(pluginName))
}

// TestRollbackStartupProcessWaitsForRuntimeDrain verifies rollback waits for
// in-flight runtime requests before it removes startup claims.
//
// VALIDATES: A running process becomes reloadable only after runtime cleanup completes.
// PREVENTS: In-flight requests publishing name-scoped state after rollback reports completion.
func TestRollbackStartupProcessWaitsForRuntimeDrain(t *testing.T) {
	const (
		pluginName      = "autoload-runtime-drain-owner"
		replacementName = "autoload-runtime-drain-replacement"
		commandName     = "show autoload runtime drain"
		gatedCommand    = "show autoload runtime gate"
	)

	pluginSide, engineSide := net.Pipe()
	t.Cleanup(func() {
		_ = pluginSide.Close()
		_ = engineSide.Close()
	})

	entered := make(chan struct{})
	release := make(chan struct{})
	dispatcher := NewDispatcher()
	dispatcher.Register(gatedCommand, func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		close(entered)
		<-release
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, gatedCommand)

	s, pm := newAutoloadTeardownTestServer()
	s.dispatcher = dispatcher
	s.subscriptions = newSubscriptionManager()
	s.ctx, s.cancel = context.WithCancel(context.Background())
	t.Cleanup(s.cancel)

	proc := process.NewProcess(plugin.PluginConfig{Name: pluginName})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))
	proc.SetStage(plugin.StageRunning)
	proc.SetRegistration(&plugin.PluginRegistration{Name: pluginName, Commands: []string{commandName}})
	pm.AddProcess(pluginName, proc)
	require.NoError(t, s.registry.Register(proc.Registration()))
	s.markPluginLoaded(pluginName)

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()

	pluginConn := rpc.NewConn(pluginSide, pluginSide)
	callReturned := make(chan error, 1)
	go func() {
		_, err := pluginConn.CallRPC(context.Background(), "ze-plugin-engine:dispatch-command", &rpc.DispatchCommandInput{
			Command: gatedCommand,
		})
		callReturned <- err
	}()

	waitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	select {
	case <-entered:
	case <-waitCtx.Done():
		t.Fatal("runtime request did not enter the gated handler")
	}

	rollbackDone := make(chan struct{})
	go func() {
		s.rollbackStartupProcess(proc)
		close(rollbackDone)
	}()

	select {
	case err := <-callReturned:
		require.Error(t, err)
	case <-waitCtx.Done():
		t.Fatal("rollback did not stop the plugin connection")
	}
	select {
	case <-rollbackDone:
		t.Fatal("rollback completed before the in-flight runtime request drained")
	default:
	}

	replacement := process.NewProcess(plugin.PluginConfig{Name: replacementName})
	replacement.SetRegistration(&plugin.PluginRegistration{
		Name:     replacementName,
		Commands: []string{commandName},
	})
	require.Error(t, s.registry.Register(replacement.Registration()))

	close(release)
	select {
	case <-rollbackDone:
	case <-waitCtx.Done():
		t.Fatal("rollback did not complete after the runtime request drained")
	}
	select {
	case <-handlerDone:
	case <-waitCtx.Done():
		t.Fatal("runtime handler did not exit after cleanup")
	}

	require.NoError(t, s.registry.Register(replacement.Registration()))
	pm.AddProcess(replacementName, replacement)
	assert.Equal(t, replacementName, s.registry.LookupCommand(commandName))
}

// TestSocketReloadRefusesToStopCallingProcess verifies a socket-dispatched
// reload fails before config mutation when the new config removes its caller.
//
// VALIDATES: Self-removing socket reload returns an error and preserves config and claims.
// PREVENTS: request reload deadlocking while rollback waits for its own dispatch.
func TestSocketReloadRefusesToStopCallingProcess(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const (
		pluginName  = "autoload-self-reload-socket"
		commandName = "show autoload self reload socket"
		configRoot  = "self-reload/socket"
	)
	require.NoError(t, registry.Register(registry.Registration{
		Name:        pluginName,
		Description: "socket self-reload test plugin",
		ConfigRoots: []string{configRoot},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))

	running := map[string]any{"self-reload": map[string]any{"socket": map[string]any{"enabled": true}}}
	reactor := &mockReloadReactor{tree: running}
	s, pm := newAutoloadTeardownTestServer()
	s.reactor = reactor
	s.dispatcher = NewDispatcher()
	s.subscriptions = newSubscriptionManager()
	s.ctx, s.cancel = context.WithCancel(context.Background())
	t.Cleanup(s.cancel)
	s.dispatcher.Register("request reload", handleDaemonReload, "reload")
	s.SetFullReloadFunc(func(ctx context.Context) error {
		return s.ReloadConfig(ctx, map[string]any{})
	})

	pluginSide, engineSide := net.Pipe()
	t.Cleanup(func() {
		_ = pluginSide.Close()
		_ = engineSide.Close()
	})
	proc := process.NewProcess(plugin.PluginConfig{Name: pluginName})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))
	proc.SetRegistration(&plugin.PluginRegistration{Name: pluginName, Commands: []string{commandName}})
	pm.AddProcess(pluginName, proc)
	require.NoError(t, s.registry.Register(proc.Registration()))
	s.markPluginLoaded(pluginName)

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()

	callCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := rpc.NewConn(pluginSide, pluginSide).CallRPC(
		callCtx,
		"ze-plugin-engine:dispatch-command",
		&rpc.DispatchCommandInput{Command: "request reload"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "would stop calling plugin")
	assert.Nil(t, reactor.setTree)
	assert.Equal(t, running, reactor.GetConfigTree())
	assert.Same(t, proc, pm.GetProcess(pluginName))
	assert.Equal(t, pluginName, s.registry.LookupCommand(commandName))
	assert.True(t, s.isPluginLoaded(pluginName))

	_ = pluginSide.Close()
	select {
	case <-handlerDone:
	case <-callCtx.Done():
		t.Fatal("socket runtime handler did not exit")
	}
}

// TestBridgeRollbackWaitsForDirectDispatch verifies rollback drains admitted
// plugin-to-engine bridge calls before it removes startup claims.
//
// VALIDATES: Bridge rollback rejects new calls and becomes reloadable after admitted calls finish.
// PREVENTS: Direct bridge handlers publishing name-scoped state after cleanup.
func TestBridgeRollbackWaitsForDirectDispatch(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const (
		pluginName      = "autoload-bridge-drain-owner"
		replacementName = "autoload-bridge-drain-replacement"
		commandName     = "show autoload bridge drain"
	)
	require.NoError(t, registry.Register(registry.Registration{
		Name:        pluginName,
		Description: "bridge drain test plugin",
		RunEngine: func(conn net.Conn) int {
			var buf [1]byte
			if _, err := conn.Read(buf[:]); err != nil {
				return 0
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	s, pm := newAutoloadTeardownTestServer()
	s.ctx, s.cancel = context.WithCancel(context.Background())
	t.Cleanup(s.cancel)
	proc := process.NewProcess(plugin.PluginConfig{Name: pluginName, Internal: true})
	require.NoError(t, proc.StartWithContext(s.ctx))
	require.NoError(t, proc.InitConns())
	proc.SetStage(plugin.StageRunning)
	proc.SetRegistration(&plugin.PluginRegistration{Name: pluginName, Commands: []string{commandName}})
	pm.AddProcess(pluginName, proc)
	require.NoError(t, s.registry.Register(proc.Registration()))
	s.markPluginLoaded(pluginName)

	bridge := proc.Bridge()
	require.NotNil(t, bridge)
	proc.Conn().SetBridge(bridge)
	entered := make(chan struct{})
	release := make(chan struct{})
	bridge.SetDispatchRPC(func(string, json.RawMessage) (json.RawMessage, error) {
		close(entered)
		<-release
		return json.RawMessage(`{"status":"done"}`), nil
	})
	bridge.SetReady()

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()
	callDone := make(chan error, 1)
	go func() {
		_, err := bridge.DispatchRPC("test", nil)
		callDone <- err
	}()

	waitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	select {
	case <-entered:
	case <-waitCtx.Done():
		t.Fatal("direct bridge call did not enter its handler")
	}

	rollbackDone := make(chan struct{})
	go func() {
		s.rollbackStartupProcess(proc)
		close(rollbackDone)
	}()
	select {
	case <-proc.Done():
	case <-waitCtx.Done():
		t.Fatal("rollback did not stop the bridge process")
	}
	_, err := bridge.DispatchRPC("rejected", nil)
	require.ErrorIs(t, err, rpc.ErrBridgeClosed)
	select {
	case <-rollbackDone:
		t.Fatal("bridge rollback completed before direct dispatch drained")
	default:
	}

	replacement := process.NewProcess(plugin.PluginConfig{Name: replacementName})
	replacement.SetRegistration(&plugin.PluginRegistration{Name: replacementName, Commands: []string{commandName}})
	require.Error(t, s.registry.Register(replacement.Registration()))

	close(release)
	select {
	case err := <-callDone:
		require.NoError(t, err)
	case <-waitCtx.Done():
		t.Fatal("admitted bridge call did not finish")
	}
	select {
	case <-rollbackDone:
	case <-waitCtx.Done():
		t.Fatal("bridge rollback did not complete after dispatch drained")
	}
	select {
	case <-handlerDone:
	case <-waitCtx.Done():
		t.Fatal("bridge runtime handler did not exit")
	}
	require.NoError(t, proc.Wait(waitCtx))

	require.NoError(t, s.registry.Register(replacement.Registration()))
	pm.AddProcess(replacementName, replacement)
	assert.Equal(t, replacementName, s.registry.LookupCommand(commandName))
}

// TestBridgeReloadRefusesToStopCallingProcess verifies a direct bridge reload
// fails before config mutation when the new config removes its caller.
//
// VALIDATES: Self-removing bridge reload returns promptly and preserves config and claims.
// PREVENTS: Direct request reload waiting for the dispatch that invoked it.
func TestBridgeReloadRefusesToStopCallingProcess(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const (
		pluginName  = "autoload-self-reload-bridge"
		commandName = "show autoload self reload bridge"
		configRoot  = "self-reload/bridge"
	)
	require.NoError(t, registry.Register(registry.Registration{
		Name:        pluginName,
		Description: "bridge self-reload test plugin",
		ConfigRoots: []string{configRoot},
		RunEngine: func(conn net.Conn) int {
			var buf [1]byte
			if _, err := conn.Read(buf[:]); err != nil {
				return 0
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	running := map[string]any{"self-reload": map[string]any{"bridge": map[string]any{"enabled": true}}}
	reactor := &mockReloadReactor{tree: running}
	s, pm := newAutoloadTeardownTestServer()
	s.reactor = reactor
	s.dispatcher = NewDispatcher()
	s.subscriptions = newSubscriptionManager()
	s.ctx, s.cancel = context.WithCancel(context.Background())
	t.Cleanup(s.cancel)
	s.dispatcher.Register("request reload", handleDaemonReload, "reload")
	s.SetFullReloadFunc(func(ctx context.Context) error {
		return s.ReloadConfig(ctx, map[string]any{})
	})

	proc := process.NewProcess(plugin.PluginConfig{Name: pluginName, Internal: true})
	require.NoError(t, proc.StartWithContext(s.ctx))
	require.NoError(t, proc.InitConns())
	proc.SetStage(plugin.StageRunning)
	proc.SetRegistration(&plugin.PluginRegistration{Name: pluginName, Commands: []string{commandName}})
	pm.AddProcess(pluginName, proc)
	require.NoError(t, s.registry.Register(proc.Registration()))
	s.markPluginLoaded(pluginName)

	bridge := proc.Bridge()
	require.NotNil(t, bridge)
	proc.Conn().SetBridge(bridge)
	s.wireBridgeDispatch(proc)
	bridge.SetReady()

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()
	callDone := make(chan error, 1)
	go func() {
		_, err := bridge.DispatchCommand("request reload")
		callDone <- err
	}()

	waitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	select {
	case err := <-callDone:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "would stop calling plugin")
	case <-waitCtx.Done():
		t.Fatal("self-removing bridge reload did not return")
	}
	assert.Nil(t, reactor.setTree)
	assert.Equal(t, running, reactor.GetConfigTree())
	assert.Same(t, proc, pm.GetProcess(pluginName))
	assert.Equal(t, pluginName, s.registry.LookupCommand(commandName))
	assert.True(t, s.isPluginLoaded(pluginName))
	assert.True(t, bridge.Ready())

	proc.Stop()
	select {
	case <-handlerDone:
	case <-waitCtx.Done():
		t.Fatal("bridge runtime handler did not exit")
	}
	require.NoError(t, proc.Wait(waitCtx))
}

// TestReloadKeepsSharedDependencyForNewConfigOwner verifies orphan selection
// is recomputed after auto-load adds a replacement dependent.
//
// VALIDATES: Removing owner A and adding owner B keeps their shared dependency running.
// PREVENTS: A pre-auto-load orphan snapshot stopping a dependency that owner B needs.
func TestReloadKeepsSharedDependencyForNewConfigOwner(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const (
		dependencyName = "autoload-swap-dependency"
		oldOwnerName   = "autoload-swap-old-owner"
		newOwnerName   = "autoload-swap-new-owner"
		commandName    = "show autoload swap owner"
		oldRoot        = "autoload-swap-old"
		newRoot        = "autoload-swap-new"
	)
	require.NoError(t, registry.Register(registry.Registration{
		Name:        dependencyName,
		Description: "shared dependency for reload swap",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:         oldOwnerName,
		Description:  "removed config owner",
		ConfigRoots:  []string{oldRoot},
		Dependencies: []string{dependencyName},
		RunEngine:    func(net.Conn) int { return 0 },
		CLIHandler:   func([]string) int { return 0 },
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:         newOwnerName,
		Description:  "added config owner",
		ConfigRoots:  []string{newRoot},
		Dependencies: []string{dependencyName},
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(newOwnerName, conn)
			if err := p.Run(context.Background(), sdk.Registration{
				Commands: []sdk.CommandDecl{{Name: commandName}},
			}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	oldTree := map[string]any{oldRoot: map[string]any{"enabled": true}}
	newTree := map[string]any{newRoot: map[string]any{"enabled": true}}
	reactor := &mockReloadReactor{tree: oldTree}
	s, spawner := newLifecycleStartupServer(t)
	s.reactor = reactor

	pm := process.NewProcessManager(nil)
	require.NoError(t, pm.StartWithContext(s.ctx))
	spawner.pm = pm
	s.procManager.Store(pm)

	oldOwner := process.NewProcess(plugin.PluginConfig{Name: oldOwnerName})
	dependency := process.NewProcess(plugin.PluginConfig{Name: dependencyName})
	pm.AddProcess(oldOwnerName, oldOwner)
	pm.AddProcess(dependencyName, dependency)
	s.markPluginLoaded(oldOwnerName)
	s.markPluginLoaded(dependencyName)

	require.NoError(t, s.ReloadConfig(context.Background(), newTree))

	assert.Nil(t, pm.GetProcess(oldOwnerName))
	assert.Same(t, dependency, pm.GetProcess(dependencyName))
	newOwner := pm.GetProcess(newOwnerName)
	require.NotNil(t, newOwner)
	assert.True(t, newOwner.Running())
	assert.Equal(t, plugin.StageRunning, newOwner.Stage())
	assert.Equal(t, newOwnerName, s.registry.LookupCommand(commandName))
	assert.Equal(t, newTree, reactor.setTree)
}

// TestRemovingLastConfigPluginDoesNotStopServer verifies an empty runtime
// plugin set after reload does not signal daemon shutdown.
//
// VALIDATES: Config teardown leaves Server.Wait blocked until an explicit shutdown request.
// PREVENTS: Removing the final auto-loaded plugin terminating a no-BGP/API daemon.
func TestRemovingLastConfigPluginDoesNotStopServer(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const (
		pluginName = "autoload-last-runtime-plugin"
		configRoot = "autoload-last-runtime"
	)
	require.NoError(t, registry.Register(registry.Registration{
		Name:        pluginName,
		Description: "last runtime plugin lifecycle test",
		ConfigRoots: []string{configRoot},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))

	reactor := &mockReactor{}
	s, err := NewServer(&ServerConfig{}, reactor)
	require.NoError(t, err)
	require.NoError(t, s.StartWithContext(context.Background()))
	t.Cleanup(s.Stop)

	pm := process.NewProcessManager(nil)
	proc := process.NewProcess(plugin.PluginConfig{Name: pluginName})
	pm.AddProcess(pluginName, proc)
	s.procManager.Store(pm)
	s.markPluginLoaded(pluginName)

	s.autoStopForRemovedConfigPaths([]string{configRoot})

	assert.Nil(t, pm.GetProcess(pluginName))
	assert.False(t, s.isPluginLoaded(pluginName))
	select {
	case <-s.shutdownRequested:
		t.Fatal("config teardown signaled server shutdown")
	default:
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	err = s.Wait(waitCtx)
	cancel()
	require.ErrorIs(t, err, context.DeadlineExceeded)

	resp, err := handleDaemonShutdown(&CommandContext{Server: s}, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	resp.TransportComplete()
	assert.True(t, reactor.stopped)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	require.NoError(t, s.Wait(shutdownCtx))
}

// TestExplicitShutdownReleasesWaitBeforeRuntimeHandlerDrain verifies the hub
// can leave its wait loop and call Stop while a command handler is still alive.
//
// VALIDATES: Explicit shutdown releases Server.Wait before Stop drains runtime handlers.
// PREVENTS: request shutdown deadlocking on the handler that issued the request.
func TestExplicitShutdownReleasesWaitBeforeRuntimeHandlerDrain(t *testing.T) {
	reactor := &mockReactor{}
	s, err := NewServer(&ServerConfig{}, reactor)
	require.NoError(t, err)
	require.NoError(t, s.StartWithContext(context.Background()))

	pluginSide, engineSide := net.Pipe()
	t.Cleanup(func() {
		_ = pluginSide.Close()
		_ = engineSide.Close()
	})
	proc := process.NewProcess(plugin.PluginConfig{Name: "shutdown-runtime-handler"})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))
	pm := process.NewProcessManager(nil)
	pm.AddProcess(proc.Name(), proc)
	s.procManager.Store(pm)

	handlerStarted := make(chan struct{})
	handlerDone := make(chan struct{})
	s.wg.Go(func() {
		defer close(handlerDone)
		close(handlerStarted)
		s.handleSingleProcessCommandsRPC(proc)
	})
	<-handlerStarted

	resp, err := handleDaemonShutdown(&CommandContext{Server: s}, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	resp.TransportComplete()

	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, s.Wait(waitCtx))
	select {
	case <-handlerDone:
		t.Fatal("runtime handler drained before Server.Stop closed its connection")
	default:
	}

	s.Stop()
	select {
	case <-handlerDone:
	case <-waitCtx.Done():
		t.Fatal("Server.Stop did not drain the tracked runtime handler")
	}

	s.wg.Add(1)
	blockedCtx, blockedCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	err = s.Wait(blockedCtx)
	blockedCancel()
	require.ErrorIs(t, err, context.DeadlineExceeded)
	s.wg.Done()
	require.NoError(t, s.Wait(waitCtx))
}
