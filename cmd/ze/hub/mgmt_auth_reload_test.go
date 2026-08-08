package hub

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
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
	migrator := NewListenerMigrator(nil)
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
	migrator := NewListenerMigrator(nil)
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
	migrator := NewListenerMigrator(nil)
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

// VALIDATES: the API reloader resolves a token from the reloaded config and
// hands the migrator material a server can install.
// PREVENTS: registering a reloader that reports a mode without the credentials
// to build it, leaving the rebuild with nothing to apply.
func TestAPIAuthReloaderResolvesConfiguredToken(t *testing.T) {
	migrator := NewListenerMigrator(nil)
	registerMgmtAuthReloaders(migrator, mgmtAuthInputs{})

	intent, ok, err := migrator.authReloaders["rest"](apiTokenTree("reloaded-token"))
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, intent.authenticated)
	assert.Equal(t, "reloaded-token", intent.token)

	// REST and gRPC share the API block, so they must share one answer.
	grpcIntent, _, err := migrator.authReloaders["grpc"](apiTokenTree("reloaded-token"))
	require.NoError(t, err)
	assert.Equal(t, intent.token, grpcIntent.token)
}

// VALIDATES: a reloaded config with no api-server block leaves the running API
// servers' credentials alone.
// PREVENTS: deleting a config block silently stripping authentication off a
// server that is still listening.
func TestAPIAuthReloaderSilentWithoutBlock(t *testing.T) {
	migrator := NewListenerMigrator(nil)
	registerMgmtAuthReloaders(migrator, mgmtAuthInputs{})

	_, ok, err := migrator.authReloaders["rest"](zeconfig.NewTree())
	require.NoError(t, err)
	assert.False(t, ok)
}

// setAPIConfigDir points zefs at a directory holding no database, so
// loadZefsUsers fails the way it does when the credentials become unreadable.
func setAPIConfigDir(t *testing.T, dir string) {
	t.Helper()
	orig := env.Get("ze.config.dir")
	t.Cleanup(func() { _ = env.Set("ze.config.dir", orig) })
	require.NoError(t, env.Set("ze.config.dir", dir))
}

// VALIDATES: a daemon that COULD read its power-user credentials at boot and
// cannot read them now fails the reload rather than rebuilding the API servers
// without them.
// PREVENTS: the most security-relevant branch in this file being an untested
// claim. Rebuilding on an empty user set silently drops every power user, which
// is the fail-open shape ai/rules/evidence.md names.
func TestAPIAuthReloaderFailsClosedWhenCredentialsBecomeUnreadable(t *testing.T) {
	setAPIConfigDir(t, t.TempDir())

	migrator := NewListenerMigrator(nil)
	registerMgmtAuthReloaders(migrator, mgmtAuthInputs{apiZefsUsersOK: true})

	intent, ok, err := migrator.authReloaders["rest"](apiTokenTree(""))
	require.Error(t, err, "credentials that authenticated callers at boot must not vanish silently")
	assert.Contains(t, err.Error(), "no longer readable")
	assert.False(t, ok)
	assert.False(t, intent.authenticated)
}

// VALIDATES: a daemon that never had those credentials keeps reloading; the
// fail-closed branch is keyed on losing them, not on their absence.
// PREVENTS: over-refusing on every box without zefs initialized, which would
// make the guard's first real failure indistinguishable from routine noise.
func TestAPIAuthReloaderProceedsWhenCredentialsNeverExisted(t *testing.T) {
	setAPIConfigDir(t, t.TempDir())

	migrator := NewListenerMigrator(nil)
	registerMgmtAuthReloaders(migrator, mgmtAuthInputs{apiZefsUsersOK: false})

	intent, ok, err := migrator.authReloaders["rest"](apiTokenTree("configured-token"))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "configured-token", intent.token)
	assert.True(t, intent.authenticated)
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
	migrator := NewListenerMigrator(nil)

	// Boot order: the guard's answer reaches the migrator first, then the
	// servers do. Every surface is classified, handle or no handle.
	markMgmtAuth(migrator, map[string]bool{svcWeb: false, svcMCP: true, svcREST: true, svcGRPC: true})
	for _, name := range []string{svcWeb, svcMCP, svcREST, svcGRPC} {
		_, known := migrator.runningAuth(name)
		assert.Truef(t, known, "%s must be classified before its handle is installed", name)
	}

	web := &recordingReconfigurable{addrs: []string{"127.0.0.1:3443"}}
	migrator.SetWeb(web)

	_, err := migrator.ReloadListeners(context.Background(), webOnlyTree(nonLoopbackServiceTree("3444")))
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
// server. It also stops a reloader that FAILS -- apiAuthReloader refuses when
// the power-user credentials stop being readable -- failing a reload over a
// server that does not exist.
func TestUnbuiltSurfaceResolvesNoAuthIntent(t *testing.T) {
	rest := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}}}
	migrator := NewListenerMigrator(nil)
	markMgmtAuth(migrator, map[string]bool{svcREST: true, svcGRPC: true})
	migrator.SetREST(rest)

	called := map[string]int{}
	migrator.SetAuthReloader(svcREST, func(*zeconfig.Tree) (authIntent, bool, error) {
		called[svcREST]++
		return authIntent{authenticated: true}, true, nil
	})
	migrator.SetAuthReloader(svcGRPC, func(*zeconfig.Tree) (authIntent, bool, error) {
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

// VALIDATES: an unbuilt transport does not refuse the reload when the API's
// authentication changes, end to end through ReloadListeners (AC-5).
// PREVENTS: the operator-facing symptom the marking fix removes -- a token
// removal failing the whole commit over a server that does not exist.
func TestReloadListenersProceedsWhenSiblingTransportWasNeverBuilt(t *testing.T) {
	rest := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}}, token: "boot-token"}
	migrator := NewListenerMigrator(nil)
	migrator.SetREST(rest)
	markMgmtAuth(migrator, map[string]bool{"rest": true, "grpc": true})

	// One api-server block answers for both transports, so both reloaders read
	// the same intent: the operator removed the token.
	migrator.SetAuthReloader("rest", staticAuth(false, ""))
	migrator.SetAuthReloader("grpc", staticAuth(false, ""))

	_, err := migrator.ReloadListeners(context.Background(), restOnlyTree("127.0.0.1", "8081"))
	require.NoError(t, err, "a transport that was never built cannot refuse a reload")
	assert.Equal(t, []string{""}, rest.updates, "the running transport still rebuilds its authentication")
	assert.Empty(t, rest.token, "the removed token is removed from the running server")
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
