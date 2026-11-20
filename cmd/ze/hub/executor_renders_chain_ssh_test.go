//go:build ze_ssh

// Design: docs/architecture/api/commands.md -- a rendering surface sends the typed line and renders the residual chain.
// Related: service_ssh.go -- the two SSH exec executors these tests drive.

package hub

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zessh "github.com/ze-software/ze/internal/component/ssh"
)

// TestSSHDaemonExecutorRendersResidualChain asks whether the daemon's SSH
// exec executor (sshWireImpl) dispatches the typed line as a caller that
// renders the residual chain. The SSH exec channel applies that chain to the
// answer itself (zessh.CommandExecutor), so a dispatch without the statement
// would apply `| count` twice.
//
// Method: the real sshWireImpl over a real plugin server, one read-only
// command whose handler records the CommandContext's RendersResidualChain,
// and the executor the server hands a session for `<command> | count`.
//
// MUTATION: drop RendersResidualChain from sshWireImpl's CommandContext; the
// handler records false and the dispatcher answers the count, not the rows.
func TestSSHDaemonExecutorRendersResidualChain(t *testing.T) {
	const path = "show fixture ssh-exec rows"
	apiServer, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(apiServer.Stop)
	require.NoError(t, apiServer.StartWithContext(t.Context()))
	rows := plugin.Map{"rows": []any{map[string]any{"name": "a"}, map[string]any{"name": "b"}}}
	var calls int
	var renders bool
	require.NoError(t, apiServer.Dispatcher().RegisterWithOptions(path, func(ctx *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		calls++
		renders = ctx.RendersResidualChain
		return plugin.NewResponse(plugin.StatusDone, rows), nil
	}, "Read two fixture rows", pluginserver.RegisterOptions{ReadOnly: true}))

	reactor := &lifecycleReactor{dispatcher: apiServer.Dispatcher()}
	params := infra.HookParams{
		Reactor:            reactor,
		APIServer:          func() *pluginserver.Server { return apiServer },
		FormatResponseData: func(any) string { return "formatted" },
	}
	sshSrv, err := zessh.NewServer(zessh.Config{HostKeyPath: filepath.Join(t.TempDir(), "host-key")})
	require.NoError(t, err)
	sshWireImpl(sshSrv, &sshWireInputs{Reactor: reactor, Params: params})

	executor := sshSrv.ExecutorForUser(t.Context(), "operator", "192.0.2.46:2200", sshRunAllowPolicy())
	require.NotNil(t, executor)
	response, err := executor(path + " | count")
	require.NoError(t, err)
	require.NotNil(t, response)
	require.NotNil(t, response.Response)

	assert.Equal(t, 1, calls)
	assert.True(t, renders, "the SSH exec channel renders the residual chain, so the dispatch must say so")
	want, err := json.Marshal(rows)
	require.NoError(t, err)
	got, err := json.Marshal(response.Response.Data)
	require.NoError(t, err)
	assert.JSONEq(t, string(want), string(got), "the residual | count is the channel's to apply, never the dispatcher's")
}

// TestSSHStandaloneExecutorRendersResidualChain asks the same of the
// standalone SSH server's exec executor (standaloneSSHExecutorFactory), the
// one sshBuildStandaloneImpl installs: it must send the line as typed, under
// the session's identity, as a caller that renders the residual chain.
//
// Method: the factory over a dispatcher that records the caller and the line.
//
// MUTATION: drop RendersResidualChain from the factory's CallerIdentity, or
// send a line with the chain cut off; either assertion fails.
func TestSSHStandaloneExecutorRendersResidualChain(t *testing.T) {
	const line = "show fixture ssh-standalone rows | count"
	var seen []plugin.CallerIdentity
	var sent []string
	dispatch := plugin.CommandDispatcher(func(_ context.Context, caller plugin.CallerIdentity, input string) (*plugin.Response, error) {
		seen = append(seen, caller)
		sent = append(sent, input)
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"ok": true}), nil
	})
	policy := sshRunAllowPolicy()
	executor := standaloneSSHExecutorFactory(dispatch)(t.Context(), "operator", "192.0.2.47:2200", policy)
	_, err := executor(line)
	require.NoError(t, err)

	require.Len(t, seen, 1)
	assert.True(t, seen[0].RendersResidualChain, "the SSH exec channel renders the residual chain, so the dispatch must say so")
	assert.Equal(t, "operator", seen[0].Username)
	assert.Equal(t, "192.0.2.47:2200", seen[0].RemoteAddr)
	assert.Equal(t, policy, seen[0].Authorizer)
	assert.Equal(t, []string{line}, sent, "the typed line, pipe chain included")
}

// sshRunAllowPolicy is a session policy that lets "operator" run every
// read-only command, so the tests above judge the dispatch statement alone.
func sshRunAllowPolicy() plugin.Authorizer {
	policy := authz.NewStore()
	policy.AddProfile(authz.Profile{Name: "exec-operator", Run: authz.Section{Default: authz.Allow}})
	policy.AssignProfiles("operator", []string{"exec-operator"})
	return authz.StoreAuthorizer{Store: policy}
}
