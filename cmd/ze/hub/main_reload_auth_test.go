// Design: ai/rules/evidence.md -- the exposure guard, re-run on every reload
// Related: main_reload.go -- runReload, the driver these tests exercise
// Related: listener_migrate.go -- reloadListeners returns the undo runReload runs

package hub

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"
	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/env"
)

// reloadDriver builds the smallest server runReload will drive all the way to
// the certificate-rotation step. The coordinator is handed the SAME tree the
// reload carries, so the plugin-server diff is empty and ReloadConfig returns
// without needing plugins; every step after it still runs in order.
func reloadDriver(t *testing.T, tree map[string]any) (*pluginserver.Server, *zeconfig.Provider) {
	t.Helper()
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, plugin.NewCoordinator(tree))
	require.NoError(t, err)
	return srv, zeconfig.NewProvider()
}

type failAcquireStorage struct {
	storage.Storage
}

func (s failAcquireStorage) AcquireLock(string) (storage.WriteGuard, error) {
	return nil, assert.AnError
}

type rollbackFailReconfigurable struct {
	addrs []string
	calls int
}

func (r *rollbackFailReconfigurable) Addresses() []string {
	return append([]string(nil), r.addrs...)
}

func (r *rollbackFailReconfigurable) Reconfigure(_ context.Context, addrs []string) error {
	r.calls++
	if r.calls == 2 {
		return assert.AnError
	}
	r.addrs = append([]string(nil), addrs...)
	return nil
}

type retryRollbackFailReconfigurable struct {
	addrs []string
	calls int
}

func (r *retryRollbackFailReconfigurable) Addresses() []string {
	return append([]string(nil), r.addrs...)
}

func (r *retryRollbackFailReconfigurable) Reconfigure(_ context.Context, addrs []string) error {
	r.calls++
	if r.calls > 1 {
		return assert.AnError
	}
	r.addrs = append([]string(nil), addrs...)
	return nil
}

func reloadIdentitySystem(hash, action string, assigned bool) map[string]any {
	user := map[string]any{"password": hash}
	if assigned {
		user["profile"] = "reload-policy"
	}
	return map[string]any{
		"authentication": map[string]any{
			"user": map[string]any{"operator": user},
		},
		"authorization": map[string]any{
			"profile": map[string]any{
				"reload-policy": map[string]any{
					"run":  map[string]any{"default-action": action},
					"edit": map[string]any{"default-action": action},
				},
			},
		},
	}
}

func withReloadIdentity(tree map[string]any, hash, action string, assigned bool) map[string]any {
	tree["system"] = reloadIdentitySystem(hash, action, assigned)
	return tree
}

func reloadAuthorizationStore() *authz.Store {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "reload-policy",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Allow},
	})
	store.AssignProfiles("operator", []string{"reload-policy"})
	return store
}

// VALIDATES: candidate credentials and policy remain staged together through
// listener authentication rebuild, then become visible in one final
// publication.
// PREVENTS: a candidate password authenticating while the prior allow policy is
// still live.
func TestRunReloadPublishesAcceptedIdentityAtomically(t *testing.T) {
	resetAAABundleForTest(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-pass"), bcrypt.MinCost)
	require.NoError(t, err)
	newHash, err := bcrypt.GenerateFromPassword([]byte("new-pass"), bcrypt.MinCost)
	require.NoError(t, err)

	tree := withReloadIdentity(map[string]any{}, string(newHash), "deny", true)
	srv, cp := reloadDriver(t, tree)
	oldSystem := reloadIdentitySystem(string(oldHash), "allow", true)
	cp.SetRoot("system", oldSystem)
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(
		infra.ExtractAuthUsers(oldSystem),
		reloadAuthorizationStore(),
		resolveCandidate,
		"old",
	))
	authenticator := aaa.WithProfileAuthorizer(&authz.LocalAuthenticator{UsersFunc: liveAcceptedLocalUsers}, nil)
	authorizer := liveLocalAuthorizer{}
	_, err = authenticator.Authenticate(aaa.AuthRequest{Username: "operator", Password: "old-pass"})
	require.NoError(t, err)
	require.True(t, authorizer.Authorize("operator", "", "show version", true))

	rest := &recordingAuthServer{authenticated: true}
	lm := newListenerMigrator()
	lm.setREST(rest)
	lm.markAuthenticated(svcREST)
	lm.setAuthReloader(svcREST, func(*zeconfig.Tree) (authIntent, bool, error) {
		staged := liveAcceptedAPIAuthentication()
		assert.True(t, staged.Required)
		assert.Nil(t, staged.Authenticate, "staging must admit neither accepted nor candidate API credentials")
		_, candidateErr := authenticator.Authenticate(aaa.AuthRequest{Username: "operator", Password: "new-pass"})
		assert.Error(t, candidateErr, "candidate local credentials must stay invisible while reload remains fallible")
		_, oldErr := authenticator.Authenticate(aaa.AuthRequest{Username: "operator", Password: "old-pass"})
		assert.NoError(t, oldErr, "non-API local authentication remains on the accepted generation")
		return authIntent{authenticated: true}, true, nil
	})
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	require.NoError(t, runReload(srv, cp, load, lm))
	_, err = authenticator.Authenticate(aaa.AuthRequest{Username: "operator", Password: "new-pass"})
	require.NoError(t, err, "candidate credentials must become live after final publication")
	_, err = authenticator.Authenticate(aaa.AuthRequest{Username: "operator", Password: "old-pass"})
	assert.Error(t, err, "the replaced credential must stop authenticating")
	assert.False(t, authorizer.Authorize("operator", "", "show version", true),
		"the candidate policy must become live with its credentials")
}

// VALIDATES: a reload that omits environment.api-server still publishes local
// user authentication from the accepted system configuration.
// PREVENTS: treating an absent API listener block as an instruction to disable
// user credentials for a listener started by flags or another source.
func TestRunReloadWithoutAPIBlockKeepsUserAuthentication(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))
	hash, err := bcrypt.GenerateFromPassword([]byte("operator-pass"), bcrypt.MinCost)
	require.NoError(t, err)

	tree := withReloadIdentity(map[string]any{}, string(hash), "allow", true)
	srv, cp := reloadDriver(t, tree)
	system := reloadIdentitySystem(string(hash), "allow", true)
	cp.SetRoot("system", system)
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(
		nil,
		reloadAuthorizationStore(),
		resolveCandidate,
		"old-token",
	))

	_, ok := liveAcceptedAPIAuthentication().Authenticate("Bearer old-token")
	require.True(t, ok)
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}
	require.NoError(t, runReload(srv, cp, load, nil))

	_, ok = liveAcceptedAPIAuthentication().Authenticate("Bearer old-token")
	assert.False(t, ok, "an omitted API block must not preserve its prior token")
	caller, ok := liveAcceptedAPIAuthentication().Authenticate("Bearer operator:operator-pass")
	require.True(t, ok, "local user authentication must survive an omitted API block")
	assert.Equal(t, "operator", caller.Username)
}

// VALIDATES: an accepted reload publishes the exact configured API token and
// replaces the token from the prior accepted generation.
// PREVENTS: publishing only the candidate authentication mode while requests
// continue authenticating against stale credential material.
func TestRunReloadPublishesExactCandidateAPIToken(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	tree := apiTokenTree("candidate-token").ToMap()
	tree["system"] = map[string]any{"authentication": map[string]any{}}
	srv, cp := reloadDriver(t, tree)
	cp.SetRoot("system", map[string]any{"authentication": map[string]any{}})
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, resolveCandidate, "accepted-token"))
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	require.NoError(t, runReload(srv, cp, load, nil))
	authentication := liveAcceptedAPIAuthentication()
	caller, ok := authentication.Authenticate("Bearer candidate-token")
	require.True(t, ok, "the exact candidate token must authenticate after acceptance")
	assert.Equal(t, aaa.ReservedSharedAPIUsername, caller.Username)
	_, ok = authentication.Authenticate("Bearer accepted-token")
	assert.False(t, ok, "the replaced token must stop authenticating")
	_, ok = authentication.Authenticate("Bearer candidate-token-suffix")
	assert.False(t, ok, "publication must not accept a token other than the exact candidate")
}

// VALIDATES: a live-user source error rejects the reload and leaves the
// previously accepted API credentials authenticating.
// PREVENTS: a failed candidate user read replacing accepted credentials with a
// partial token-only or fail-open identity.
func TestRunReloadUserSourceErrorPreservesAcceptedAPICredentials(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	tree := apiTokenTree("candidate-token").ToMap()
	tree["system"] = map[string]any{"authentication": map[string]any{}}
	srv, cp := reloadDriver(t, tree)
	resolveCandidate := func() ([]authz.UserConfig, error) {
		return nil, assert.AnError
	}
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, resolveCandidate, "accepted-token"))
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	err := runReload(srv, cp, load, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "resolve candidate local identity")
	authentication := liveAcceptedAPIAuthentication()
	_, ok := authentication.Authenticate("Bearer accepted-token")
	require.True(t, ok, "a rejected user-source read must preserve the accepted token")
	_, ok = authentication.Authenticate("Bearer candidate-token")
	assert.False(t, ok, "the rejected candidate token must never authenticate")
}

// VALIDATES: an invalid effective API listen setting rejects the reload,
// clears the staged candidate, and preserves the accepted API credentials.
// PREVENTS: the API identity resolution error path leaving a candidate wedged
// after rollback while the daemon continues serving the accepted generation.
func TestRunReloadInvalidAPIListenClearsCandidate(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	require.NoError(t, env.Set("ze.api-server.rest.enabled", "1"))
	require.NoError(t, env.Set("ze.api-server.rest.listen", "invalid-listen"))
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	tree := map[string]any{
		"system": map[string]any{"authentication": map[string]any{}},
	}
	srv, cp := reloadDriver(t, tree)
	cp.SetRoot("system", map[string]any{"authentication": map[string]any{}})
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, resolveCandidate, "accepted-token"))

	dir := t.TempDir()
	configPath := filepath.Join(dir, "router.conf")
	store := storage.NewFilesystem()
	_, err := storage.WriteCandidateVersion(store, configPath, []byte("candidate"), mustParseReloadStamp(t, "20260817-120000.000"))
	require.NoError(t, err)
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	err = runReloadContext(t.Context(), srv, nil, cp, store, configPath, load, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve candidate API identity")
	_, _, candidateExists, readErr := storage.ReadCandidateConfig(store, configPath)
	require.NoError(t, readErr)
	assert.False(t, candidateExists, "a rejected invalid API setting must clear the staged candidate")
	_, ok := liveAcceptedAPIAuthentication().Authenticate("Bearer accepted-token")
	require.True(t, ok, "the rejected API settings must preserve accepted credentials")
}

// VALIDATES: a valid candidate user source returning no users and an explicit
// API block with no token publishes no-auth mode.
// PREVENTS: treating an empty user set as a source failure and retaining stale
// authentication after the candidate is accepted.
func TestRunReloadEmptyUsersSelectNoAPIAuthentication(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	tree := apiTokenTree("").ToMap()
	tree["system"] = map[string]any{"authentication": map[string]any{}}
	srv, cp := reloadDriver(t, tree)
	cp.SetRoot("system", map[string]any{"authentication": map[string]any{}})
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, resolveCandidate, "accepted-token"))
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	require.NoError(t, runReload(srv, cp, load, nil))
	authentication := liveAcceptedAPIAuthentication()
	assert.False(t, authentication.Required, "valid empty users and no token must select no-auth mode")
	assert.Nil(t, authentication.Authenticate)
}

// VALIDATES: configured users take precedence over the shared API token after
// an accepted reload.
// PREVENTS: publishing both credential forms and leaving a shared-token bypass
// around per-user identity and authorization.
func TestRunReloadConfiguredUsersTakePrecedenceOverSharedToken(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	require.NoError(t, env.Set("ze.api-server.token", "shared-token"))
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))
	hash, err := bcrypt.GenerateFromPassword([]byte("operator-pass"), bcrypt.MinCost)
	require.NoError(t, err)

	tree := withReloadIdentity(apiTokenTree("").ToMap(), string(hash), "allow", true)
	srv, cp := reloadDriver(t, tree)
	cp.SetRoot("system", map[string]any{"authentication": map[string]any{}})
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, resolveCandidate, "accepted-token"))
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	require.NoError(t, runReload(srv, cp, load, nil))
	authentication := liveAcceptedAPIAuthentication()
	caller, ok := authentication.Authenticate("Bearer operator:operator-pass")
	require.True(t, ok, "the configured user must authenticate after acceptance")
	assert.Equal(t, "operator", caller.Username)
	_, ok = authentication.Authenticate("Bearer shared-token")
	assert.False(t, ok, "configured users must disable the shared-token credential")
}

// VALIDATES: when a running API listener came from flags or environment, a
// reload with no environment.api-server block retains its accepted token.
// PREVENTS: the identity publisher stripping credentials from a listener the
// listener migrator deliberately keeps running.
func TestRunReloadWithoutAPIBlockPreservesAcceptedAPIToken(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	tree := map[string]any{
		"system": map[string]any{"authentication": map[string]any{}},
	}
	srv, cp := reloadDriver(t, tree)
	cp.SetRoot("system", map[string]any{"authentication": map[string]any{}})
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, resolveCandidate, "accepted-token"))
	lm := newListenerMigrator()
	rest := &recordingAuthServer{
		addrs:         []string{"192.0.2.10:8081"},
		authenticated: true,
	}
	lm.setREST(rest)
	lm.markAuthenticated(svcREST)
	registerMgmtAuthReloaders(lm, mgmtAuthInputs{apiCandidateUsers: resolveCandidate})
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	require.NoError(t, runReload(srv, cp, load, lm))
	assert.Empty(t, rest.calls, "an absent API block must leave the non-loopback listener in place")
	authentication := liveAcceptedAPIAuthentication()
	caller, ok := authentication.Authenticate("Bearer accepted-token")
	require.True(t, ok, "an absent API block must preserve credentials for the running listener")
	assert.Equal(t, aaa.ReservedSharedAPIUsername, caller.Username)
}

// VALIDATES: a reload rejected after listener credentials were applied keeps
// both credentials and policy from the prior accepted generation.
func TestRunReloadFailurePreservesAcceptedIdentity(t *testing.T) {
	resetAAABundleForTest(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-pass"), bcrypt.MinCost)
	require.NoError(t, err)
	newHash, err := bcrypt.GenerateFromPassword([]byte("new-pass"), bcrypt.MinCost)
	require.NoError(t, err)

	caB64, certB64, keyB64 := caSignedB64(t, "authz rollback leaf")
	tree := withReloadIdentity(
		pkiWebTree(caB64, certB64, keyB64, "authz-rollback-cert", "authz-rollback-cert"),
		string(newHash),
		"deny",
		true,
	)
	srv, cp := reloadDriver(t, tree)
	oldSystem := reloadIdentitySystem(string(oldHash), "allow", true)
	cp.SetRoot("system", oldSystem)
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(
		infra.ExtractAuthUsers(oldSystem),
		reloadAuthorizationStore(),
		resolveCandidate,
		"original",
	))
	authenticator := aaa.WithProfileAuthorizer(&authz.LocalAuthenticator{UsersFunc: liveAcceptedLocalUsers}, nil)
	authorizer := liveLocalAuthorizer{}

	lm := newListenerMigrator()
	lm.setWebTLS(&fakeTLSUpdatable{err: assert.AnError})
	lm.setREST(&recordingAuthServer{
		addrs:         []string{"127.0.0.1:8081"},
		authenticated: true,
	})
	lm.markAuthenticated(svcREST)
	lm.setAuthReloader(svcREST, staticAuth(true, "candidate"))
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	require.Error(t, runReload(srv, cp, load, lm))
	_, err = authenticator.Authenticate(aaa.AuthRequest{Username: "operator", Password: "old-pass"})
	require.NoError(t, err, "failed reload must preserve the accepted credential")
	_, err = authenticator.Authenticate(aaa.AuthRequest{Username: "operator", Password: "new-pass"})
	assert.Error(t, err, "failed reload must never expose the candidate credential")
	assert.True(t, authorizer.Authorize("operator", "", "show version", true),
		"failed reload must preserve the accepted authorization policy")
}

// VALIDATES: a reload rejected by the late live-certificate update restores
// the original accepted API token after listener rollback succeeds.
// PREVENTS: the fail-closed staging generation becoming permanent after a late
// rejection even though the accepted listener addresses were restored.
func TestRunReloadCertificateFailurePreservesAcceptedAPIToken(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	caB64, certB64, keyB64 := caSignedB64(t, "api token rollback leaf")
	tree := pkiWebTree(caB64, certB64, keyB64, "api-token-rollback-cert", "api-token-rollback-cert")
	apiEnvironment, ok := apiTokenTree("candidate-token").ToMap()["environment"].(map[string]any)
	require.True(t, ok)
	treeEnvironment, ok := tree["environment"].(map[string]any)
	require.True(t, ok)
	treeEnvironment["api-server"] = apiEnvironment["api-server"]
	tree["system"] = map[string]any{"authentication": map[string]any{}}
	srv, cp := reloadDriver(t, tree)
	cp.SetRoot("system", map[string]any{"authentication": map[string]any{}})
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, resolveCandidate, "accepted-token"))

	lm := newListenerMigrator()
	lm.setWebTLS(&fakeTLSUpdatable{err: assert.AnError})
	lm.setREST(&recordingAuthServer{
		addrs:         []string{"0.0.0.0:8081"},
		authenticated: true,
	})
	lm.markAuthenticated(svcREST)
	lm.setAuthReloader(svcREST, staticAuth(true, "candidate-token"))
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	require.Error(t, runReload(srv, cp, load, lm))
	authentication := liveAcceptedAPIAuthentication()
	caller, ok := authentication.Authenticate("Bearer accepted-token")
	require.True(t, ok, "late certificate rejection must restore the original API token")
	assert.Equal(t, aaa.ReservedSharedAPIUsername, caller.Username)
	_, ok = authentication.Authenticate("Bearer candidate-token")
	assert.False(t, ok, "the rejected candidate token must never authenticate")
}

// VALIDATES: accepted API credentials are not restored when listener rollback
// fails after a rejected reload.
// PREVENTS: a prior credential becoming valid on an address from the rejected
// candidate configuration.
func TestRunReloadListenerRollbackFailureStaysFailClosed(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	treeValue := restOnlyTree("127.0.0.1", "8082")
	apiBlock := treeValue.GetContainer("environment")
	require.NotNil(t, apiBlock)
	apiServer := apiBlock.GetContainer("api-server")
	require.NotNil(t, apiServer)
	apiServer.Set("token", "candidate-token")
	tree := treeValue.ToMap()
	srv, cp := reloadDriver(t, tree)
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, resolveCandidate, "accepted-token"))

	listener := &rollbackFailReconfigurable{addrs: []string{"127.0.0.1:8081"}}
	lm := newListenerMigrator()
	lm.setREST(listener)
	lm.markAuthenticated(svcREST)
	lm.setAuthReloader(svcREST, staticAuth(true, "candidate-token"))

	dir := t.TempDir()
	configPath := filepath.Join(dir, "router.conf")
	baseStore := storage.NewFilesystem()
	_, err := storage.WriteCandidateVersion(baseStore, configPath, []byte("candidate"), mustParseReloadStamp(t, "20260817-100000.000"))
	require.NoError(t, err)
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeValue, nil
	}

	err = runReloadContext(t.Context(), srv, nil, cp, failAcquireStorage{Storage: baseStore}, configPath, load, lm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "promote candidate")
	assert.Equal(t, 2, listener.calls, "listener migration and its failed rollback must both run")
	authentication := liveAcceptedAPIAuthentication()
	assert.True(t, authentication.Required)
	assert.Nil(t, authentication.Authenticate, "failed listener rollback must leave API authentication fail closed")
}

// VALIDATES: a partial listener migration whose internal rollback and outer
// retry both fail keeps API authentication in staging.
// PREVENTS: reloadListeners returning a no-op undo on its error path, which
// restores accepted credentials while the API still serves a rejected address.
func TestRunReloadInternalAndRetryListenerRollbackFailureStaysFailClosed(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	treeValue := apiTokenTree("candidate-token")
	apiServer := treeValue.GetContainer("environment").GetContainer("api-server")
	require.NotNil(t, apiServer)
	restServer := zeconfig.NewTree()
	restServer.Set("ip", "127.0.0.1")
	restServer.Set("port", "8082")
	apiServer.GetContainer("rest").AddListEntry("server", "main", restServer)
	grpcServer := zeconfig.NewTree()
	grpcServer.Set("ip", "127.0.0.1")
	grpcServer.Set("port", "50052")
	apiServer.GetContainer("grpc").AddListEntry("server", "main", grpcServer)
	system := zeconfig.NewTree()
	system.SetContainer("authentication", zeconfig.NewTree())
	treeValue.SetContainer("system", system)
	tree := treeValue.ToMap()
	srv, cp := reloadDriver(t, tree)
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, resolveCandidate, "accepted-token"))

	rest := &retryRollbackFailReconfigurable{addrs: []string{"127.0.0.1:8081"}}
	grpc := &recordingAuthServer{
		addrs:         []string{"127.0.0.1:50051"},
		fail:          assert.AnError,
		authenticated: true,
	}
	lm := newListenerMigrator()
	lm.setREST(rest)
	lm.setGRPC(grpc)
	lm.markAuthenticated(svcREST)
	lm.markAuthenticated(svcGRPC)
	lm.setAuthReloader(svcREST, staticAuth(true, "candidate-token"))
	lm.setAuthReloader(svcGRPC, staticAuth(true, "candidate-token"))
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeValue, nil
	}

	err := runReload(srv, cp, load, lm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reconfigure grpc")
	assert.Contains(t, err.Error(), "listener rollback failed")
	assert.Equal(t, 3, rest.calls,
		"migration, internal rollback, and outer retry must all run")
	assert.Equal(t, []string{"127.0.0.1:8082"}, rest.addrs,
		"both failed rollback attempts leave the REST listener on the candidate address")
	authentication := liveAcceptedAPIAuthentication()
	assert.True(t, authentication.Required)
	assert.Nil(t, authentication.Authenticate,
		"accepted credentials must stay hidden while listener restoration is incomplete")
}

// VALIDATES: after an accepted reload removes a user's profile assignment, the
// next successful profile-less authentication replaces the prior login result
// with empty state and authorization denies the removed grant.
// PREVENTS: a username retaining a profile or recovery grant from an earlier
// successful authentication after its accepted credential becomes profile-less.
func TestRunReloadRemovedAssignmentDeniedAfterReauthentication(t *testing.T) {
	resetAAABundleForTest(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))
	hash, err := bcrypt.GenerateFromPassword([]byte("operator-pass"), bcrypt.MinCost)
	require.NoError(t, err)

	tree := withReloadIdentity(map[string]any{}, string(hash), "allow", false)
	srv, cp := reloadDriver(t, tree)
	oldSystem := reloadIdentitySystem(string(hash), "allow", true)
	cp.SetRoot("system", oldSystem)
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(
		infra.ExtractAuthUsers(oldSystem),
		reloadAuthorizationStore(),
		resolveCandidate,
		"",
	))
	authenticator := aaa.WithProfileAuthorizer(&authz.LocalAuthenticator{UsersFunc: liveAcceptedLocalUsers}, nil)
	authorizer := liveLocalAuthorizer{}
	_, err = authenticator.Authenticate(aaa.AuthRequest{Username: "operator", Password: "operator-pass"})
	require.NoError(t, err)
	require.True(t, authorizer.Authorize("operator", "", "show version", true))

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}
	require.NoError(t, runReload(srv, cp, load, nil))
	assert.False(t, authorizer.Authorize("operator", "", "show version", true),
		"accepted publication must revoke an assignment from the prior generation")

	_, err = authenticator.Authenticate(aaa.AuthRequest{Username: "operator", Password: "operator-pass"})
	require.NoError(t, err, "the profile-less credential remains valid")
	assert.False(t, authorizer.Authorize("operator", "", "show version", true))
}

// TestReloadHashesPlaintextPassword: SIGHUP inherits the password transform.
//
// VALIDATES: spec-ssh-optional-composition AC-7 -- the reload path has no branch of
// its own. diskConfigLoaders (main_reload.go) is the loader the daemon installs
// for SIGHUP, it goes through zeconfig.LoadConfig, and the tree it hands
// runReload carries the bcrypt hash rather than the operator's plaintext.
//
// PREVENTS: a fix applied to boot only. Hashing at start and not at reload
// would lock every user out on the first SIGHUP after they set a password,
// which is worse than the defect being fixed. Driving diskConfigLoaders rather
// than a load closure written here is what makes this a wiring test: a
// hand-built closure would prove only that the test can call LoadConfig.
func TestReloadHashesPlaintextPassword(t *testing.T) {
	t.Cleanup(func() { _ = zepki.Load(nil) })

	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "router.conf")
	config := `system {
	authentication {
		user lab {
			plaintext-password "labsecret";
		}
	}
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

	_, loadBoth := diskConfigLoaders(store, configPath, nil)

	treeMap, tree, err := loadBoth()
	require.NoError(t, err)

	lab := tree.GetContainer("system").GetContainer("authentication").GetList("user")["lab"]
	require.NotNil(t, lab)
	hash, ok := lab.Get("password")
	require.True(t, ok, "the reload loader must populate the canonical password leaf")
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("labsecret")))
	_, plainOK := lab.Get("plaintext-password")
	assert.False(t, plainOK, "the ephemeral plaintext leaf must not survive the reload")

	// The map the plugin runtime receives carries the same hash, so a plugin
	// reading credentials after a SIGHUP sees the hashed leaf, not an empty one.
	assert.Equal(t, hash, reloadUserPassword(t, treeMap, "lab"))

	reactor := &reloadTestReactor{tree: map[string]any{}}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	require.NoError(t, doReload(server, nil, nil, store, configPath, loadBoth, nil))
	installed := reloadUserPassword(t, reactor.setTree, "lab")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(installed), []byte("labsecret")),
		"the tree the reload installed carries a hash of the operator's password")
	// R-6, measured here rather than assumed: bcrypt salts randomly, so each
	// load of the same file yields a DIFFERENT hash. The credential is the same
	// one either way; what it costs is the plugin-server diff seeing `system`
	// change on every SIGHUP, which the spec accepted for a config that carries
	// a plaintext password at all.
	assert.NotEqual(t, hash, installed, "each load re-salts, so the two hashes differ")
}

// reloadUserPassword reads system.authentication.user.<name>.password out of a
// tree map, the shape Tree.ToMap produces (a list is a map keyed by list key).
func reloadUserPassword(t *testing.T, tree map[string]any, name string) string {
	t.Helper()
	system, ok := tree["system"].(map[string]any)
	require.True(t, ok, "tree carries a system container")
	auth, ok := system["authentication"].(map[string]any)
	require.True(t, ok, "system carries authentication")
	users, ok := auth["user"].(map[string]any)
	require.True(t, ok, "authentication carries a user list")
	entry, ok := users[name].(map[string]any)
	require.True(t, ok, "the user list carries %q", name)
	password, ok := entry["password"].(string)
	require.True(t, ok, "user %q carries a password leaf", name)
	return password
}

// recoveryZefsUsers returns the power user `ze init` writes to zefs: the one
// account that carries the reserved recovery profile, and therefore the only
// one whose live session the accepted generation can revoke.
func recoveryZefsUsers(hash string) []authz.UserConfig {
	return []authz.UserConfig{{
		Name:     "recovery-admin",
		Hash:     hash,
		Profiles: []string{aaa.ReservedRecoveryProfile},
	}}
}

// VALIDATES: a reload that re-parses an UNCHANGED configuration republishes the
// SAME accepted generation, so a live break-glass session keeps its authority.
// The two credential sets compared here are produced INDEPENDENTLY, as
// production produces them: the accepted set from the map the boot provider
// held, the candidate set from the map the reload's own parse installs, each
// through infra.ExtractAuthUsers and mergeAuthUsers.
// PREVENTS: an operator's own web config commit revoking, inside its own
// request, the session that issued it. Every reload advanced the counter, a
// bare SIGHUP included, so the commit bar came back read-only and every later
// edit answered 403 until the operator logged in again.
func TestRunReloadReusesGenerationWhenNoCredentialChanged(t *testing.T) {
	resetAAABundleForTest(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))
	hash, err := bcrypt.GenerateFromPassword([]byte("operator-pass"), bcrypt.MinCost)
	require.NoError(t, err)

	tree := withReloadIdentity(map[string]any{}, string(hash), "allow", true)
	srv, cp := reloadDriver(t, tree)
	// An EQUAL but distinct map. The accepted extraction reads this one; the
	// candidate extraction reads the root the reload installs from its own
	// parse, so nothing but the credential values can make the two agree.
	cp.SetRoot("system", reloadIdentitySystem(string(hash), "allow", true))
	resolveCandidate := liveLocalUsers(recoveryZefsUsers(string(hash)), func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	bootUsers, err := resolveCandidate()
	require.NoError(t, err)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(
		bootUsers,
		reloadAuthorizationStore(),
		resolveCandidate,
		"",
	))

	accepted := acceptedLocalIdentity.Load().generation
	session := recoverySessionAuthorizer(t, "recovery-admin")
	require.True(t, session.Authorize("recovery-admin", "", "config commit", false))

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}
	require.NoError(t, runReload(srv, cp, load, nil))

	assert.Equal(t, accepted, acceptedLocalIdentity.Load().generation,
		"a reload that re-parses the same credentials must republish the same generation")
	assert.True(t, session.Authorize("recovery-admin", "", "config commit", false),
		"a live break-glass session must survive a reload that changed no credential")
}

// VALIDATES: a reload that changes one configured credential still advances the
// accepted generation and revokes the live break-glass session.
// PREVENTS: the reuse above becoming a grant no password rotation, profile
// demotion, or account removal can take back.
func TestRunReloadAdvancesGenerationWhenACredentialChanged(t *testing.T) {
	resetAAABundleForTest(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-pass"), bcrypt.MinCost)
	require.NoError(t, err)
	newHash, err := bcrypt.GenerateFromPassword([]byte("new-pass"), bcrypt.MinCost)
	require.NoError(t, err)

	tree := withReloadIdentity(map[string]any{}, string(newHash), "allow", true)
	srv, cp := reloadDriver(t, tree)
	cp.SetRoot("system", reloadIdentitySystem(string(oldHash), "allow", true))
	resolveCandidate := liveLocalUsers(recoveryZefsUsers(string(oldHash)), func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	bootUsers, err := resolveCandidate()
	require.NoError(t, err)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(
		bootUsers,
		reloadAuthorizationStore(),
		resolveCandidate,
		"",
	))

	accepted := acceptedLocalIdentity.Load().generation
	session := recoverySessionAuthorizer(t, "recovery-admin")
	require.True(t, session.Authorize("recovery-admin", "", "config commit", false))

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}
	require.NoError(t, runReload(srv, cp, load, nil))

	assert.NotEqual(t, accepted, acceptedLocalIdentity.Load().generation,
		"a rotated password must publish a new generation")
	assert.False(t, session.Authorize("recovery-admin", "", "config commit", false),
		"the session pinned to the replaced generation must lose its authority")
}
