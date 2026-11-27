// Design: docs/architecture/fleet-config.md -- hub AAA wrapper local device-scope projection
// Related: aaa_lifecycle.go -- liveLocalAuthorizer, liveLocalProfileAuthorizer, acceptedLocalGenerationAuthorizer, liveAAABundleAuthorizer, liveAAABundleProfileAuthorizer

package hub

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const (
	fleetScopeProfileWest  = "fleet-west"
	fleetScopeDeviceEdge   = "edge-01"
	fleetScopeDeviceCore   = "core-01"
	fleetScopeUser         = "west-op"
	fleetScopeHistoryCmd   = "show fleet sample latest"
	fleetScopeRecoveryUser = "bootstrap-admin"
)

func fleetScopeStoreForTest() *authz.Store {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: fleetScopeProfileWest,
		Run: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Allow, Match: "^show fleet device name edge-.*$", Regex: true},
				{Number: 20, Action: authz.Allow, Match: fleetScopeHistoryCmd, Regex: false},
			},
		},
	})
	store.AssignProfiles(fleetScopeUser, []string{fleetScopeProfileWest})
	return store
}

// countingRemoteAuthorizer mirrors tacacsAuthorizer's local/remote boundary
// inside package hub without importing the gated tacacs package.
type countingRemoteAuthorizer struct {
	remoteCalls   *atomic.Int64
	local         aaa.Authorizer
	profilesBound bool
	hasProfiles   bool
}

func (c *countingRemoteAuthorizer) Authorize(string, string, string, bool) bool {
	c.remoteCalls.Add(1)
	return true
}

func (c *countingRemoteAuthorizer) AuthorizeCommandArgs(string, string, string, []string, string, bool) bool {
	c.remoteCalls.Add(1)
	return true
}

func (c *countingRemoteAuthorizer) BindProfiles(profiles []string) aaa.Authorizer {
	clone := *c
	clone.local = aaa.BindProfiles(c.local, profiles)
	clone.profilesBound = true
	clone.hasProfiles = len(profiles) > 0
	return &clone
}

func (c *countingRemoteAuthorizer) BindLocalFallback(local aaa.Authorizer) aaa.Authorizer {
	clone := *c
	clone.local = local
	return &clone
}

func (c *countingRemoteAuthorizer) AuthorizeLocalScope(username, command string, args []string, isReadOnly bool) bool {
	if c.profilesBound {
		if !c.hasProfiles {
			return false
		}
	}
	if c.local == nil {
		return false
	}
	allowed, supported := aaa.AuthorizeLocalScope(c.local, username, command, args, isReadOnly)
	if !supported {
		return false
	}
	return allowed
}

type unsupportedBundleAuthorizer struct {
	calls int
}

func (u *unsupportedBundleAuthorizer) Authorize(string, string, string, bool) bool {
	u.calls++
	return true
}

// VALIDATES: all six hub AAA lifecycle wrappers forward local device-scope
// projection accurately for mixed allow/deny rules.
// PREVENTS: any surface wrapper (SSH, web, REST, gRPC, CLI, MCP) stripping
// LocalScopeAuthorizer or ignoring profile bindings.
func TestHubAAAWrappersLocalScopeMixedAllowDeny(t *testing.T) {
	resetAAABundleForTest(t)
	store := fleetScopeStoreForTest()
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, store, nil, ""))
	liveGen := acceptedLocalIdentity.Load().authzGeneration
	require.NotZero(t, liveGen)
	swapAAABundle(&aaa.Bundle{Authorizer: liveLocalAuthorizer{}}, nil)

	wrappers := map[string]aaa.Authorizer{
		"liveLocalAuthorizer":                    liveLocalAuthorizer{},
		"liveLocalProfileAuthorizer":             liveLocalProfileAuthorizer{profiles: []string{fleetScopeProfileWest}},
		"acceptedLocalGenerationAuthorizer":      acceptedLocalGenerationAuthorizer{store: store, generation: liveGen},
		"acceptedLocalGenerationProfileFallback": acceptedLocalGenerationProfileFallback{store: store, generation: liveGen, profiles: []string{fleetScopeProfileWest}},
		"liveAAABundleAuthorizer":                liveAAABundleAuthorizer{},
		"liveAAABundleProfileAuthorizer":         liveAAABundleProfileAuthorizer{profiles: []string{fleetScopeProfileWest}},
	}

	for name, wrapper := range wrappers {
		t.Run(name, func(t *testing.T) {
			ctx := &pluginserver.CommandContext{Username: fleetScopeUser, Authorizer: wrapper}
			scope := pluginserver.ResolveFleetDeviceScope(ctx)
			assert.True(t, scope.AllowsDevice(fleetScopeDeviceEdge), "%s must allow edge-*", name)
			assert.False(t, scope.AllowsDevice(fleetScopeDeviceCore), "%s must deny core-*", name)
		})
	}
}

// VALIDATES: a profile-bound remote session through liveAAABundleProfileAuthorizer
// authorizes the fleet history command remotely once and projects 4,096 devices
// strictly locally with zero additional remote requests.
// PREVENTS: hub bundle wrappers triggering remote authorization per device.
func TestLiveAAABundleProfileAuthorizerProjects4096DevicesWithOneRemoteCall(t *testing.T) {
	const totalDevices = 4096
	resetAAABundleForTest(t)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, fleetScopeStoreForTest(), nil, ""))

	var remoteCalls atomic.Int64
	remote := &countingRemoteAuthorizer{
		remoteCalls: &remoteCalls,
		local:       liveLocalAuthorizer{},
	}
	swapAAABundle(&aaa.Bundle{Authorizer: remote}, nil)

	sessionAuth := aaa.AuthorizerForResult(liveAAABundleAuthorizer{}, aaa.AuthResult{
		Authenticated: true,
		Source:        "tacacs",
		Profiles:      []string{fleetScopeProfileWest},
	})

	devices := make([]string, 0, totalDevices)
	for i := range totalDevices / 2 {
		devices = append(devices, fmt.Sprintf("edge-%04d", i), fmt.Sprintf("core-%04d", i))
	}

	d := pluginserver.NewDispatcher()
	var visible []string
	require.NoError(t, d.RegisterWithOptions(fleetScopeHistoryCmd, func(ctx *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		visible = pluginserver.ResolveFleetDeviceScope(ctx).FilterDeviceNames(devices)
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}, fleetScopeHistoryCmd, pluginserver.RegisterOptions{ReadOnly: true}))

	resp, err := d.Dispatch(&pluginserver.CommandContext{
		Username:   "remote-west",
		RemoteAddr: "198.51.100.20:22",
		Authorizer: sessionAuth,
	}, fleetScopeHistoryCmd)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	assert.Equal(t, int64(1), remoteCalls.Load(),
		"expected 1 remote authorization call for command dispatch and 0 for 4,096 device projections")
	assert.Len(t, visible, totalDevices/2)
	assert.Equal(t, "edge-0000", visible[0])
}

// VALIDATES: unmapped remote sessions, unknown remote profiles, and remote
// sessions on a hub without local RBAC fail closed with zero remote calls.
// PREVENTS: a missing remote profile mapping being treated as explicit no-RBAC.
func TestHubAAAWrappersRemoteMappingFailClosed(t *testing.T) {
	resetAAABundleForTest(t)
	var remoteCalls atomic.Int64
	remote := &countingRemoteAuthorizer{
		remoteCalls: &remoteCalls,
		local:       liveLocalAuthorizer{},
	}
	swapAAABundle(&aaa.Bundle{Authorizer: remote}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, fleetScopeStoreForTest(), nil, ""))

	t.Run("unbound remote bundle evaluates local, internal-RPC, and shared-API callers locally", func(t *testing.T) {
		ctxLocal := &pluginserver.CommandContext{Username: fleetScopeUser, Authorizer: liveAAABundleAuthorizer{}}
		assert.True(t, pluginserver.AuthorizeFleetDevice(ctxLocal, fleetScopeDeviceEdge))
		assert.False(t, pluginserver.AuthorizeFleetDevice(ctxLocal, fleetScopeDeviceCore))

		ctxUnassigned := &pluginserver.CommandContext{Username: "remote-user", Authorizer: liveAAABundleAuthorizer{}}
		assert.False(t, pluginserver.AuthorizeFleetDevice(ctxUnassigned, fleetScopeDeviceEdge))

		ctxInternal := &pluginserver.CommandContext{Username: aaa.ReservedInternalRPCIdentity, Authorizer: liveAAABundleAuthorizer{}}
		assert.True(t, pluginserver.AuthorizeFleetDevice(ctxInternal, fleetScopeDeviceEdge))

		ctxSharedAPI := &pluginserver.CommandContext{Username: aaa.ReservedSharedAPIUsername, Authorizer: liveAAABundleAuthorizer{}}
		assert.True(t, pluginserver.AuthorizeFleetDevice(ctxSharedAPI, fleetScopeDeviceEdge))
	})

	t.Run("empty remote profile list denies", func(t *testing.T) {
		for _, empty := range [][]string{nil, {}} {
			bound := aaa.BindProfiles(liveAAABundleAuthorizer{}, empty)
			ctx := &pluginserver.CommandContext{Username: fleetScopeUser, Authorizer: bound}
			assert.False(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceEdge))
		}
	})

	t.Run("unknown remote profile denies", func(t *testing.T) {
		bound := aaa.BindProfiles(liveAAABundleAuthorizer{}, []string{"unknown-profile"})
		ctx := &pluginserver.CommandContext{Username: "remote-user", Authorizer: bound}
		assert.False(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceEdge))
	})

	t.Run("missing local store denies remote profile even when bundle is live", func(t *testing.T) {
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, nil, ""))
		bound := aaa.BindProfiles(liveAAABundleAuthorizer{}, []string{fleetScopeProfileWest})
		ctx := &pluginserver.CommandContext{Username: "remote-user", Authorizer: bound}
		assert.False(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceEdge))
	})

	assert.Equal(t, int64(0), remoteCalls.Load(), "local scope checks must never make remote calls")
}

// VALIDATES: retired or absent AAA bundles, retired local identities, and
// authorization generation changes invalidate all positive fleet device scope.
// PREVENTS: stale sessions or retired bundles retaining positive device scope.
func TestHubAAAWrappersRetiredAndGenerationChangeFailClosed(t *testing.T) {
	resetAAABundleForTest(t)

	t.Run("absent bundle denies bundle wrappers", func(t *testing.T) {
		for name, wrapper := range map[string]aaa.Authorizer{
			"liveAAABundleAuthorizer":           liveAAABundleAuthorizer{},
			"liveAAABundleProfileAuthorizer":    liveAAABundleProfileAuthorizer{profiles: []string{fleetScopeProfileWest}},
			"acceptedLocalGenerationAuthorizer": acceptedLocalGenerationAuthorizer{store: fleetScopeStoreForTest()},
		} {
			ctx := &pluginserver.CommandContext{Username: fleetScopeUser, Authorizer: wrapper}
			assert.False(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceEdge),
				"%s must deny when AAA bundle is nil", name)
		}
	})

	t.Run("absent local identity denies local wrappers", func(t *testing.T) {
		ctxLocal := &pluginserver.CommandContext{Username: fleetScopeUser, Authorizer: liveLocalAuthorizer{}}
		assert.False(t, pluginserver.AuthorizeFleetDevice(ctxLocal, fleetScopeDeviceEdge))

		ctxProfile := &pluginserver.CommandContext{
			Username:   fleetScopeUser,
			Authorizer: liveLocalProfileAuthorizer{profiles: []string{fleetScopeProfileWest}},
		}
		assert.False(t, pluginserver.AuthorizeFleetDevice(ctxProfile, fleetScopeDeviceEdge))
	})

	t.Run("unstamped generation zero fails closed on generation wrappers", func(t *testing.T) {
		resetAAABundleForTest(t)
		store := fleetScopeStoreForTest()
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, store, nil, ""))
		swapAAABundle(&aaa.Bundle{Authorizer: liveLocalAuthorizer{}}, nil)

		zeroGen := &pluginserver.CommandContext{
			Username:   fleetScopeUser,
			Authorizer: acceptedLocalGenerationAuthorizer{store: store, generation: 0},
		}
		assert.False(t, pluginserver.AuthorizeFleetDevice(zeroGen, fleetScopeDeviceEdge),
			"acceptedLocalGenerationAuthorizer with generation 0 must fail closed")

		zeroFallback := &pluginserver.CommandContext{
			Username: fleetScopeUser,
			Authorizer: acceptedLocalGenerationProfileFallback{
				store:      store,
				generation: 0,
				profiles:   []string{fleetScopeProfileWest},
			},
		}
		assert.False(t, pluginserver.AuthorizeFleetDevice(zeroFallback, fleetScopeDeviceEdge),
			"acceptedLocalGenerationProfileFallback with generation 0 must fail closed")
	})

	t.Run("API acceptedLocalGenerationAuthorizer denies after generation change", func(t *testing.T) {
		resetAAABundleForTest(t)
		swapAAABundle(&aaa.Bundle{Authorizer: liveLocalAuthorizer{}}, nil)
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, fleetScopeStoreForTest(), nil, "tok-1"))

		apiAuth := liveAcceptedAPIAuthentication()
		caller, ok := apiAuth.Authenticate("Bearer tok-1", "rest")
		require.True(t, ok)

		ctx := &pluginserver.CommandContext{Username: caller.Username, Authorizer: caller.Authorizer}
		assert.True(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceEdge),
			"live generation must authorize permitted device")

		// Publish a new accepted local identity generation.
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, fleetScopeStoreForTest(), nil, "tok-1"))
		assert.False(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceEdge),
			"no positive scope may survive an authorization generation change")
	})

	t.Run("recovery grant denies after user generation change", func(t *testing.T) {
		resetAAABundleForTest(t)
		swapAAABundle(&aaa.Bundle{Authorizer: liveLocalAuthorizer{}}, nil)
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(
			[]aaa.UserCredential{{Name: fleetScopeRecoveryUser, Hash: "h1", Profiles: []string{aaa.ReservedRecoveryProfile}}},
			fleetScopeStoreForTest(),
			nil,
			"",
		))
		users, err := liveAcceptedLocalUsers()
		require.NoError(t, err)
		require.Len(t, users, 1)

		grant := aaa.AuthorizerForResult(liveAAABundleAuthorizer{}, aaa.AuthResult{
			Authenticated:  true,
			Source:         aaa.SourceLocal,
			Profiles:       []string{aaa.ReservedRecoveryProfile},
			UserGeneration: users[0].UserGeneration,
		})
		ctx := &pluginserver.CommandContext{Username: fleetScopeRecoveryUser, Authorizer: grant}
		assert.True(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceEdge))
		assert.True(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceCore))

		// Mutate recovery user hash so its UserGeneration advances.
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(
			[]aaa.UserCredential{{Name: fleetScopeRecoveryUser, Hash: "h2", Profiles: []string{aaa.ReservedRecoveryProfile}}},
			fleetScopeStoreForTest(),
			nil,
			"",
		))
		assert.False(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceEdge),
			"recovery grant must fail closed after user generation change")
	})
}

// VALIDATES: explicit no-RBAC mode is preserved for a live bundle whose
// selected authorizer is nil, and for a live policy-free local hub, while an
// unsupported bundle authorizer fails closed.
// PREVENTS: policy-free hubs breaking or unsupported providers failing open.
func TestHubAAAWrappersNoRBACAndUnsupportedProvider(t *testing.T) {
	resetAAABundleForTest(t)

	t.Run("live bundle with nil authorizer enforces plugin bus profiles and restricts profile wrapper", func(t *testing.T) {
		swapAAABundle(&aaa.Bundle{Authorizer: nil}, nil)
		ctx := &pluginserver.CommandContext{Username: "operator", Authorizer: liveAAABundleAuthorizer{}}
		assert.True(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceEdge))
		assert.True(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceCore))

		internalCtx := &pluginserver.CommandContext{Username: aaa.ReservedInternalRPCIdentity, Authorizer: liveAAABundleAuthorizer{}}
		assert.True(t, pluginserver.AuthorizeFleetDevice(internalCtx, fleetScopeDeviceEdge))

		pluginCtx := &pluginserver.CommandContext{
			Username:   aaa.ReservedInternalPluginPrefix + "unregistered-plugin",
			Authorizer: liveAAABundleAuthorizer{},
		}
		assert.False(t, pluginserver.AuthorizeFleetDevice(pluginCtx, fleetScopeDeviceEdge),
			"nil bundle authorizer must still enforce compiled bus command profiles on plugin identities")

		for _, profiles := range [][]string{nil, {}, {fleetScopeProfileWest}} {
			profileCtx := &pluginserver.CommandContext{
				Username:   "operator",
				Authorizer: liveAAABundleProfileAuthorizer{profiles: profiles},
			}
			assert.False(t, pluginserver.AuthorizeFleetDevice(profileCtx, fleetScopeDeviceEdge),
				"liveAAABundleProfileAuthorizer with nil bundle authorizer must fail closed for non-recovery profiles")
		}

		recoveryCtx := &pluginserver.CommandContext{
			Username:   fleetScopeRecoveryUser,
			Authorizer: liveAAABundleProfileAuthorizer{profiles: []string{aaa.ReservedRecoveryProfile}},
		}
		assert.True(t, pluginserver.AuthorizeFleetDevice(recoveryCtx, fleetScopeDeviceEdge))
	})

	t.Run("live policy-free local hub allows operator reads and fails closed on shutdown", func(t *testing.T) {
		resetAAABundleForTest(t)
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, nil, "tok-policy-free"))
		bundle, err := buildAAABundle(nil, nil, nil, nil)
		require.NoError(t, err)
		swapAAABundle(bundle, nil)

		ctx := &pluginserver.CommandContext{Username: "operator", Authorizer: liveAAABundleAuthorizer{}}
		assert.True(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceEdge))

		apiCaller, ok := liveAcceptedAPIAuthentication().Authenticate("Bearer tok-policy-free", "rest")
		require.True(t, ok)
		apiCtx := &pluginserver.CommandContext{Username: apiCaller.Username, Authorizer: apiCaller.Authorizer}
		assert.True(t, pluginserver.AuthorizeFleetDevice(apiCtx, fleetScopeDeviceEdge))

		closeAAABundle(nil)
		assert.False(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceEdge),
			"closed bundle must fail closed")
		assert.False(t, pluginserver.AuthorizeFleetDevice(apiCtx, fleetScopeDeviceEdge),
			"closed bundle must fail closed for accepted generation authorizer")
	})

	t.Run("unsupported bundle authorizer fails closed", func(t *testing.T) {
		resetAAABundleForTest(t)
		unsupported := &unsupportedBundleAuthorizer{}
		swapAAABundle(&aaa.Bundle{Authorizer: unsupported}, nil)

		ctx := &pluginserver.CommandContext{Username: "operator", Authorizer: liveAAABundleAuthorizer{}}
		assert.False(t, pluginserver.AuthorizeFleetDevice(ctx, fleetScopeDeviceEdge))
		assert.Equal(t, 0, unsupported.calls, "unsupported provider must not have Authorize called")
	})
}

func explicitPKIStoreForHubTest() *authz.Store {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "admin",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Allow},
	})
	store.AddProfile(authz.Profile{
		Name: "pki-exporter",
		Run:  authz.Section{Default: authz.Deny},
		Edit: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Allow, Match: "show pki certificate name bundle pem"},
			},
		},
	})
	store.AssignProfiles("admin-op", []string{"admin"})
	store.AssignProfiles("exporter-op", []string{"pki-exporter"})
	return store
}

// VALIDATES: all six hub AAA authorizer wrappers implement ExplicitAuthorizer,
// delegate explicit default-deny grants, fail closed when no store or bundle is
// installed or after generation retirement, and never allow via
// ReservedRecoveryProfile.
// PREVENTS: any surface wrapper silently bypassing explicit RBAC or permitting
// private-key export through recovery or unconfigured-store fallthrough.
func TestHubAuthorizerWrappersAuthorizeExplicit(t *testing.T) {
	const (
		canonicalPath = "show pki certificate name bundle pem"
		remoteAddr    = "198.51.100.40:22"
	)
	tokens := []string{"show", "pki", "certificate", "name", "dev-1", "bundle", "pem", "confirm"}

	t.Run("all six wrappers allow explicit grants and deny default-allow and recovery", func(t *testing.T) {
		resetAAABundleForTest(t)
		store := explicitPKIStoreForHubTest()
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(
			[]aaa.UserCredential{{Name: fleetScopeRecoveryUser, Hash: "h1", Profiles: []string{aaa.ReservedRecoveryProfile}}},
			store,
			nil,
			"",
		))
		liveGen := acceptedLocalIdentity.Load().authzGeneration
		require.NotZero(t, liveGen)
		swapAAABundle(&aaa.Bundle{Authorizer: liveLocalAuthorizer{}}, nil)

		exporterWrappers := map[string]aaa.Authorizer{
			"liveLocalAuthorizer":                    liveLocalAuthorizer{},
			"liveLocalProfileAuthorizer":             liveLocalProfileAuthorizer{profiles: []string{"pki-exporter"}},
			"acceptedLocalGenerationAuthorizer":      acceptedLocalGenerationAuthorizer{store: store, generation: liveGen},
			"acceptedLocalGenerationProfileFallback": acceptedLocalGenerationProfileFallback{store: store, generation: liveGen, profiles: []string{"pki-exporter"}},
			"liveAAABundleAuthorizer":                liveAAABundleAuthorizer{},
			"liveAAABundleProfileAuthorizer":         liveAAABundleProfileAuthorizer{profiles: []string{"pki-exporter"}},
		}
		for name, wrapper := range exporterWrappers {
			allowed, supported := aaa.AuthorizeExplicit(wrapper, "exporter-op", remoteAddr, canonicalPath, tokens, false)
			assert.True(t, supported, "%s must implement aaa.ExplicitAuthorizer", name)
			assert.True(t, allowed, "%s must allow explicit grant", name)
		}

		adminWrappers := map[string]aaa.Authorizer{
			"liveLocalAuthorizer":                    liveLocalAuthorizer{},
			"liveLocalProfileAuthorizer":             liveLocalProfileAuthorizer{profiles: []string{"admin"}},
			"acceptedLocalGenerationAuthorizer":      acceptedLocalGenerationAuthorizer{store: store, generation: liveGen},
			"acceptedLocalGenerationProfileFallback": acceptedLocalGenerationProfileFallback{store: store, generation: liveGen, profiles: []string{"admin"}},
			"liveAAABundleAuthorizer":                liveAAABundleAuthorizer{},
			"liveAAABundleProfileAuthorizer":         liveAAABundleProfileAuthorizer{profiles: []string{"admin"}},
		}
		for name, wrapper := range adminWrappers {
			allowed, supported := aaa.AuthorizeExplicit(wrapper, "admin-op", remoteAddr, canonicalPath, tokens, false)
			assert.True(t, supported, "%s must implement aaa.ExplicitAuthorizer", name)
			assert.False(t, allowed, "%s must deny default-action allow profile", name)
		}

		// Recovery profile MUST NEVER satisfy AuthorizeExplicit on any wrapper.
		users, err := liveAcceptedLocalUsers()
		require.NoError(t, err)
		require.Len(t, users, 1)
		recoveryGrant := aaa.AuthorizerForResult(liveAAABundleAuthorizer{}, aaa.AuthResult{
			Authenticated:  true,
			Source:         aaa.SourceLocal,
			Profiles:       []string{aaa.ReservedRecoveryProfile},
			UserGeneration: users[0].UserGeneration,
		})
		recoveryWrappers := map[string]aaa.Authorizer{
			"AuthorizerForResult(recovery)":            recoveryGrant,
			"liveLocalProfileAuthorizer(recovery)":     liveLocalProfileAuthorizer{profiles: []string{aaa.ReservedRecoveryProfile}},
			"acceptedLocalGenerationProfileFallback":   acceptedLocalGenerationProfileFallback{store: store, generation: liveGen, profiles: []string{aaa.ReservedRecoveryProfile}},
			"liveAAABundleProfileAuthorizer(recovery)": liveAAABundleProfileAuthorizer{profiles: []string{aaa.ReservedRecoveryProfile}},
		}
		for name, wrapper := range recoveryWrappers {
			allowed, supported := aaa.AuthorizeExplicit(wrapper, fleetScopeRecoveryUser, remoteAddr, canonicalPath, tokens, false)
			assert.True(t, supported, "%s must implement aaa.ExplicitAuthorizer", name)
			assert.False(t, allowed, "%s must never allow via ReservedRecoveryProfile", name)
		}
	})

	t.Run("missing store, missing bundle, nil bundle authorizer, retired generation, and unsupported provider fail closed", func(t *testing.T) {
		resetAAABundleForTest(t)

		// Absent identity and absent bundle deny on all six wrappers.
		store := explicitPKIStoreForHubTest()
		for name, wrapper := range map[string]aaa.Authorizer{
			"liveLocalAuthorizer":                    liveLocalAuthorizer{},
			"liveLocalProfileAuthorizer":             liveLocalProfileAuthorizer{profiles: []string{"pki-exporter"}},
			"acceptedLocalGenerationAuthorizer":      acceptedLocalGenerationAuthorizer{store: store, generation: 1},
			"acceptedLocalGenerationProfileFallback": acceptedLocalGenerationProfileFallback{store: store, generation: 1, profiles: []string{"pki-exporter"}},
			"liveAAABundleAuthorizer":                liveAAABundleAuthorizer{},
			"liveAAABundleProfileAuthorizer":         liveAAABundleProfileAuthorizer{profiles: []string{"pki-exporter"}},
		} {
			allowed, supported := aaa.AuthorizeExplicit(wrapper, "exporter-op", remoteAddr, canonicalPath, tokens, false)
			assert.True(t, supported, "%s must implement aaa.ExplicitAuthorizer", name)
			assert.False(t, allowed, "%s must fail closed with no accepted identity/bundle", name)
		}

		// Live policy-free hub (nil store, nil bundle authorizer) still fails closed, including for recovery.
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, nil, ""))
		liveGen := acceptedLocalIdentity.Load().authzGeneration
		swapAAABundle(&aaa.Bundle{Authorizer: nil}, nil)

		for name, wrapper := range map[string]aaa.Authorizer{
			"liveLocalAuthorizer(nil store)":                       liveLocalAuthorizer{},
			"liveLocalProfileAuthorizer(recovery, nil store)":      liveLocalProfileAuthorizer{profiles: []string{aaa.ReservedRecoveryProfile}},
			"acceptedLocalGenerationAuthorizer(nil store)":         acceptedLocalGenerationAuthorizer{store: nil, generation: liveGen},
			"acceptedLocalGenerationProfileFallback(recovery)":     acceptedLocalGenerationProfileFallback{store: nil, generation: liveGen, profiles: []string{aaa.ReservedRecoveryProfile}},
			"liveAAABundleAuthorizer(nil bundle authorizer)":       liveAAABundleAuthorizer{},
			"liveAAABundleProfileAuthorizer(recovery, nil author)": liveAAABundleProfileAuthorizer{profiles: []string{aaa.ReservedRecoveryProfile}},
		} {
			allowed, supported := aaa.AuthorizeExplicit(wrapper, fleetScopeRecoveryUser, remoteAddr, canonicalPath, tokens, false)
			assert.True(t, supported, "%s must implement aaa.ExplicitAuthorizer", name)
			assert.False(t, allowed, "%s must fail closed when no store/authorizer is installed", name)
		}

		// Generation retirement denies acceptedLocalGeneration wrappers.
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, store, nil, ""))
		gen1 := acceptedLocalIdentity.Load().authzGeneration
		swapAAABundle(&aaa.Bundle{Authorizer: liveLocalAuthorizer{}}, nil)

		genAuth := acceptedLocalGenerationAuthorizer{store: store, generation: gen1}
		genFallback := acceptedLocalGenerationProfileFallback{store: store, generation: gen1, profiles: []string{"pki-exporter"}}
		allowed, _ := aaa.AuthorizeExplicit(genAuth, "exporter-op", remoteAddr, canonicalPath, tokens, false)
		assert.True(t, allowed)
		allowed, _ = aaa.AuthorizeExplicit(genFallback, "exporter-op", remoteAddr, canonicalPath, tokens, false)
		assert.True(t, allowed)

		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, store, nil, ""))
		allowed, _ = aaa.AuthorizeExplicit(genAuth, "exporter-op", remoteAddr, canonicalPath, tokens, false)
		assert.False(t, allowed, "retired generation must deny acceptedLocalGenerationAuthorizer")
		allowed, _ = aaa.AuthorizeExplicit(genFallback, "exporter-op", remoteAddr, canonicalPath, tokens, false)
		assert.False(t, allowed, "retired generation must deny acceptedLocalGenerationProfileFallback")

		// Unsupported bundle authorizer fails closed without invoking Authorize.
		unsupported := &unsupportedBundleAuthorizer{}
		swapAAABundle(&aaa.Bundle{Authorizer: unsupported}, nil)
		allowed, supported := aaa.AuthorizeExplicit(liveAAABundleAuthorizer{}, "exporter-op", remoteAddr, canonicalPath, tokens, false)
		assert.True(t, supported)
		assert.False(t, allowed)
		assert.Equal(t, 0, unsupported.calls, "unsupported bundle authorizer must not have Authorize called")
	})
}
