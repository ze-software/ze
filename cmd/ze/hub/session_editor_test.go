package hub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// sessionEditorStore writes a config and opens the blob storage over it.
func sessionEditorStore(t *testing.T) (storage.Storage, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("set system host-name test\n"), 0o600))

	store, err := storage.NewBlob(filepath.Join(dir, "database.zefs"), dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store, configPath
}

// TestSessionEditorHasReloadNotifier: the session editor must be built with a
// reload notifier. cmdCommitSession then takes the transactional
// CommitSessionCandidate + NotifyReload branch, and a session commit reaches
// the running daemons.
//
// VALIDATES: AC-8 "SSH session editor built by buildSessionModelFactory has
// HasReloadNotifier() true".
// PREVENTS: commits writing config.conf without reloading the daemon.
func TestSessionEditorHasReloadNotifier(t *testing.T) {
	store, configPath := sessionEditorStore(t)

	called := false
	reload := func() error {
		called = true
		return nil
	}

	ed, err := newSessionEditor(store, configPath, "thomas", sessionOriginSSH, reload)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })

	assert.True(t, ed.HasReloadNotifier(),
		"the session editor must be wired with the reload notifier")
	require.NoError(t, ed.NotifyReload())
	assert.True(t, called, "NotifyReload must invoke the wired reload function")
}

// TestSessionEditorWithoutReloadFn: a nil reload function leaves the editor
// without a notifier (web-only / standalone semantics preserved).
//
// VALIDATES: nil reload wiring degrades to the no-notifier editor.
// PREVENTS: a nil function pointer masquerading as a configured notifier.
func TestSessionEditorWithoutReloadFn(t *testing.T) {
	store, configPath := sessionEditorStore(t)

	ed, err := newSessionEditor(store, configPath, "thomas", sessionOriginSSH, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })

	assert.False(t, ed.HasReloadNotifier(),
		"nil reload function must not register a notifier")
}

// TestSessionEditorStampsOrigin: the origin the caller gives is the origin the
// edit session carries. Two surfaces share this constructor, and the draft
// metadata tells their changes apart by "user@origin".
//
// VALIDATES: the attached console and the SSH server stamp different origins.
// PREVENTS: a local edit appearing in the draft as an SSH edit.
func TestSessionEditorStampsOrigin(t *testing.T) {
	for _, origin := range []string{sessionOriginSSH, sessionOriginLocal} {
		t.Run(origin, func(t *testing.T) {
			store, configPath := sessionEditorStore(t)

			ed, err := newSessionEditor(store, configPath, "thomas", origin, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = ed.Close() })

			want := "thomas@" + origin + "%"
			assert.True(t, strings.HasPrefix(ed.SessionID(), want),
				"session id %q must start with %q", ed.SessionID(), want)
		})
	}
}

// TestSessionEditorRefusesInvalidUser: the username is a change-file
// identifier, so an unusable one must fail the build rather than reach the
// filesystem.
//
// VALIDATES: ValidateUser guards the editor constructor.
// PREVENTS: an empty or traversing username naming a change file.
func TestSessionEditorRefusesInvalidUser(t *testing.T) {
	store, configPath := sessionEditorStore(t)

	for _, username := range []string{"", "..", "over/there"} {
		_, err := newSessionEditor(store, configPath, username, sessionOriginLocal, nil)
		assert.Error(t, err, "username %q must be refused", username)
	}
}
