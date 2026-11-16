// Design: docs/architecture/api/commands.md -- shared daemon lifecycle commands.
package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestDaemonRestartCommandRegistration proves that every command adapter can
// discover daemon restart from the same real YANG and builtin registrations.
// Method: load the daemon's command tree and inspect the canonical dispatch
// owner, with shutdown as the existing lifecycle registration control.
func TestDaemonRestartCommandRegistration(t *testing.T) {
	loader, err := yang.DefaultLoader()
	require.NoError(t, err)
	paths := yang.WireMethodToPaths(loader)
	require.Contains(t, paths["ze-plugin:system-daemon-shutdown"], "request shutdown")
	require.Contains(t, paths["ze-plugin:system-daemon-restart"], "request restart")
	d := restartLineDispatcher(t)
	registered := d.Lookup("request restart")
	require.NotNil(t, registered, "daemon restart must have a shared builtin handler")
	assert.Equal(t, "ze-plugin:system-daemon-restart", registered.Owner)
	assert.False(t, registered.ReadOnly, "restarting the daemon changes its state")
	require.NotEmpty(t, registered.ShortHelp)
	require.NotEmpty(t, registered.Description)
}

// TestDaemonRestartDispatchRefusalIsAccounted proves that an authorized request
// reaches the lifecycle handler even on a reactorless server with no action
// configured. Method: dispatch through the real registry with an actual local
// policy and capture the START and STOP records without stopping any process.
func TestDaemonRestartDispatchRefusalIsAccounted(t *testing.T) {
	d := restartLineDispatcher(t)
	policy := daemonRestartPolicy()
	d.SetAuthorizer(authz.StoreAuthorizer{Store: policy})
	accountant := &fakeAccountant{}
	d.SetAccountingHook(accountant)
	s := &Server{dispatcher: d}
	ctx := &CommandContext{
		Server: s, RequestContext: context.Background(),
		Username: "admin", RemoteAddr: "192.0.2.10:2200",
	}
	resp, err := d.Dispatch(ctx, "request restart")
	require.Error(t, err, "an unwired lifecycle action must be refused")
	require.NotErrorIs(t, err, ErrUnknownCommand, "restart must reach its registered owner")
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "restart")
	assert.Contains(t, resp.Error, "not available")
	assert.Equal(t, []string{"request restart"}, accountant.starts)
	assert.Equal(t, []string{"request restart"}, accountant.stops)
	assert.Equal(t, []string{"admin"}, accountant.startUsers)
	assert.Equal(t, []string{"admin"}, accountant.stopUsers)
	assert.Equal(t, []string{ctx.RemoteAddr}, accountant.startRemotes)
	assert.Equal(t, []string{ctx.RemoteAddr}, accountant.stopRemotes)
}

// TestDaemonRestartAuthorizationDeniesReader proves that the real local policy
// denies a registered lifecycle mutation while allowing the same user's read.
// Method: dispatch an allowed version query, then restart through the shared
// dispatcher and check the permission classification and accounting identity.
func TestDaemonRestartAuthorizationDeniesReader(t *testing.T) {
	d := restartLineDispatcher(t)
	require.NotNil(t, d.Lookup("request restart"), "the denial must judge a registered command")
	d.SetAuthorizer(authz.StoreAuthorizer{Store: daemonRestartPolicy()})
	accountant := &fakeAccountant{}
	d.SetAccountingHook(accountant)
	s := &Server{dispatcher: d}
	ctx := &CommandContext{
		Server: s, RequestContext: context.Background(),
		Username: "reader", RemoteAddr: "192.0.2.11:2200",
	}
	read, err := d.Dispatch(ctx, "show version")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, read.Status)
	resp, err := d.Dispatch(ctx, "request restart")
	require.ErrorIs(t, err, ErrUnauthorized)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Equal(t, []string{"show version", "request restart"}, accountant.starts)
	assert.Equal(t, accountant.starts, accountant.stops)
	assert.Equal(t, []string{"reader", "reader"}, accountant.startUsers)
	assert.Equal(t, accountant.startUsers, accountant.stopUsers)
}

// daemonRestartPolicy returns a separate real policy store for each test;
// lifecycle tests never install a process-wide policy or a real stop action.
func daemonRestartPolicy() *authz.Store {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "admin", Run: authz.Section{Default: authz.Allow},
		Edit: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{{Number: 10, Action: authz.Allow, Match: "request restart"}},
		},
	})
	store.AddProfile(authz.Profile{
		Name: "reader", Run: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{{Number: 10, Action: authz.Allow, Match: "show version"}},
		},
		Edit: authz.Section{Default: authz.Deny},
	})
	store.AssignProfiles("admin", []string{"admin"})
	store.AssignProfiles("reader", []string{"reader"})
	return store
}
