// Design: docs/architecture/api/commands.md -- SSH aliases use shared lifecycle admission.
//go:build ze_ssh

package hub

import (
	"context"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zessh "github.com/ze-software/ze/internal/component/ssh"
)

// TestSessionFactoryLifecycleUsesCallerDispatcher drives both production model
// constructors and their actual SSH executor. The confirmation must precede
// canonical admission; a reader refusal cannot call an unscoped stop closure.
func TestSessionFactoryLifecycleUsesCallerDispatcher(t *testing.T) {
	for _, editor := range []bool{false, true} {
		for _, alias := range []string{"stop", "restart"} {
			for _, user := range []string{"reader", "admin"} {
				name := user + "/" + alias
				if editor {
					name += "/editor"
				}
				t.Run(name, func(t *testing.T) {
					server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
					require.NoError(t, err)
					t.Cleanup(server.Stop)
					accepted := 0
					server.SetShutdownFunc(func() { accepted++ })
					server.SetRestartFunc(func() { accepted++ })
					accountant := &infraBootAccountant{name: "ssh-lifecycle"}
					server.Dispatcher().SetAccountingHook(accountant)
					sshServer, err := zessh.NewServer(zessh.Config{
						HostKeyPath: filepath.Join(t.TempDir(), "host-key"),
					})
					require.NoError(t, err)
					params := infra.HookParams{
						APIServer:            func() *pluginserver.Server { return server },
						FormatResponseData:   func(any) string { return "accepted" },
						CollectLoginWarnings: func(plugin.ReactorIntrospector) []infra.LoginWarning { return nil },
					}
					if editor {
						params.Store = newTestStore(t)
						params.ConfigPath = "router.conf"
						require.NoError(t, params.Store.WriteFile(params.ConfigPath, nil, 0o600))
					}
					sshWireImpl(sshServer, &sshWireInputs{
						Reactor: &authWiringReactor{dispatcher: server.Dispatcher()},
						Params:  params,
					})
					policy := &lifecycleSessionPolicy{policy: authz.StoreAuthorizer{
						Store: daemonRestartSurfacePolicy(),
					}}
					created, err := buildSessionModelFactory(sshServer, params, nil, nil)(
						t.Context(), user, "192.0.2.45:2200", policy, zessh.SessionRequest{})
					require.NoError(t, err)
					model, ok := created.(cli.Model)
					require.True(t, ok)
					t.Cleanup(func() { require.NoError(t, model.Close()) })
					model.SetInput(alias)
					model = lifecycleModelKey(t, &model, tea.KeyPressMsg{Code: tea.KeyEnter})
					assert.Contains(t, model.View().Content, "Continue? [y/N]")
					assert.Empty(t, accountant.starts, "confirmation must come before admission")
					assert.Zero(t, accepted)
					model = lifecycleModelKey(t, &model, tea.KeyPressMsg{Code: 'y', Text: "y"})
					canonical := "request restart"
					if alias == "stop" {
						canonical = "request shutdown"
					}
					assert.Equal(t, []string{canonical}, accountant.starts)
					assert.Equal(t, accountant.starts, accountant.stops)
					assert.Equal(t, user, policy.username)
					assert.Equal(t, "192.0.2.45:2200", policy.remote)
					assert.Equal(t, canonical, policy.command)
					if user == "reader" {
						assert.Zero(t, accepted)
						assert.Contains(t, model.View().Content, plugin.UnauthorizedMessage)
					} else {
						assert.Equal(t, 1, accepted)
					}
				})
			}
		}
	}
}

type lifecycleSessionPolicy struct {
	policy   plugin.Authorizer
	username string
	remote   string
	command  string
}

func (p *lifecycleSessionPolicy) Authorize(username, remote, command string, readOnly bool) bool {
	p.username, p.remote, p.command = username, remote, command
	return p.policy.Authorize(username, remote, command, readOnly)
}

// lifecycleModelKey processes the bounded result/completion chain a single
// lifecycle key creates. No model Init, timer or background worker is started.
func lifecycleModelKey(t *testing.T, model *cli.Model, key tea.KeyPressMsg) cli.Model {
	t.Helper()
	updated, next := model.Update(key)
	for steps := 0; next != nil; steps++ {
		require.Less(t, steps, 8, "lifecycle key produced an unbounded command chain")
		updated, next = updated.Update(next())
	}
	result, ok := updated.(cli.Model)
	require.True(t, ok)
	return result
}

// TestSessionFactoryLifecycleCancellationUsesSessionContext cancels the
// authenticated session after confirmation but before restart admission.
func TestSessionFactoryLifecycleCancellationUsesSessionContext(t *testing.T) {
	server, err := zessh.NewServer(zessh.Config{HostKeyPath: filepath.Join(t.TempDir(), "host-key")})
	require.NoError(t, err)
	dispatched := 0
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	server.SetExecutorFactory(func(session context.Context, _, _ string, _ plugin.Authorizer) zessh.CommandExecutor {
		return func(input string) (*plugin.RenderedResponse, error) {
			dispatched++
			assert.Equal(t, "request restart", input)
			return &plugin.RenderedResponse{}, session.Err()
		}
	})
	created, err := buildSessionModelFactory(server, infra.HookParams{}, nil, nil)(ctx,
		"admin", "192.0.2.45:2200", authz.StoreAuthorizer{Store: daemonRestartSurfacePolicy()}, zessh.SessionRequest{})
	require.NoError(t, err)
	model, ok := created.(cli.Model)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, model.Close()) })
	model.SetInput("restart")
	model = lifecycleModelKey(t, &model, tea.KeyPressMsg{Code: tea.KeyEnter})
	cancel()
	model = lifecycleModelKey(t, &model, tea.KeyPressMsg{Code: 'y', Text: "y"})
	assert.Equal(t, 1, dispatched)
	assert.Contains(t, model.View().Content, context.Canceled.Error())
}

// TestSSHSessionLifecycleRefusesMissingPolicy proves remote construction never
// substitutes the daemon's permissive default for absent authenticated policy.
// A real session policy is the positive control on the same executor factory.
func TestSSHSessionLifecycleRefusesMissingPolicy(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	policy := authz.StoreAuthorizer{Store: daemonRestartSurfacePolicy()}
	server.Dispatcher().SetAuthorizer(policy)
	accepted := 0
	server.SetRestartFunc(func() { accepted++ })
	sshServer, err := zessh.NewServer(zessh.Config{HostKeyPath: filepath.Join(t.TempDir(), "host-key")})
	require.NoError(t, err)
	sshWireImpl(sshServer, &sshWireInputs{
		Reactor: &authWiringReactor{dispatcher: server.Dispatcher()},
		Params: infra.HookParams{
			APIServer:            func() *pluginserver.Server { return server },
			FormatResponseData:   func(any) string { return "accepted" },
			CollectLoginWarnings: func(plugin.ReactorIntrospector) []infra.LoginWarning { return nil },
		},
	})
	executor := sshServer.ExecutorForUser(t.Context(), "admin", "192.0.2.45:2200", nil)
	require.NotNil(t, executor)
	response, err := executor("request restart")
	assert.ErrorIs(t, err, pluginserver.ErrUnauthorized)
	if response != nil {
		response.TransportComplete()
	}
	assert.Zero(t, accepted)
	response, err = sshServer.ExecutorForUser(t.Context(), "admin", "192.0.2.45:2200", policy)("request restart")
	require.NoError(t, err)
	require.NotNil(t, response)
	response.TransportComplete()
	assert.Equal(t, 1, accepted)
}
