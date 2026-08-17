//go:build ze_ssh

package hub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/config/infra"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestMergePluginCommandsNilSafe verifies the SSH per-session merge is a no-op
// (never panics, never mutates) when the dispatcher is not yet reachable. This
// is the early-startup race: a session factory is built before the reactor wires
// the API server, so params.APIServer / the server may be nil on the first tab.
//
// VALIDATES: R-2 -- completion degrades gracefully during the startup window.
// PREVENTS: a nil-deref crash on the SSH session goroutine at daemon start.
func TestMergePluginCommandsNilSafe(t *testing.T) {
	tree := &command.Node{Children: map[string]*command.Node{}}

	mergePluginCommands(tree, infra.HookParams{}) // APIServer field nil
	mergePluginCommands(tree, infra.HookParams{   // APIServer returns nil server
		APIServer: func() *pluginserver.Server { return nil },
	})

	if len(tree.Children) != 0 {
		t.Errorf("nil command source must not add nodes, got %v", tree.Children)
	}
}

// TestSessionEditorHasReloadNotifier: the SSH session editor must be built
// with a reload notifier so cmdCommitSession takes the transactional
// CommitSessionCandidate + NotifyReload branch and a session commit reaches
// the running daemons.
//
// VALIDATES: AC-8 "SSH session editor built by buildSessionModelFactory has
// HasReloadNotifier() true".
// PREVENTS: SSH commits writing config.conf without reloading the daemon.
func TestSessionEditorHasReloadNotifier(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("set system host-name test\n"), 0o600))

	store, err := storage.NewBlob(filepath.Join(dir, "database.zefs"), dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	called := false
	reload := func() error {
		called = true
		return nil
	}

	ed, err := newSessionEditor(store, configPath, "thomas", reload)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })

	assert.True(t, ed.HasReloadNotifier(),
		"SSH session editor must be wired with the reload notifier")
	require.NoError(t, ed.NotifyReload())
	assert.True(t, called, "NotifyReload must invoke the wired reload function")
}

// TestSessionEditorWithoutReloadFn: a nil reload function leaves the editor
// without a notifier (web-only / standalone semantics preserved).
//
// VALIDATES: nil reload wiring degrades to the no-notifier editor.
// PREVENTS: a nil function pointer masquerading as a configured notifier.
func TestSessionEditorWithoutReloadFn(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("set system host-name test\n"), 0o600))

	store, err := storage.NewBlob(filepath.Join(dir, "database.zefs"), dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	ed, err := newSessionEditor(store, configPath, "thomas", nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })

	assert.False(t, ed.HasReloadNotifier(),
		"nil reload function must not register a notifier")
}

// TestDashboardFactoryUsesPublicSummaryCommand verifies that SSH dashboard
// polling uses the registered CLI path rather than the internal RPC nickname.
//
// VALIDATES: monitor bgp reaches the live BGP summary over an SSH session.
// PREVENTS: a healthy dashboard rendering an empty peer table.
func TestDashboardFactoryUsesPublicSummaryCommand(t *testing.T) {
	var command string
	factory := dashboardFactoryFromExecutor(func(input string) (*plugin.RenderedResponse, error) {
		command = input
		return &plugin.RenderedResponse{Output: `{"summary":{"peers-configured":3}}`}, nil
	})

	poller, err := factory()
	require.NoError(t, err)
	output, err := poller()
	require.NoError(t, err)

	assert.Equal(t, "show bgp summary", command)
	assert.JSONEq(t, `{"summary":{"peers-configured":3}}`, output)
}
