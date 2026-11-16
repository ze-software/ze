//go:build ze_web && ze_bgp

// Design: docs/architecture/api/commands.md -- authenticated browser streaming.

package hub

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
