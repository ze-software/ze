package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// TestDropPlaintextPasswordEntries: the helper drops only entries whose
// final path segment starts with "plaintext-"; preserves order; returns
// a fresh slice (input is not mutated).
//
// VALIDATES: orphan-metadata cleanup after ApplyPasswordHashing.
// PREVENTS: commit metadata referencing a leaf the hash hook removed.
func TestDropPlaintextPasswordEntries(t *testing.T) {
	in := []config.SessionEntry{
		{Path: "system authentication user alice plaintext-password"},
		{Path: "bgp local-as"},
		{Path: "system authentication user bob plaintext-password"},
		{Path: "system authentication user alice profile"},
	}
	original := make([]config.SessionEntry, len(in))
	copy(original, in)

	out := dropPlaintextPasswordEntries(in)

	assert.Len(t, out, 2)
	assert.Equal(t, "bgp local-as", out[0].Path)
	assert.Equal(t, "system authentication user alice profile", out[1].Path)

	// Input slice must not have been mutated (we returned a fresh allocation).
	for i, se := range in {
		assert.Equal(t, original[i].Path, se.Path,
			"input slice modified at index %d", i)
	}
}

// TestDropPlaintextPasswordEntriesEmpty: empty input returns empty output.
func TestDropPlaintextPasswordEntriesEmpty(t *testing.T) {
	out := dropPlaintextPasswordEntries(nil)
	assert.Empty(t, out)
}

// TestDropPlaintextPasswordEntriesNoMatches: no plaintext-* entries returns
// the same content (different backing array).
func TestDropPlaintextPasswordEntriesNoMatches(t *testing.T) {
	in := []config.SessionEntry{
		{Path: "bgp local-as"},
		{Path: "system authentication user alice profile"},
	}
	out := dropPlaintextPasswordEntries(in)
	assert.Len(t, out, 2)
	assert.Equal(t, in[0].Path, out[0].Path)
	assert.Equal(t, in[1].Path, out[1].Path)
}

// TestDropPlaintextPasswordEntriesAllMatch: all-plaintext input returns empty.
func TestDropPlaintextPasswordEntriesAllMatch(t *testing.T) {
	in := []config.SessionEntry{
		{Path: "system authentication user alice plaintext-password"},
		{Path: "system authentication user bob plaintext-password"},
	}
	out := dropPlaintextPasswordEntries(in)
	assert.Empty(t, out)
}

// newBlobEditor builds a session editor backed by blob storage, mirroring the
// production web-only wiring (the only mode that exercised the commit deadlock).
// The filesystem seed is removed so reads provably come from the blob store.
func newBlobEditor(t *testing.T, seed string) (*Editor, storage.Storage, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(seed), 0o600))

	store, err := storage.NewBlob(filepath.Join(dir, "database.zefs"), dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, os.Remove(configPath))

	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })
	ed.SetSession(NewEditSession("thomas", "local"))
	return ed, store, configPath
}

// TestCommitSessionFlushesOnBlob is the editor-layer regression for F1/F2:
// CommitSession used to self-deadlock on the zefs store mutex when it deleted
// the .edit file through e.store.Remove while holding the write guard. The
// deadlock prevented the guard from flushing, so the committed value never
// reached the blob. This test would hang (and fail via -timeout) before the
// fix that routes the .edit cleanup through the guard.
//
// This exercises the bare CommitSession() path with no reload notifier — the
// same path used by BOTH the web-only commit (no commit hook) and the appliance
// SSH session editor (session_factory.go wires no notifier, so cmdCommitSession
// takes the non-transactional CommitSession branch). The bug is not web-only.
//
// VALIDATES: blob-backed CommitSession completes and the value is persisted.
// PREVENTS: CommitSession self-deadlock + silent loss of the committed config.
func TestCommitSessionFlushesOnBlob(t *testing.T) {
	ed, store, configPath := newBlobEditor(t, validBGPConfig)

	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	// A fresh ReadFile goes through the store's own lock; it would itself hang
	// if CommitSession had not released the guard, and would lack the value if
	// the guard never flushed.
	data, err := store.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "9.9.9.9",
		"committed router-id must be flushed to the blob store")
	assert.False(t, ed.Dirty(), "editor should be clean after commit")
}

// TestDiscardSessionPathOnBlob covers the same lock-discipline class as F1:
// DiscardSessionPath called e.store.Exists(changePath) while holding the write
// guard, and blobStorage.Exists re-takes the store's read lock, deadlocking
// against the held write lock. Filesystem-backed tests never caught it because
// filesystem Exists does not touch the AcquireLock mutex. The fix uses
// guard.Has. This test would hang before the fix.
//
// VALIDATES: blob-backed DiscardSessionPath completes without deadlock.
// PREVENTS: store access under a held guard recurring elsewhere in the editor.
func TestDiscardSessionPathOnBlob(t *testing.T) {
	ed, _, _ := newBlobEditor(t, validBGPConfig)

	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))
	require.True(t, ed.Dirty(), "edit should mark the session dirty")

	require.NoError(t, ed.DiscardSessionPath(nil))
	assert.False(t, ed.Dirty(), "discard-all should clear the dirty flag")
}

// TestCommitStampsSchemaVersion verifies that CommitSession prepends the schema
// stamp to the written config file.
func TestCommitStampsSchemaVersion(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	content := string(data)
	assert.True(t, strings.HasPrefix(content, "# ze-schema: "),
		"committed config should start with schema stamp, got first line: %q",
		strings.SplitN(content, "\n", 2)[0])

	scanned := config.ScanStampRelease(data)
	assert.NotEmpty(t, scanned, "stamp release should be present")

	// AC-7: show config (WorkingContent) must NOT include the stamp.
	showOutput := ed.WorkingContent()
	assert.False(t, strings.HasPrefix(showOutput, "# ze-schema:"),
		"show config output should not contain schema stamp")

	// AC-9: re-commit to create a backup, then verify the backup has the stamp.
	err = ed.SetValue([]string{"bgp"}, "router-id", "8.8.8.8")
	require.NoError(t, err)
	_, err = ed.CommitSession()
	require.NoError(t, err)

	backups, err := ed.ListBackups()
	require.NoError(t, err)
	if assert.NotEmpty(t, backups, "should have at least one backup") {
		backupData, readErr := os.ReadFile(backups[0].Path)
		require.NoError(t, readErr)
		backupRelease := config.ScanStampRelease(backupData)
		assert.NotEmpty(t, backupRelease,
			"backup should contain schema stamp from committed config")
	}
}
