// Design: docs/architecture/hub-architecture.md -- daemon restart composition.
package hub

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type daemonRestartReactor struct {
	authWiringReactor
	stops int
	cause rpc.DrainInput
}

func (r *daemonRestartReactor) StopForRestart() {
	r.stops++
	r.cause = stopDrainInput()
}

// TestDaemonRestartInfraUsesExistingGracefulStop proves the real infrastructure
// post-start hook binds the shared command to its existing restart owner.
// Method: run infraSetup with a counting reactor and real local policy, then
// dispatch through serverDispatcher and inspect the stop cause at completion.
// No protocol session, listener or operating-system signal is created.
func TestDaemonRestartInfraUsesExistingGracefulStop(t *testing.T) {
	resetAAABundleForTest(t)
	stopRestartCause.Store(nil)
	rebootRequested.Store(false)
	t.Cleanup(func() {
		stopRestartCause.Store(nil)
		rebootRequested.Store(false)
	})
	s, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(s.Stop)
	policy := daemonRestartSurfacePolicy()
	accountant := &infraBootAccountant{name: "restart"}
	swapAAABundle(&aaa.Bundle{
		Authorizer: authz.StoreAuthorizer{Store: policy}, Accountant: accountant,
	}, nil)
	r := &daemonRestartReactor{authWiringReactor: authWiringReactor{dispatcher: s.Dispatcher()}}
	infraSetup(infra.HookParams{
		Reactor: r, APIServer: func() *pluginserver.Server { return s },
	}, nil, &configMutationOwner{}, nil)
	require.NotNil(t, r.postStart)
	r.postStart()
	resp, err := serverDispatcher(s, audit.Web)(context.Background(),
		plugin.CallerIdentity{Username: "admin"}, "request restart")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	assert.Zero(t, r.stops, "the accepted response still belongs to its transport")
	assert.Equal(t, []string{"request restart"}, accountant.starts)
	assert.Equal(t, accountant.starts, accountant.stops)
	resp.TransportComplete()
	assert.Equal(t, 1, r.stops)
	assert.Equal(t, rpc.DrainInput{
		Kind: rpc.DrainKindRestart, RestartCause: rpc.DrainRestartInPlace,
	}, r.cause, "restart cause must be visible before StopForRestart")
	assert.False(t, rebootRequested.Load(), "daemon restart must leave the host running")
}

// TestDaemonRestartSurfaceDispatchUsesCallerPolicy proves every hub command
// dispatcher carries the same real caller policy into restart admission.
// Method: for each management surface, deny a reader with a positive read
// control, then accept an administrator and complete a recording action.
// This judges the shared hub adapter; transport delivery has separate tests.
func TestDaemonRestartSurfaceDispatchUsesCallerPolicy(t *testing.T) {
	for _, surface := range []string{audit.CLI, audit.SSH, audit.Web, audit.REST, audit.GRPC, audit.MCP} {
		t.Run(surface, func(t *testing.T) {
			s, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
			require.NoError(t, err)
			t.Cleanup(s.Stop)
			policy := authz.StoreAuthorizer{Store: daemonRestartSurfacePolicy()}
			calls := 0
			s.SetRestartFunc(func() { calls++ })
			dispatch := serverDispatcher(s, surface)
			reader := plugin.CallerIdentity{Username: "reader", Authorizer: policy}
			read, err := dispatch(context.Background(), reader, "show version")
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, read.Status)
			denied, err := dispatch(context.Background(), reader, "request restart")
			require.ErrorIs(t, err, pluginserver.ErrUnauthorized)
			require.NotNil(t, denied)
			denied.TransportComplete()
			assert.Zero(t, calls)
			accepted, err := dispatch(context.Background(),
				plugin.CallerIdentity{Username: "admin", Authorizer: policy}, "request restart")
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, accepted.Status)
			assert.Zero(t, calls)
			accepted.TransportComplete()
			assert.Equal(t, 1, calls)
		})
	}
}

func daemonRestartSurfacePolicy() *authz.Store {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "admin", Run: authz.Section{Default: authz.Allow},
		Edit: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Allow, Match: "request restart"},
				{Number: 20, Action: authz.Allow, Match: "request shutdown"},
			},
		},
	})
	store.AddProfile(authz.Profile{
		Name: "reader", Run: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{{Number: 10, Action: authz.Allow, Match: "show version"}},
		}, Edit: authz.Section{Default: authz.Deny},
	})
	store.AssignProfiles("admin", []string{"admin"})
	store.AssignProfiles("reader", []string{"reader"})
	return store
}
