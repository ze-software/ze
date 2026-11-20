//go:build ze_web && ze_bgp

// Design: docs/architecture/api/commands.md -- authenticated browser streaming.

package hub

import (
	"bytes"
	"context"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestWebDaemonStreamUsesRegistryIdentityAndAccounting executes the registered
// event monitor, cancels after its first real output and checks subscription
// cleanup and START/STOP identity. No SSH listener or transport is involved.
// MUTATION: omit the handler call in serverStreamDispatcher; no header is read.
func TestWebDaemonStreamUsesRegistryIdentityAndAccounting(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	require.NoError(t, server.StartWithContext(t.Context()))
	accounting := &webStreamAccounting{}
	server.Dispatcher().SetAccountingHook(accounting)
	server.Dispatcher().SetAuthorizer(webStreamPolicy{allowed: false})
	stream := serverStreamDispatcher(server)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	output := &webStreamCancelWriter{cancel: cancel}
	handled, err := stream(ctx, plugin.CallerIdentity{
		Username: "alice", RemoteAddr: "192.0.2.25:1234", Authorizer: webStreamPolicy{allowed: true},
	}, "monitor event", output)
	require.True(t, handled)
	require.NoError(t, err)
	assert.NotEmpty(t, output.String())
	assert.Zero(t, server.Monitors().Count())
	assert.Equal(t, []string{"alice|192.0.2.25:1234|monitor event"}, accounting.starts)
	assert.Equal(t, accounting.starts, accounting.stops)
}

// TestWebDaemonStreamDenialAndInvalidArgsStayUnexecuted proves identity policy
// and schema validation both run before a registered producer can write.
func TestWebDaemonStreamDenialAndInvalidArgsStayUnexecuted(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	require.NoError(t, server.StartWithContext(t.Context()))
	accounting := &webStreamAccounting{}
	server.Dispatcher().SetAccountingHook(accounting)
	stream := serverStreamDispatcher(server)
	var output bytes.Buffer
	caller := plugin.CallerIdentity{Username: "blocked", Authorizer: webStreamPolicy{allowed: false}}
	handled, err := stream(t.Context(), caller, "monitor event", &output)
	assert.True(t, handled)
	assert.ErrorIs(t, err, plugin.ErrStreamUnauthorized)
	assert.Empty(t, output.String())
	caller.Authorizer = webStreamPolicy{allowed: true}
	handled, err = stream(t.Context(), caller, "monitor event include", &output)
	assert.True(t, handled)
	assert.Error(t, err)
	assert.Empty(t, output.String())
	handled, err = stream(t.Context(), caller, "monitor event | head 1", &output)
	assert.True(t, handled)
	assert.ErrorContains(t, err, "pipe")
	assert.Empty(t, output.String())
	assert.Len(t, accounting.starts, 3)
	assert.Equal(t, accounting.starts, accounting.stops)
	handled, err = stream(t.Context(), caller, "show version", &output)
	assert.False(t, handled)
	assert.NoError(t, err)
	assert.Len(t, accounting.starts, 3)
}

// TestWebDaemonStreamBeforeStartupRefuses proves a transport cannot turn the
// daemon's unstarted lifecycle into a panic or a successful empty response.
func TestWebDaemonStreamBeforeStartupRefuses(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	var output bytes.Buffer
	handled, err := serverStreamDispatcher(server)(t.Context(), plugin.CallerIdentity{
		Username: "alice", Authorizer: webStreamPolicy{allowed: true},
	}, "monitor event", &output)
	assert.True(t, handled)
	assert.ErrorIs(t, err, errServerNotReady)
	assert.Empty(t, output.String())
}

type webStreamPolicy struct{ allowed bool }

func (p webStreamPolicy) Authorize(_, _, _ string, _ bool) bool { return p.allowed }

type webStreamAccounting struct{ starts, stops []string }

func (a *webStreamAccounting) CommandStart(username, remote, input string) string {
	a.starts = append(a.starts, username+"|"+remote+"|"+input)
	return "web-stream-task"
}

func (a *webStreamAccounting) CommandStop(_, username, remote, input string) {
	a.stops = append(a.stops, username+"|"+remote+"|"+input)
}

type webStreamCancelWriter struct {
	bytes.Buffer
	cancel context.CancelFunc
}

func (w *webStreamCancelWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	w.cancel()
	return n, err
}

var _ io.Writer = (*webStreamCancelWriter)(nil)

// TestStreamRoutePartitionsAndHandsDeclaredOperations asks whether a monitor
// line with a chain reaches its streaming handler partitioned on the two stream
// sites in this package: the web site (serverStreamCommand) and the REST and
// gRPC site (apiStreamSource, which APIEngine.Stream calls). The monitor's own
// declared operation must arrive in the declared list of ValidatedArgs, and no
// chain word may arrive as an argument.
//
// Method: a monitor prefix declares one operation, `only`, taking an argument.
// The line carries one argument before the pipe and that operation after it,
// and the registered handler records what it was handed.
//
// Red before the partition: the web site folds the operation into the command
// (arguments `alpha only up`), the REST and gRPC site splits nothing
// (arguments `alpha | only up`), and neither hands a declared list.
// MUTATION: drop the declared list GetStreamingHandlerForCommand attaches; the
// operations comparison fails on both sites.
func TestStreamRoutePartitionsAndHandsDeclaredOperations(t *testing.T) {
	const prefix = "monitor test stream-declared-operations"
	const line = prefix + " alpha | only up"
	type handed struct {
		tokens     []string
		operations []command.PipeOperation
	}
	calls := make(chan handed, 1)
	pluginserver.RegisterStreamingHandler(prefix, pluginserver.OneShotStreamingOnly,
		func(_ context.Context, _ *pluginserver.Server, output io.Writer, _ string, args command.ValidatedArgs) error {
			calls <- handed{tokens: args.Tokens(), operations: args.PipeOperations()}
			_, err := io.WriteString(output, "{\"state\":\"up\"}\n")
			return err
		})
	t.Cleanup(func() { pluginserver.UnregisterStreamingHandler(prefix) })
	command.RegisterPipeFilters([]string{prefix},
		command.PipeFilter{Name: "only", Description: "Keep the events of one state", Argument: command.ArgText})

	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	require.NoError(t, server.StartWithContext(t.Context()))
	allow := webStreamPolicy{allowed: true}

	sites := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"web", func(t *testing.T) {
			var output bytes.Buffer
			handled, runErr := serverStreamDispatcher(server)(t.Context(), plugin.CallerIdentity{
				Username: "alice", RemoteAddr: "192.0.2.25:1234", Authorizer: allow,
			}, line, &output)
			require.True(t, handled)
			require.NoError(t, runErr)
		}},
		{"rest and grpc", func(t *testing.T) {
			events, cancel, runErr := apiStreamSource(server)(t.Context(), api.CallerIdentity{
				Username: "alice", RemoteAddr: "192.0.2.25:1234", Authorizer: allow,
			}, line)
			require.NoError(t, runErr)
			for range events {
			}
			cancel()
		}},
	}
	want := []command.PipeOperation{{Name: "only", Args: []string{"up"}}}
	for _, site := range sites {
		t.Run(site.name, func(t *testing.T) {
			site.run(t)
			var got handed
			select {
			case got = <-calls:
			case <-time.After(3 * time.Second):
				t.Fatal("the streaming handler was never called")
			}
			assert.Equal(t, []string{"alpha"}, got.tokens,
				"a chain word reached the monitor as an argument")
			assert.True(t, slices.EqualFunc(want, got.operations, sameStreamPipeOperation),
				"declared list = %+v, want %+v", got.operations, want)
		})
	}
}

// sameStreamPipeOperation compares two declared operations by name and
// argument words.
func sameStreamPipeOperation(a, b command.PipeOperation) bool {
	return a.Name == b.Name && slices.Equal(a.Args, b.Args)
}
