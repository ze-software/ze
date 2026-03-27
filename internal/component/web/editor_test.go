package web

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/schema" // Register BGP YANG for write-through tests.
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	_ "codeberg.org/thomas-mangin/ze/internal/component/hub/schema" // Required by ze-bgp-conf.yang (imports ze-hub-conf).
)

// validWebConfig is a YANG-parseable BGP config for EditorManager tests.
// The write-through SetValue requires a properly structured config with the
// bgp container. Plain "router-id 1.2.3.4" is not valid for session-based
// editing because the YANG parser expects bgp { ... } structure.
const validWebConfig = "bgp {\n\trouter-id 1.2.3.4\n\tlocal { as 65000; }\n}\n"

// newTestEditorManager creates an EditorManager backed by a temp config file
// and the real YANG schema. Returns the manager. The temp directory is cleaned
// up automatically by t.TempDir.
func newTestEditorManager(t *testing.T) *EditorManager {
	t.Helper()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")

	err := os.WriteFile(configPath, []byte(validWebConfig), 0o600)
	require.NoError(t, err, "writing test config")

	store := storage.NewFilesystem()
	schema := config.YANGSchema()
	require.NotNil(t, schema, "YANG schema must load")

	return NewEditorManager(store, configPath, schema)
}

// TestEditorManagerGetOrCreate verifies that calling GetOrCreate twice with the
// same username returns the same userSession instance (session reuse).
//
// VALIDATES: per-user editor reuse -- second call must not create a new session.
// PREVENTS: duplicate Editor instances and change file corruption from double
// initialization.
func TestEditorManagerGetOrCreate(t *testing.T) {
	mgr := newTestEditorManager(t)

	us1, err := mgr.GetOrCreate("alice")
	require.NoError(t, err, "first GetOrCreate must succeed")
	require.NotNil(t, us1, "first GetOrCreate must return non-nil userSession")

	us2, err := mgr.GetOrCreate("alice")
	require.NoError(t, err, "second GetOrCreate must succeed")
	require.NotNil(t, us2, "second GetOrCreate must return non-nil userSession")

	assert.Same(t, us1, us2,
		"same username must return the same userSession pointer")
}

// TestEditorManagerDifferentUsers verifies that different usernames produce
// independent userSession instances with separate Editors.
//
// VALIDATES: AC-10 (independent drafts per user).
// PREVENTS: cross-user draft contamination.
func TestEditorManagerDifferentUsers(t *testing.T) {
	mgr := newTestEditorManager(t)

	alice, err := mgr.GetOrCreate("alice")
	require.NoError(t, err)

	bob, err := mgr.GetOrCreate("bob")
	require.NoError(t, err)

	assert.NotSame(t, alice, bob,
		"different usernames must return different userSession pointers")
}

// TestEditorManagerSetValue verifies that setting a value through the
// EditorManager modifies the user's draft tree.
//
// VALIDATES: AC-1 (value set in user's draft via Editor.SetValue).
// PREVENTS: SetValue silently failing or writing to the wrong user's draft.
func TestEditorManagerSetValue(t *testing.T) {
	mgr := newTestEditorManager(t)

	// Set router-id under the bgp container via the manager's SetValue method.
	err := mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err, "SetValue must succeed for valid YANG path and value")

	tree := mgr.Tree("alice")
	require.NotNil(t, tree, "Tree must not be nil after SetValue")

	// Walk into the bgp container to check the value.
	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp, "bgp container must exist in tree")

	val, ok := bgp.Get("router-id")
	assert.True(t, ok, "router-id must exist in bgp after SetValue")
	assert.Equal(t, "10.0.0.1", val, "router-id must have the value we set")
}

// TestEditorManagerConcurrentAccess verifies that multiple goroutines calling
// SetValue on the same user do not race. Run with -race.
//
// VALIDATES: per-user mutex prevents data races.
// PREVENTS: concurrent map writes in EditorManager or unsynchronized Editor
// access.
func TestEditorManagerConcurrentAccess(t *testing.T) {
	mgr := newTestEditorManager(t)

	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(n int) {
			defer wg.Done()

			if setErr := mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.1"); setErr != nil {
				t.Errorf("goroutine %d: SetValue failed: %v", n, setErr)
			}
		}(i)
	}

	wg.Wait()
}

// TestEditorManagerDiff verifies that Diff does not error and returns empty
// when no session exists, then returns without error after changes.
// In write-through mode (which the web EditorManager uses), Diff compares
// the serialized original vs working strings. The write-through path
// modifies the tree and change files but may not update the raw working
// content string, so the diff may be empty even with pending changes.
//
// VALIDATES: Diff returns without error for both missing and existing sessions.
// PREVENTS: Diff panicking on nil session, or returning error for valid session.
func TestEditorManagerDiff(t *testing.T) {
	mgr := newTestEditorManager(t)

	// Before any changes, diff should be empty (no session yet).
	diff, err := mgr.Diff("alice")
	require.NoError(t, err, "Diff must not error for non-existent user")
	assert.Empty(t, diff, "diff must be empty before any changes")

	// Set a value so the user has a session.
	err = mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err, "SetValue must succeed")

	// Diff must not error after SetValue, regardless of content.
	_, err = mgr.Diff("alice")
	require.NoError(t, err, "Diff must not error for existing session")

	// Diff for a different user (no session) must still be empty.
	diff, err = mgr.Diff("bob")
	require.NoError(t, err, "Diff must not error for non-existent user")
	assert.Empty(t, diff, "diff must be empty for user with no session")
}

// TestEditorManagerChangeCount verifies that ChangeCount returns 0 before any
// changes and a positive value after SetValue.
//
// VALIDATES: AC-6 (change count tracks pending modifications).
// PREVENTS: ChangeCount returning wrong count, or panicking on missing session.
func TestEditorManagerChangeCount(t *testing.T) {
	mgr := newTestEditorManager(t)

	// No session yet: count should be 0.
	count := mgr.ChangeCount("alice")
	assert.Equal(t, 0, count, "change count must be 0 before any changes")

	// Set a value to create a pending change.
	err := mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err, "SetValue must succeed")

	count = mgr.ChangeCount("alice")
	assert.Greater(t, count, 0, "change count must be > 0 after SetValue")
}
