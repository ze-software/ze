// Design: docs/architecture/aaa-tacacs.md -- local device-scope projection for TACACS+ sessions
// Related: authorizer.go -- tacacsAuthorizer.AuthorizeLocalScope

package tacacs

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func fleetWestStoreForTest() *authz.Store {
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
	store.AssignProfiles("local-west", []string{"fleet-west"})
	return store
}

// VALIDATES: a TACACS+-authenticated reader authorizes the fleet history
// command remotely once and projects 4,096 declared devices strictly locally
// through its bound local profile with zero additional remote requests.
// PREVENTS: per-device TACACS+ network fanout during fleet sample queries.
func TestTacacsAuthorizerFleetScopeSingleRemoteCallFor4096Devices(t *testing.T) {
	const (
		totalDevices   = 4096
		historyCommand = "show fleet sample latest"
		profileName    = "fleet-west"
	)

	key := []byte("secret")
	var remoteCalls atomic.Int64
	srv := newTestServer(t, key, func(hdr PacketHeader, body []byte) []byte {
		remoteCalls.Add(1)
		return authorReply(AuthorStatusPassAdd)(hdr, body)
	})
	t.Cleanup(srv.close)

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})
	t.Cleanup(client.Close)

	base := newTacacsAuthorizer(client, authz.StoreAuthorizer{Store: fleetWestStoreForTest()})
	sessionAuth := aaa.AuthorizerForResult(base, aaa.AuthResult{
		Authenticated: true,
		Source:        backendName,
		Profiles:      []string{profileName},
	})

	devices := make([]string, 0, totalDevices)
	for i := range totalDevices / 2 {
		devices = append(devices, fmt.Sprintf("edge-%04d", i), fmt.Sprintf("core-%04d", i))
	}

	d := pluginserver.NewDispatcher()
	var visible []string
	require.NoError(t, d.RegisterWithOptions(historyCommand, func(ctx *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		scope := pluginserver.ResolveFleetDeviceScope(ctx)
		visible = scope.FilterDeviceNames(devices)
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}, historyCommand, pluginserver.RegisterOptions{ReadOnly: true}))

	resp, err := d.Dispatch(&pluginserver.CommandContext{
		Username:   "noc-west",
		RemoteAddr: "198.51.100.10:22",
		Authorizer: sessionAuth,
	}, historyCommand)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	assert.Equal(t, int64(1), remoteCalls.Load(),
		"expected exactly one remote TACACS+ authorization request for the command and zero for 4,096 device projections")
	require.Len(t, visible, totalDevices/2)
	assert.Equal(t, "edge-0000", visible[0])
	assert.Equal(t, fmt.Sprintf("edge-%04d", totalDevices/2-1), visible[len(visible)-1])
}

// VALIDATES: TACACS+ local scope fails closed with zero remote requests when
// no local profile is bound, when bound profiles are empty or unknown, when
// the local store is missing, or when the local fallback is unsupported.
// PREVENTS: an unmapped TACACS+ login bypassing device visibility or leaking
// per-device remote TACACS+ requests.
func TestTacacsAuthorizerLocalScopeFailClosedCases(t *testing.T) {
	key := []byte("secret")
	var remoteCalls atomic.Int64
	srv := newTestServer(t, key, func(hdr PacketHeader, body []byte) []byte {
		remoteCalls.Add(1)
		return authorReply(AuthorStatusPassAdd)(hdr, body)
	})
	t.Cleanup(srv.close)

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})
	t.Cleanup(client.Close)

	store := fleetWestStoreForTest()
	edgeCmd, edgeArgs := aaa.FleetDeviceReadTokens("edge-01")
	coreCmd, coreArgs := aaa.FleetDeviceReadTokens("core-01")

	t.Run("unbound tacacs authorizer evaluates local, internal-RPC, and shared-API callers locally", func(t *testing.T) {
		auth := newTacacsAuthorizer(client, authz.StoreAuthorizer{Store: store})
		assert.True(t, auth.AuthorizeLocalScope("local-west", edgeCmd, edgeArgs, true))
		assert.False(t, auth.AuthorizeLocalScope("local-west", coreCmd, coreArgs, true))
		assert.False(t, auth.AuthorizeLocalScope("unassigned-user", edgeCmd, edgeArgs, true))
		assert.True(t, auth.AuthorizeLocalScope(aaa.ReservedInternalRPCIdentity, edgeCmd, edgeArgs, true))
		assert.True(t, auth.AuthorizeLocalScope(aaa.ReservedSharedAPIUsername, edgeCmd, edgeArgs, true))
	})

	t.Run("empty profile binding denies unmapped remote sessions", func(t *testing.T) {
		auth := newTacacsAuthorizer(client, authz.StoreAuthorizer{Store: store})
		for _, empty := range [][]string{nil, {}} {
			bound := auth.BindProfiles(empty)
			allowed, supported := aaa.AuthorizeLocalScope(bound, "local-west", edgeCmd, edgeArgs, true)
			assert.True(t, supported)
			assert.False(t, allowed)
		}
	})

	t.Run("unknown remote profile denies", func(t *testing.T) {
		auth := newTacacsAuthorizer(client, authz.StoreAuthorizer{Store: store})
		bound := auth.BindProfiles([]string{"unknown-profile"})
		allowed, supported := aaa.AuthorizeLocalScope(bound, "noc-west", edgeCmd, edgeArgs, true)
		assert.True(t, supported)
		assert.False(t, allowed)
	})

	t.Run("missing local store denies", func(t *testing.T) {
		auth := newTacacsAuthorizer(client, authz.StoreAuthorizer{Store: nil})
		bound := auth.BindProfiles([]string{"fleet-west"})
		allowed, supported := aaa.AuthorizeLocalScope(bound, "noc-west", edgeCmd, edgeArgs, true)
		assert.True(t, supported)
		assert.False(t, allowed)
	})

	t.Run("unsupported local fallback denies", func(t *testing.T) {
		auth := newTacacsAuthorizer(client, &fakeLocalAuthz{allow: true})
		bound := auth.BindProfiles([]string{"fleet-west"})
		allowed, supported := aaa.AuthorizeLocalScope(bound, "noc-west", edgeCmd, edgeArgs, true)
		assert.True(t, supported)
		assert.False(t, allowed)
	})

	t.Run("BindLocalFallback preserves bound profile state", func(t *testing.T) {
		auth := newTacacsAuthorizer(client, &fakeLocalAuthz{allow: false})
		bound := auth.BindProfiles([]string{"fleet-west"})
		boundAuth, ok := bound.(*tacacsAuthorizer)
		require.True(t, ok)
		rebound := boundAuth.BindLocalFallback(authz.StoreAuthorizer{Store: store}.BindProfiles([]string{"fleet-west"}))

		allowed, supported := aaa.AuthorizeLocalScope(rebound, "noc-west", edgeCmd, edgeArgs, true)
		assert.True(t, supported)
		assert.True(t, allowed)

		allowed, supported = aaa.AuthorizeLocalScope(rebound, "noc-west", coreCmd, coreArgs, true)
		assert.True(t, supported)
		assert.False(t, allowed)

		emptyBound := auth.BindProfiles(nil)
		emptyBoundAuth, ok := emptyBound.(*tacacsAuthorizer)
		require.True(t, ok)
		emptyRebound := emptyBoundAuth.BindLocalFallback(authz.StoreAuthorizer{Store: store})
		allowed, supported = aaa.AuthorizeLocalScope(emptyRebound, "local-west", edgeCmd, edgeArgs, true)
		assert.True(t, supported)
		assert.False(t, allowed, "BindLocalFallback must preserve profilesBound/hasProfiles fail-closed state")
	})

	assert.Equal(t, int64(0), remoteCalls.Load(),
		"local scope projection must never send any remote TACACS+ packet")
}
