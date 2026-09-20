// Design: docs/architecture/api/process-protocol.md — Stage 2 config delivery
// Overview: startup.go — deliverConfigRPC, isConfigRefusal, configRefusalIsFatal

package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// fatalConfigPlugin is the registration name the two tests below configure. It
// names no shipped plugin: the mechanism is the same for every plugin that
// asks for a fatal refusal, and a test that named one would read as a rule
// about that plugin.
const fatalConfigPlugin = "test-fatal-on-config"

// registerFatalOnConfigPlugin registers a plugin that asks for a refusal of its
// configuration to stop ze.
func registerFatalOnConfigPlugin(t *testing.T) {
	t.Helper()
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:               fatalConfigPlugin,
		Description:        "stops ze when it refuses its configuration",
		FatalOnConfigError: true,
		RunEngine:          func(net.Conn) int { return 0 },
		CLIHandler:         func([]string) int { return 0 },
	}))
}

// newStage2Server builds the engine side of one Stage 2 delivery: a Server with
// no reactor and no coordinator, and the process the configure callback goes to.
func newStage2Server(t *testing.T, ctx context.Context) (*Server, *process.Process, *rpc.MuxConn) {
	t.Helper()
	registerFatalOnConfigPlugin(t)

	engineConn, pluginMux := newDriverPipe(t)

	s := &Server{
		config:        &ServerConfig{},
		registry:      plugin.NewPluginRegistry(),
		ctx:           ctx,
		loadedPlugins: map[string]bool{fatalConfigPlugin: true},
	}

	proc := process.NewProcess(plugin.PluginConfig{Name: fatalConfigPlugin})
	proc.SetRegistration(&plugin.PluginRegistration{})
	proc.SetConn(engineConn)

	return s, proc, pluginMux
}

// TestConfigTransportFailureDoesNotStopZe is the whole point of the
// distinction: a dead RPC is not a verdict on the configuration.
//
// VALIDATES: a Stage 2 delivery that dies on the transport leaves startupErr
// unset even for a plugin whose registration asks for a fatal refusal, so ze
// starts.
// PREVENTS: an operator losing the whole daemon because a pipe closed, a peer
// process was slow, or a context was canceled while the configuration was in
// flight. FatalOnConfigError used to make ANY error from SendConfigure fatal,
// and a transport failure is the majority of the errors that call can produce.
//
// The transport is killed BEFORE the engine sends, so the plugin says nothing
// at all -- which is exactly the state a crashed or disconnected plugin leaves
// the engine in.
func TestConfigTransportFailureDoesNotStopZe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s, proc, pluginMux := newStage2Server(t, ctx)
	require.NoError(t, pluginMux.Close())

	err := s.deliverConfigRPC(ctx, proc)

	require.Error(t, err, "a configure sent down a closed transport must fail")
	assert.False(t, isConfigRefusal(err),
		"a dead transport is not the plugin refusing its configuration")
	assert.NoError(t, s.startupErr,
		"ze must still start: the plugin never said anything about its configuration")
}

// TestConfigRefusalStopsZe is the other half, and without it the test above
// would pass over a server that never stops ze at all.
//
// VALIDATES: a plugin that answers Stage 2 with an error response stops ze when
// its registration asked for that.
// PREVENTS: a mistyped configuration producing a running router silently
// missing the feature the refusing plugin owns.
//
// The plugin side writes its refusal with rpc.MuxConn.SendError, which is the
// call the SDK itself makes for a configure handler that returns an error
// (sendCallbackError, pkg/plugin/sdk/sdk_dispatch.go). The SDK half -- that an
// OnConfigure error becomes that response rather than a dropped connection --
// is TestConfigureHandlerErrorReachesEngineAsAResponse in that package.
func TestConfigRefusalStopsZe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s, proc, pluginMux := newStage2Server(t, ctx)

	const reason = `address family "ipv6-unicat" is not a family`
	go func() {
		select {
		case req := <-pluginMux.Requests():
			pluginMux.SendError(ctx, req.ID, reason) //nolint:errcheck // test plugin side
		case <-ctx.Done():
		}
	}()

	err := s.deliverConfigRPC(ctx, proc)

	require.Error(t, err, "the engine must report the refusal to its caller")
	assert.True(t, isConfigRefusal(err), "an error response is the plugin's refusal")
	require.Error(t, s.startupErr, "a refused configuration must stop ze")
	assert.Contains(t, s.startupErr.Error(), fatalConfigPlugin,
		"the operator is told which plugin refused")
	assert.Contains(t, s.startupErr.Error(), reason,
		"and what it refused")
}

// TestConfigRefusalIsNotFatalWithoutTheFlag pins that the refusal mechanism
// adds no fatality of its own.
//
// VALIDATES: a plugin that refuses its configuration and did NOT ask for a
// fatal refusal leaves ze running, as it does today.
// PREVENTS: the recognition of a refusal being read as a reason to stop ze.
func TestConfigRefusalIsNotFatalWithoutTheFlag(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	const name = "test-tolerant-on-config"
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "runs ze on without it when it refuses its configuration",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))

	engineConn, pluginMux := newDriverPipe(t)
	s := &Server{
		config:        &ServerConfig{},
		registry:      plugin.NewPluginRegistry(),
		ctx:           ctx,
		loadedPlugins: map[string]bool{name: true},
	}
	proc := process.NewProcess(plugin.PluginConfig{Name: name})
	proc.SetRegistration(&plugin.PluginRegistration{})
	proc.SetConn(engineConn)

	go func() {
		select {
		case req := <-pluginMux.Requests():
			pluginMux.SendError(ctx, req.ID, "no") //nolint:errcheck // test plugin side
		case <-ctx.Done():
		}
	}()

	err := s.deliverConfigRPC(ctx, proc)

	require.Error(t, err)
	assert.True(t, isConfigRefusal(err))
	assert.NoError(t, s.startupErr,
		"a plugin that did not ask for a fatal refusal must not stop ze")
}
