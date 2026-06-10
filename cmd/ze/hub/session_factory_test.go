package hub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

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
