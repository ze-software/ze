// Design: ai/rules/evidence.md -- the exposure guard, re-run on every reload
// Related: main_reload.go -- runReload, the driver these tests exercise
// Related: listener_migrate.go -- ReloadListeners returns the undo runReload runs

package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
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

// VALIDATES: runReload runs the credential undo when a step AFTER
// ReloadListeners fails, so a reload the operator is told failed does not leave
// the API servers authenticating against the config that was rolled back.
// PREVENTS: the second call site of restoreAuth being unwired. ReloadListeners
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
	lm := NewListenerMigrator(nil)
	lm.SetWebTLS(failingTLS)
	lm.SetREST(rest)
	lm.MarkAuthenticated("rest")
	lm.SetAuthReloader("rest", staticAuth(false, ""))

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
	lm := NewListenerMigrator(nil)
	lm.SetWebTLS(&fakeTLSUpdatable{})
	lm.SetREST(rest)
	lm.MarkAuthenticated("rest")
	lm.SetAuthReloader("rest", staticAuth(true, "reloaded"))

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	require.NoError(t, runReload(srv, nil, cp, nil, "", load, lm))
	assert.Equal(t, "reloaded", rest.token, "a reload that completed must keep what it installed")
}
