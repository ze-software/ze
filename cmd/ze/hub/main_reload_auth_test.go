// Design: ai/rules/evidence.md -- the exposure guard, re-run on every reload
// Related: main_reload.go -- runReload, the driver these tests exercise
// Related: listener_migrate.go -- reloadListeners returns the undo runReload runs

package hub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
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

func withReloadAuthorization(tree map[string]any, action string) map[string]any {
	tree["system"] = map[string]any{
		"authentication": map[string]any{
			"user": map[string]any{
				"operator": map[string]any{"profile": "reload-policy"},
			},
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
	return tree
}

func reloadAuthorizationStore(action authz.Action) *authz.Store {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "reload-policy",
		Run:  authz.Section{Default: action},
		Edit: authz.Section{Default: action},
	})
	store.AssignProfiles("operator", []string{"reload-policy"})
	return store
}

// VALIDATES: a fully successful reload atomically publishes the candidate
// profile and assignment store to an authorizer that already serves requests.
func TestRunReloadSuccessfulReloadSwapsLiveAuthorization(t *testing.T) {
	resetAAABundleForTest(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))
	swapLocalAuthzStore(reloadAuthorizationStore(authz.Allow))
	authorizer := liveLocalAuthorizer{}
	require.True(t, authorizer.Authorize("operator", "", "show version", true))

	tree := withReloadAuthorization(map[string]any{}, "deny")
	srv, cp := reloadDriver(t, tree)
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	require.NoError(t, runReload(srv, nil, cp, nil, "", load, nil))
	assert.False(t, authorizer.Authorize("operator", "", "show version", true))
}

// VALIDATES: a reload rejected after listener credentials were applied and
// rolled back does not publish its candidate authorization store.
func TestRunReloadFailedReloadPreservesLiveAuthorization(t *testing.T) {
	resetAAABundleForTest(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))
	swapLocalAuthzStore(reloadAuthorizationStore(authz.Allow))
	authorizer := liveLocalAuthorizer{}
	require.True(t, authorizer.Authorize("operator", "", "show version", true))

	caB64, certB64, keyB64 := caSignedB64(t, "authz rollback leaf")
	tree := withReloadAuthorization(
		pkiWebTree(caB64, certB64, keyB64, "authz-rollback-cert", "authz-rollback-cert"),
		"deny",
	)
	srv, cp := reloadDriver(t, tree)
	lm := newListenerMigrator()
	lm.setWebTLS(&fakeTLSUpdatable{err: assert.AnError})
	lm.setREST(&recordingAuthServer{
		recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}},
		token:                   "original",
	})
	lm.markAuthenticated("rest")
	lm.setAuthReloader("rest", staticAuth(true, "candidate"))
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	require.Error(t, runReload(srv, nil, cp, nil, "", load, lm))
	assert.True(t, authorizer.Authorize("operator", "", "show version", true),
		"failed reload must retain the previously installed authorization store")
}

// VALIDATES: runReload runs the credential undo when a step AFTER
// reloadListeners fails, so a reload the operator is told failed does not leave
// the API servers authenticating against the config that was rolled back.
// PREVENTS: the second call site of restoreAuth being unwired. reloadListeners
// returning an undo proves nothing on its own; this drives runReload itself and
// asserts the credentials the reload installed are gone afterwards.
func TestRunReloadUndoesCredentialsWhenCertificateRotationFails(t *testing.T) {
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	caB64, certB64, keyB64 := caSignedB64(t, "auth undo leaf")
	tree := pkiWebTree(caB64, certB64, keyB64, "rotate-cert", "rotate-cert")

	srv, cp := reloadDriver(t, tree)

	// The certificate resolves, so runReload passes its gate and reaches the
	// rotation, where the seam refuses.
	failingTLS := &fakeTLSUpdatable{err: assert.AnError}
	rest := &recordingAuthServer{
		recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}},
		token:                   "original",
	}
	lm := newListenerMigrator()
	lm.setWebTLS(failingTLS)
	lm.setREST(rest)
	lm.markAuthenticated("rest")
	lm.setAuthReloader("rest", staticAuth(false, ""))

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	err := runReload(srv, nil, cp, nil, "", load, lm)
	require.Error(t, err, "the rotation seam refused, so the reload failed")
	require.Positive(t, failingTLS.calls, "the reload must have reached the rotation step")

	// The reload DID rebuild the credentials on its way through, and the failure
	// put them back. Both halves matter: without the first, an implementation
	// that never rebuilt anything would pass this test.
	assert.Equal(t, []string{""}, rest.updates, "the reload rebuilt REST authentication")
	assert.True(t, rest.Authenticated(), "a failed reload must not leave REST unauthenticated")
	assert.Equal(t, "original", rest.token)
}

// VALIDATES: a reload that completes keeps the credentials it installed.
// PREVENTS: a fix that undoes the credentials unconditionally, which would make
// every successful reload a no-op for authentication -- the original defect
// restored by the rollback path.
func TestRunReloadKeepsCredentialsWhenReloadSucceeds(t *testing.T) {
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	caB64, certB64, keyB64 := caSignedB64(t, "auth keep leaf")
	tree := pkiWebTree(caB64, certB64, keyB64, "keep-cert", "keep-cert")

	srv, cp := reloadDriver(t, tree)

	rest := &recordingAuthServer{
		recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}},
		token:                   "original",
	}
	lm := newListenerMigrator()
	lm.setWebTLS(&fakeTLSUpdatable{})
	lm.setREST(rest)
	lm.markAuthenticated("rest")
	lm.setAuthReloader("rest", staticAuth(true, "reloaded"))

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	require.NoError(t, runReload(srv, nil, cp, nil, "", load, lm))
	assert.Equal(t, "reloaded", rest.token, "a reload that completed must keep what it installed")
}

// TestReloadHashesPlaintextPassword: SIGHUP inherits the password transform.
//
// VALIDATES: spec-netlab-integration AC-2 -- the reload path has no branch of
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
