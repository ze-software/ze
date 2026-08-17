package hub

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/env"
)

// webTreeWithInsecure builds a config tree whose web block carries the given
// value of the `insecure` leaf.
func webTreeWithInsecure(insecure string) *zeconfig.Tree {
	svc := zeconfig.NewTree()
	svc.Set("enabled", "true")
	if insecure != "" {
		svc.Set("insecure", insecure)
	}
	srv := zeconfig.NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", "3443")
	svc.AddListEntry("server", "main", srv)
	return webOnlyTree(svc)
}

// VALIDATES: a reloaded config that clears `insecure` resolves the web surface
// as authenticated, and setting it resolves it as unauthenticated.
// PREVENTS: the reload guard reading an auth mode the operator has replaced,
// which is what made an auth-mode edit a silent no-op until a restart.
func TestWebAuthReloaderFollowsConfigLeaf(t *testing.T) {
	migrator := newListenerMigrator()
	registerMgmtAuthReloaders(migrator, mgmtAuthInputs{webFollowsConfig: true})
	reload := migrator.authReloaders["web"]
	require.NotNil(t, reload)

	intent, ok, err := reload(webTreeWithInsecure("true"))
	require.NoError(t, err)
	require.True(t, ok)
	assert.False(t, intent.authenticated)

	intent, ok, err = reload(webTreeWithInsecure(""))
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, intent.authenticated)
}

// VALIDATES: when a flag or environment variable decided the web auth mode at
// boot, a reloaded config leaf does NOT override it.
// PREVENTS: the reload answering the precedence question differently from boot,
// so the migrator reports a change the daemon would never make.
func TestWebAuthReloaderIgnoresConfigWhenFlagDecided(t *testing.T) {
	migrator := newListenerMigrator()
	registerMgmtAuthReloaders(migrator, mgmtAuthInputs{webFollowsConfig: false})

	_, ok, err := migrator.authReloaders["web"](webTreeWithInsecure("true"))
	require.NoError(t, err)
	assert.False(t, ok, "a reload must not re-decide what a flag or environment variable owns")
}

// VALIDATES: the MCP reloader mirrors the server's effective-mode precedence,
// so an explicit auth-mode of "none" reads as unauthenticated even with a token.
// PREVENTS: the reload guard calling an accept-all MCP server authenticated,
// the exact mismatch mcpListenerAuthenticated exists to close.
func TestMCPAuthReloaderMirrorsServerPrecedence(t *testing.T) {
	migrator := newListenerMigrator()
	registerMgmtAuthReloaders(migrator, mgmtAuthInputs{mcpTokenBase: "flag-token"})
	reload := migrator.authReloaders["mcp"]
	require.NotNil(t, reload)

	intent, ok, err := reload(mcpSettingsTree("none"))
	require.NoError(t, err)
	require.True(t, ok)
	assert.False(t, intent.authenticated, "an explicit none must not read as authenticated, token or not")

	intent, _, err = reload(mcpSettingsTree("bearer"))
	require.NoError(t, err)
	assert.True(t, intent.authenticated)
}

func mcpSettingsTree(authMode string) *zeconfig.Tree {
	mcp := zeconfig.NewTree()
	mcp.Set("auth-mode", authMode)
	env := zeconfig.NewTree()
	env.SetContainer("mcp", mcp)
	tree := zeconfig.NewTree()
	tree.SetContainer("environment", env)
	return tree
}

// VALIDATES: the API reloader resolves the final exposure mode without handing
// candidate credential material to the listener migrator.
func TestAPIAuthReloaderResolvesConfiguredToken(t *testing.T) {
	users := func() ([]authz.UserConfig, error) { return nil, nil }
	migrator := newListenerMigrator()
	registerMgmtAuthReloaders(migrator, mgmtAuthInputs{
		apiCandidateUsers: users,
	})
	intent, ok, err := migrator.authReloaders["rest"](apiTokenTree("reloaded-token"))
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, intent.authenticated)

	grpcIntent, _, err := migrator.authReloaders["grpc"](apiTokenTree("reloaded-token"))
	require.NoError(t, err)
	assert.Equal(t, intent.authenticated, grpcIntent.authenticated)
}

// VALIDATES: an absent api-server block still produces candidate intent for an
// environment-started listener from candidate users and the retained accepted
// token.
// PREVENTS: removing the block and final user publishing no-auth while a
// non-loopback API transport remains live.
func TestAPIAuthReloaderAbsentBlockUsesCandidateUsersAndRetainedToken(t *testing.T) {
	resetAAABundleForTest(t)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, nil, "retained-token"))
	usersCalled := false
	var candidateUsers []authz.UserConfig
	migrator := newListenerMigrator()
	registerMgmtAuthReloaders(migrator, mgmtAuthInputs{
		apiCandidateUsers: func() ([]authz.UserConfig, error) {
			usersCalled = true
			return candidateUsers, nil
		},
	})

	intent, ok, err := migrator.authReloaders["grpc"](zeconfig.NewTree())
	require.NoError(t, err)
	require.True(t, ok, "a running API transport needs known candidate intent even when its block is absent")
	assert.True(t, intent.authenticated, "the identity publisher retains the accepted token when the block is absent")
	assert.True(t, usersCalled, "candidate users must participate in the published API mode")

	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, nil, ""))
	intent, ok, err = migrator.authReloaders["grpc"](zeconfig.NewTree())
	require.NoError(t, err)
	require.True(t, ok)
	assert.False(t, intent.authenticated, "no candidate users and no retained token select no-auth")

	candidateUsers = []authz.UserConfig{{Name: "operator", Hash: "unused"}}
	intent, ok, err = migrator.authReloaders["grpc"](zeconfig.NewTree())
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, intent.authenticated, "candidate users authenticate even without a retained token")
}

// VALIDATES: an unchanged reload of an environment-started gRPC API reads the
// token from a dormant api-server block even when neither transport has
// enabled=true, while its non-loopback address and TLS settings remain dormant
// config settings rather than a request to start another transport.
// PREVENTS: the auth reloader using the config-enable gate, classifying the
// running non-loopback gRPC listener as unauthenticated, and rejecting SIGHUP.
func TestRunReloadEnvEnabledGRPCKeepsDormantAPISettings(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	require.NoError(t, env.Set("ze.api-server.grpc.enabled", "1"))
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	treeValue := dormantAPIBlock(t, true)
	grpcBlock := treeValue.GetContainer("environment").GetContainer("api-server").GetContainer("grpc")
	require.NotNil(t, grpcBlock)
	grpcServer := grpcBlock.GetList("server")["main"]
	require.NotNil(t, grpcServer)
	grpcServer.Set("ip", "192.0.2.10")
	system := zeconfig.NewTree()
	system.SetContainer("authentication", zeconfig.NewTree())
	treeValue.SetContainer("system", system)
	tree := treeValue.ToMap()

	srv, cp := reloadDriver(t, tree)
	cp.SetRoot("system", map[string]any{"authentication": map[string]any{}})
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, resolveCandidate, "api-s3cret"))

	grpc := &recordingAuthServer{
		recordingReconfigurable: recordingReconfigurable{addrs: []string{"192.0.2.10:50052"}},
		authenticated:           true,
	}
	lm := newListenerMigrator()
	lm.setGRPC(grpc)
	lm.markAuthenticated(svcGRPC)
	registerMgmtAuthReloaders(lm, mgmtAuthInputs{apiCandidateUsers: resolveCandidate})
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeValue, nil
	}

	require.NoError(t, runReload(srv, cp, load, lm))
	assert.Empty(t, grpc.calls, "an unchanged env-started listener must not be rebound")
	caller, ok := liveAcceptedAPIAuthentication().Authenticate("Bearer api-s3cret")
	require.True(t, ok, "the dormant block token must remain the live credential")
	assert.Equal(t, aaa.ReservedSharedAPIUsername, caller.Username)
}

// VALIDATES: removing both environment.api-server and the final local user
// still classifies an already-running non-loopback gRPC listener from the
// candidate identity that would be published.
// PREVENTS: absent-block intent falling back to the authenticated running mode,
// accepting the reload, then publishing no API authentication on that listener.
func TestRunReloadRejectsAbsentAPIBlockAndLastUserRemovalOnRemoteGRPC(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	tree := map[string]any{
		"system": map[string]any{"authentication": map[string]any{}},
	}
	treeValue := treeFromMap(tree)
	srv, cp := reloadDriver(t, tree)
	oldSystem := map[string]any{
		"authentication": map[string]any{
			"user": map[string]any{
				"operator": map[string]any{"password": "unused"},
			},
		},
	}
	cp.SetRoot("system", oldSystem)
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(
		[]authz.UserConfig{{Name: "operator", Hash: "unused"}},
		nil,
		resolveCandidate,
		"",
	))

	grpc := &recordingAuthServer{
		recordingReconfigurable: recordingReconfigurable{addrs: []string{"192.0.2.10:50052"}},
		authenticated:           true,
	}
	lm := newListenerMigrator()
	lm.setGRPC(grpc)
	lm.markAuthenticated(svcGRPC)
	registerMgmtAuthReloaders(lm, mgmtAuthInputs{apiCandidateUsers: resolveCandidate})
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeValue, nil
	}

	err := runReload(srv, cp, load, lm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "grpc listener")
	assert.Contains(t, err.Error(), "192.0.2.10:50052")
	assert.Empty(t, grpc.calls, "exposure must be rejected before any listener mutation")
	authentication := liveAcceptedAPIAuthentication()
	assert.True(t, authentication.Required, "the rejected no-auth candidate must not be published")
	assert.NotNil(t, authentication.Authenticate, "the prior user-authenticated generation must remain accepted")
}

func apiTokenTree(token string) *zeconfig.Tree {
	rest := zeconfig.NewTree()
	rest.Set("enabled", "true")
	apiBlock := zeconfig.NewTree()
	apiBlock.Set("token", token)
	apiBlock.SetContainer("rest", rest)
	grpcBlock := zeconfig.NewTree()
	grpcBlock.Set("enabled", "true")
	apiBlock.SetContainer("grpc", grpcBlock)
	env := zeconfig.NewTree()
	env.SetContainer("api-server", apiBlock)
	tree := zeconfig.NewTree()
	tree.SetContainer("environment", env)
	return tree
}

// VALIDATES: markMgmtAuth classifies only a surface whose server handle was
// built, so the reload guard holds no record for a server the daemon never
// started (AC-5).
// PREVENTS: a classification that lands after the handle does. checkReloadExposure
// SKIPS a service it has no record for, which is its permissive branch, so an
// unauthenticated web server installed on the migrator before the mark could be
// migrated to a public address. Web carries no loopback rule of its own: this
// guard is the only one it has.
func TestMarkMgmtAuthClassifiesBeforeAnyHandleExists(t *testing.T) {
	migrator := newListenerMigrator()

	// Boot order: the guard's answer reaches the migrator first, then the
	// servers do. Every surface is classified, handle or no handle.
	markMgmtAuth(migrator, map[string]bool{svcWeb: false, svcMCP: true, svcREST: true, svcGRPC: true})
	for _, name := range []string{svcWeb, svcMCP, svcREST, svcGRPC} {
		_, known := migrator.runningAuth(name)
		assert.Truef(t, known, "%s must be classified before its handle is installed", name)
	}

	web := &recordingReconfigurable{addrs: []string{"127.0.0.1:3443"}}
	migrator.setWeb(web)

	_, err := migrator.reloadListeners(context.Background(), webOnlyTree(nonLoopbackServiceTree("3444")))
	require.Error(t, err, "an unauthenticated web server must not migrate to a public address")
	assert.Contains(t, err.Error(), "0.0.0.0:3444")
	assert.Empty(t, web.calls, "the refusal comes before anything is rebound")
}

// VALIDATES: a surface classified at boot but never built produces no auth
// intent, so its reloader is not called and it cannot refuse the reload (AC-5).
// PREVENTS: one `api-server` block classifying BOTH transports into a refusal.
// A config that enables REST alone classifies gRPC too; gRPC has no handle to
// rebuild, so without this an operator removing the API token was told "grpc
// cannot change its authentication while running" by a daemon running no gRPC
// server. It also stops an unresolved live API user source from failing a
// reload over a server that does not exist.
func TestUnbuiltSurfaceResolvesNoAuthIntent(t *testing.T) {
	rest := &recordingAuthServer{
		recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}},
		authenticated:           true,
	}
	migrator := newListenerMigrator()
	markMgmtAuth(migrator, map[string]bool{svcREST: true, svcGRPC: true})
	migrator.setREST(rest)

	called := map[string]int{}
	migrator.setAuthReloader(svcREST, func(*zeconfig.Tree) (authIntent, bool, error) {
		called[svcREST]++
		return authIntent{authenticated: true}, true, nil
	})
	migrator.setAuthReloader(svcGRPC, func(*zeconfig.Tree) (authIntent, bool, error) {
		called[svcGRPC]++
		return authIntent{}, false, errNoLiveConfigProvider
	})

	intents, err := migrator.resolveAuthIntents(restOnlyTree("127.0.0.1", "8081"))
	require.NoError(t, err, "a reloader for a server that was never built must not run")
	require.Len(t, intents, 1)
	assert.Equal(t, svcREST, intents[0].name)
	assert.Equal(t, 1, called[svcREST])
	assert.Equal(t, 0, called[svcGRPC], "the unbuilt transport's reloader is never CALLED, not merely ignored")
}

// restOnlyTree builds a config tree whose api-server block enables REST alone,
// on the given endpoint, with no token. gRPC is absent, which is the shape that
// leaves one transport running and the other never built.
func restOnlyTree(host, port string) *zeconfig.Tree {
	srv := zeconfig.NewTree()
	srv.Set("ip", host)
	srv.Set("port", port)
	rest := zeconfig.NewTree()
	rest.Set("enabled", "true")
	rest.AddListEntry("server", "main", srv)
	apiBlock := zeconfig.NewTree()
	apiBlock.SetContainer("rest", rest)
	envBlock := zeconfig.NewTree()
	envBlock.SetContainer("api-server", apiBlock)
	tree := zeconfig.NewTree()
	tree.SetContainer("environment", envBlock)
	return tree
}
