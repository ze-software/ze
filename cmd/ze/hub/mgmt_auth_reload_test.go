package hub

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
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

// VALIDATES: the API reloader resolves a token from the reloaded config and
// hands the migrator material a server can install.
// PREVENTS: registering a reloader that reports a mode without the credentials
// to build it, leaving the rebuild with nothing to apply.
func TestAPIAuthReloaderResolvesConfiguredToken(t *testing.T) {
	migrator := newListenerMigrator()
	registerMgmtAuthReloaders(migrator, mgmtAuthInputs{
		apiUsersLive: func() ([]authz.UserConfig, error) { return nil, nil },
	})
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
	calls := 0
	migrator := newListenerMigrator()
	registerMgmtAuthReloaders(migrator, mgmtAuthInputs{
		apiUsersLive: func() ([]authz.UserConfig, error) {
			calls++
			return nil, nil
		},
	})

	_, ok, err := migrator.authReloaders["rest"](zeconfig.NewTree())
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Zero(t, calls, "removing api-server must not consult or replace the running credentials")

	rest := &recordingAuthServer{token: "running-token"}
	migrator.setREST(rest)
	migrator.markAuthenticated(svcREST)
	_, err = migrator.reloadListeners(context.Background(), zeconfig.NewTree())
	require.NoError(t, err)
	assert.Equal(t, "running-token", rest.token)
	assert.Empty(t, rest.updates, "removing api-server must leave running credentials untouched")
}

func apiReloadUser(t *testing.T, name, password string) authz.UserConfig {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return authz.UserConfig{Name: name, Hash: string(hash)}
}

func apiReloadSystemRoot(users ...authz.UserConfig) map[string]any {
	entries := make(map[string]any, len(users))
	for _, user := range users {
		entries[user.Name] = map[string]any{"password": user.Hash, "profile": "admin"}
	}
	return map[string]any{"authentication": map[string]any{"user": entries}}
}

// VALIDATES: AC-9 and AC-12, runReload installs the candidate provider before
// apiAuthReloader resolves its live users. A no-SSH config user produces
// working per-user material, and that material follows later user changes.
// PREVENTS: resolving from the previous provider tree or rebuilding from a
// separate snapshot parsed directly from the candidate tree.
func TestAPIAuthReloaderUsesLiveUsersWithoutSSHBlock(t *testing.T) {
	alice := apiReloadUser(t, "alice", "alice-pass")
	bob := apiReloadUser(t, "bob", "bob-pass")
	candidate := apiTokenTree("").ToMap()
	candidate["system"] = apiReloadSystemRoot(alice)
	parsedCandidate := treeFromMap(candidate)

	server, provider := reloadDriver(t, candidate)
	calls := 0
	usersLive := func() ([]authz.UserConfig, error) {
		calls++
		return liveConfigUsers(provider)
	}
	rest := &recordingAuthServer{
		recordingReconfigurable: recordingReconfigurable{},
		token:                   "boot-token",
	}
	migrator := newListenerMigrator()
	migrator.setREST(rest)
	migrator.markAuthenticated(svcREST)
	registerMgmtAuthReloaders(migrator, mgmtAuthInputs{apiUsersLive: usersLive})

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return candidate, parsedCandidate, nil
	}
	require.NoError(t, runReload(server, nil, provider, nil, "", load, migrator))
	require.NotNil(t, rest.authenticator)
	assert.Equal(t, 1, calls, "runReload must refresh the provider before resolving API mode once")
	assert.Empty(t, rest.token, "per-user mode must replace the previous shared token")
	assert.Equal(t, []string{""}, rest.updates, "the candidate authenticator must be installed")

	user, authenticated := rest.authenticator("Bearer alice:alice-pass")
	require.True(t, authenticated, "the installed material must authenticate, not only report a boolean mode")
	assert.Equal(t, "alice", user)
	_, authenticated = rest.authenticator("Bearer alice:wrong")
	assert.False(t, authenticated)

	provider.SetRoot("system", apiReloadSystemRoot(bob))
	_, authenticated = rest.authenticator("Bearer alice:alice-pass")
	assert.False(t, authenticated, "a user removed from the live source must stop authenticating")
	user, authenticated = rest.authenticator("Bearer bob:bob-pass")
	require.True(t, authenticated, "a user added to the live source must authenticate without restart")
	assert.Equal(t, "bob", user)
}

// VALIDATES: AC-10, an unreadable live API user source rejects the reload and
// yields no installable intent.
// PREVENTS: a failed source read becoming a valid token-only or anonymous mode.
func TestAPIAuthReloaderFailsClosedWhenLiveUsersUnreadable(t *testing.T) {
	sourceErr := errors.New("provider user read failed")
	migrator := newListenerMigrator()
	registerMgmtAuthReloaders(migrator, mgmtAuthInputs{
		apiUsersLive: func() ([]authz.UserConfig, error) {
			return nil, sourceErr
		},
	})

	intent, ok, err := migrator.authReloaders["rest"](apiTokenTree("configured-token"))
	require.ErrorIs(t, err, sourceErr)
	assert.Contains(t, err.Error(), "resolve live API users", "the reload refusal must name the failed source")
	assert.False(t, ok)
	assert.False(t, intent.authenticated)
	assert.Nil(t, intent.authenticator)

	rest := &recordingAuthServer{token: "running-token"}
	migrator.setREST(rest)
	migrator.markAuthenticated(svcREST)
	_, reloadErr := migrator.reloadListeners(context.Background(), apiTokenTree("configured-token"))
	require.ErrorIs(t, reloadErr, sourceErr)
	assert.Equal(t, "running-token", rest.token, "a failed source read must preserve running credentials")
	assert.Empty(t, rest.updates, "a failed source read must install no new intent")
}

// VALIDATES: AC-7 and per-user precedence, a successful empty live read keeps
// token or no-auth mode, while a non-empty live read installs per-user
// authentication that rejects the shared token as a user credential.
// PREVENTS: confusing an empty list with a source error, or letting a token
// bypass a configured user's password after reload.
func TestAPIAuthReloaderProceedsWithTokenAndNoUsers(t *testing.T) {
	t.Run("empty live source keeps token mode", func(t *testing.T) {
		calls := 0
		migrator := newListenerMigrator()
		registerMgmtAuthReloaders(migrator, mgmtAuthInputs{
			apiUsersLive: func() ([]authz.UserConfig, error) {
				calls++
				return nil, nil
			},
		})
		intent, ok, err := migrator.authReloaders["rest"](apiTokenTree("configured-token"))
		require.NoError(t, err)
		require.True(t, ok)
		assert.True(t, intent.authenticated)
		assert.Equal(t, "configured-token", intent.token)
		assert.Nil(t, intent.authenticator)
		assert.Equal(t, 1, calls)
	})

	t.Run("empty live source keeps no-auth mode", func(t *testing.T) {
		migrator := newListenerMigrator()
		registerMgmtAuthReloaders(migrator, mgmtAuthInputs{
			apiUsersLive: func() ([]authz.UserConfig, error) {
				return []authz.UserConfig{}, nil
			},
		})
		intent, ok, err := migrator.authReloaders["rest"](apiTokenTree(""))
		require.NoError(t, err)
		require.True(t, ok)
		assert.False(t, intent.authenticated)
		assert.Empty(t, intent.token)
		assert.Nil(t, intent.authenticator)
	})

	t.Run("per-user mode takes precedence over token", func(t *testing.T) {
		alice := apiReloadUser(t, "alice", "alice-pass")
		migrator := newListenerMigrator()
		registerMgmtAuthReloaders(migrator, mgmtAuthInputs{
			apiUsersLive: func() ([]authz.UserConfig, error) {
				return []authz.UserConfig{alice}, nil
			},
		})
		intent, ok, err := migrator.authReloaders["rest"](apiTokenTree("configured-token"))
		require.NoError(t, err)
		require.True(t, ok)
		require.NotNil(t, intent.authenticator)
		_, authenticated := intent.authenticator("Bearer configured-token")
		assert.False(t, authenticated, "the shared token is not a per-user credential")
		_, authenticated = intent.authenticator("Bearer alice:alice-pass")
		assert.True(t, authenticated)
	})
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
	rest := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}}}
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

// VALIDATES: an unbuilt transport does not refuse the reload when the API's
// authentication changes, end to end through reloadListeners (AC-5).
// PREVENTS: the operator-facing symptom the marking fix removes -- a token
// removal failing the whole commit over a server that does not exist.
func TestReloadListenersProceedsWhenSiblingTransportWasNeverBuilt(t *testing.T) {
	rest := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}}, token: "boot-token"}
	migrator := newListenerMigrator()
	migrator.setREST(rest)
	markMgmtAuth(migrator, map[string]bool{"rest": true, "grpc": true})

	// One api-server block answers for both transports, so both reloaders read
	// the same intent: the operator removed the token.
	migrator.setAuthReloader("rest", staticAuth(false, ""))
	migrator.setAuthReloader("grpc", staticAuth(false, ""))

	_, err := migrator.reloadListeners(context.Background(), restOnlyTree("127.0.0.1", "8081"))
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
