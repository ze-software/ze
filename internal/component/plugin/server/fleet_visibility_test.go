// Design: docs/architecture/fleet-config.md -- request-bound local device-scope projection
// Related: fleet_visibility.go -- FleetDeviceScope, ResolveFleetDeviceScope, AuthorizeFleetDevice

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
)

type unsupportedScopeAuthorizer struct {
	calls int
}

func (u *unsupportedScopeAuthorizer) Authorize(string, string, string, bool) bool {
	u.calls++
	return true
}

func fleetVisibilityStoreForTest() *authz.Store {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "fleet-west",
		Run: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Allow, Match: "^show fleet device name edge-.*$", Regex: true},
			},
		},
	})
	store.AddProfile(authz.Profile{
		Name: "fleet-core",
		Run: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Allow, Match: "^show fleet device name core-.*$", Regex: true},
			},
		},
	})
	store.AssignProfiles("west-op", []string{"fleet-west"})
	store.AssignProfiles("core-op", []string{"fleet-core"})
	return store
}

// VALIDATES: local RBAC mixed allow/deny rules project fleet device visibility
// accurately and reject empty device names.
// PREVENTS: unauthorized devices leaking into fleet sample queries.
func TestResolveFleetDeviceScopeMixedAllowDeny(t *testing.T) {
	srv, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	srv.Dispatcher().SetAuthorizer(authz.StoreAuthorizer{Store: fleetVisibilityStoreForTest()})

	ctx := &CommandContext{
		Server:   srv,
		Username: "west-op",
	}
	scope := ResolveFleetDeviceScope(ctx)
	assert.True(t, scope.AllowsDevice("edge-01"))
	assert.False(t, scope.AllowsDevice("core-01"))
	assert.False(t, scope.AllowsDevice(""))
	assert.True(t, AuthorizeFleetDevice(ctx, "edge-02"))
	assert.False(t, AuthorizeFleetDevice(ctx, "core-02"))

	filtered := scope.FilterDeviceNames([]string{"edge-01", "core-01", "", "edge-02", "core-02"})
	assert.Equal(t, []string{"edge-01", "edge-02"}, filtered)
}

// VALIDATES: explicit no-RBAC (no request or dispatcher authorizer) allows all
// non-empty device names while still denying empty names.
// PREVENTS: policy-free hubs hiding every managed device.
func TestResolveFleetDeviceScopeNoRBACAllowsAllNonEmpty(t *testing.T) {
	srv, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)

	ctx := &CommandContext{Server: srv, Username: "operator"}
	scope := ResolveFleetDeviceScope(ctx)
	assert.True(t, scope.AllowsDevice("edge-01"))
	assert.True(t, scope.AllowsDevice("core-01"))
	assert.False(t, scope.AllowsDevice(""))
	assert.Equal(t, []string{"edge-01", "core-01"}, scope.FilterDeviceNames([]string{"edge-01", "", "core-01"}))
}

// VALIDATES: nil CommandContext and zero FleetDeviceScope fail closed.
// PREVENTS: uninitialized scope structs or missing context allowing devices.
func TestResolveFleetDeviceScopeNilAndZeroFailClosed(t *testing.T) {
	var zero FleetDeviceScope
	assert.False(t, zero.AllowsDevice("edge-01"))
	assert.Nil(t, zero.FilterDeviceNames([]string{"edge-01"}))

	nilScope := ResolveFleetDeviceScope(nil)
	assert.False(t, nilScope.AllowsDevice("edge-01"))
	assert.Nil(t, nilScope.FilterDeviceNames([]string{"edge-01"}))
	assert.False(t, AuthorizeFleetDevice(nil, "edge-01"))
}

// VALIDATES: a custom plugin.Authorizer that does not implement
// aaa.LocalScopeAuthorizer fails closed and never calls Authorize per device.
// PREVENTS: unsupported scope providers triggering remote per-device checks or
// failing open.
func TestResolveFleetDeviceScopeUnsupportedAuthorizerFailsClosed(t *testing.T) {
	unsupported := &unsupportedScopeAuthorizer{}
	ctx := &CommandContext{
		Username:   "west-op",
		Authorizer: unsupported,
	}
	scope := ResolveFleetDeviceScope(ctx)
	assert.False(t, scope.AllowsDevice("edge-01"))
	assert.Nil(t, scope.FilterDeviceNames([]string{"edge-01", "core-01"}))
	assert.Equal(t, 0, unsupported.calls, "unsupported provider must never invoke Authorize")
}

// VALIDATES: ctx.Authorizer takes precedence over Dispatcher.authorizer.
// PREVENTS: a request-carried session policy being ignored in favor of the
// dispatcher's fallback authorizer.
func TestResolveFleetDeviceScopeRequestAuthorizerOverridesDispatcher(t *testing.T) {
	store := fleetVisibilityStoreForTest()
	srv, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	srv.Dispatcher().SetAuthorizer(authz.StoreAuthorizer{Store: store})

	ctx := &CommandContext{
		Server:     srv,
		Username:   "west-op",
		Authorizer: authz.StoreAuthorizer{Store: store}.BindProfiles([]string{"fleet-core"}),
	}
	scope := ResolveFleetDeviceScope(ctx)
	assert.False(t, scope.AllowsDevice("edge-01"))
	assert.True(t, scope.AllowsDevice("core-01"))
}
