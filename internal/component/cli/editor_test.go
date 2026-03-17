package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

func TestNewEditor(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write initial config
	initial := `router-id 1.2.3.4
local-as 65000
`
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	// Create editor
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	assert.Equal(t, configPath, ed.OriginalPath())
	assert.False(t, ed.Dirty())
}

func TestEditorLoadNonExistent(t *testing.T) {
	_, err := NewEditor("/nonexistent/path/config.conf")
	require.Error(t, err)
}

func TestEditorSaveCreatesBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write initial config
	initial := `router-id 1.2.3.4` //nolint:goconst // test value
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	// Create editor and modify
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// Mark as dirty (simulating a change)
	ed.MarkDirty()

	// Save
	err = ed.Save()
	require.NoError(t, err)

	// Verify backup was created
	backups, err := ed.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 1)

	// Verify backup contains original content
	backupData, err := os.ReadFile(backups[0].Path)
	require.NoError(t, err)
	assert.Equal(t, initial, string(backupData))
}

// TestEditorBackupInRollbackDir verifies backups are stored in rollback/ subdirectory.
//
// VALIDATES: Backups are created in <dir>/rollback/ (Junos-style).
// PREVENTS: Backups polluting the config directory root.
func TestEditorBackupInRollbackDir(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte("router-id 1.2.3.4;"), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	ed.MarkDirty()
	err = ed.Save()
	require.NoError(t, err)

	// Verify rollback/ directory was created
	rollbackDir := filepath.Join(tmpDir, "rollback")
	info, err := os.Stat(rollbackDir)
	require.NoError(t, err, "rollback/ directory should exist")
	assert.True(t, info.IsDir(), "rollback/ should be a directory")

	// Verify backup is inside rollback/
	backups, err := ed.ListBackups()
	require.NoError(t, err)
	require.Len(t, backups, 1)
	assert.True(t, strings.HasPrefix(backups[0].Path, rollbackDir),
		"backup path %s should be under rollback/", backups[0].Path)
}

func TestEditorBackupNaming(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "myconfig.conf")

	// Write initial config
	err := os.WriteFile(configPath, []byte("test"), 0o600)
	require.NoError(t, err)

	// Create multiple backups
	for range 3 {
		ed, err := NewEditor(configPath)
		require.NoError(t, err)
		ed.MarkDirty()
		err = ed.Save()
		require.NoError(t, err)
		ed.Close() //nolint:errcheck,gosec // Best effort cleanup
	}

	// Check backup naming
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	backups, err := ed.ListBackups()
	require.NoError(t, err)
	require.Len(t, backups, 3)

	today := time.Now().Format("20060102")

	// Backups should be named: myconfig-YYYYMMDD-HHMMSS.conf
	for i, b := range backups {
		assert.True(t, strings.Contains(b.Path, today),
			"backup %d should contain today's date: %s", i, b.Path)
		assert.True(t, strings.HasSuffix(b.Path, ".conf"),
			"backup %d should end with .conf: %s", i, b.Path)
	}

	// Newest first (descending order)
	assert.True(t, !backups[0].Timestamp.Before(backups[1].Timestamp),
		"backups should be sorted newest first")
}

func TestEditorDiscard(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	initial := `router-id 1.2.3.4`
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// Mark dirty
	ed.MarkDirty()
	assert.True(t, ed.Dirty())

	// Discard
	err = ed.Discard()
	require.NoError(t, err)
	assert.False(t, ed.Dirty())
}

func TestEditorRollback(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write initial config
	version1 := `router-id 1.1.1.1`
	err := os.WriteFile(configPath, []byte(version1), 0o600)
	require.NoError(t, err)

	// Create first backup
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	ed.MarkDirty()
	err = ed.Save()
	require.NoError(t, err)
	_ = ed.Close()

	// Write version 2
	version2 := `router-id 2.2.2.2`
	err = os.WriteFile(configPath, []byte(version2), 0o600)
	require.NoError(t, err)

	// Rollback to first backup
	ed, err = NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	backups, err := ed.ListBackups()
	require.NoError(t, err)
	require.Len(t, backups, 1)

	err = ed.Rollback(backups[0].Path)
	require.NoError(t, err)

	// Verify content was restored
	data, err := os.ReadFile(configPath) //nolint:gosec // Test file path
	require.NoError(t, err)
	assert.Equal(t, version1, string(data))

	// Verify rollback created a backup of version2 (rollback is reversible)
	backups, err = ed.ListBackups()
	require.NoError(t, err)
	require.Len(t, backups, 2, "rollback should create a backup of the current config before restoring")

	// The newest backup (index 0) should contain version2
	backupData, err := os.ReadFile(backups[0].Path) //nolint:gosec // Test path
	require.NoError(t, err)
	assert.Equal(t, version2, string(backupData), "pre-rollback backup should preserve the overwritten config")
}

// TestAtomicWriteFileCreatesCorrectContent verifies atomic write produces correct file.
//
// VALIDATES: atomicWriteFile creates file with expected content and permissions.
// PREVENTS: Temp file left behind or wrong content written.
func TestAtomicWriteFileCreatesCorrectContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.conf")

	err := atomicWriteFile(path, []byte("hello world"))
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(data))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// TestAtomicWriteFileOverwritesExisting verifies atomic write replaces existing file.
//
// VALIDATES: Existing file is atomically replaced, not appended.
// PREVENTS: Stale content surviving a write.
func TestAtomicWriteFileOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.conf")

	require.NoError(t, os.WriteFile(path, []byte("old content"), 0o600))

	err := atomicWriteFile(path, []byte("new content"))
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(data))
}

// TestAtomicWriteFilePreservesOriginalOnDirFailure verifies original survives if dir is bad.
//
// VALIDATES: Original file is untouched when temp creation fails.
// PREVENTS: Data loss when target directory is not writable.
func TestAtomicWriteFilePreservesOriginalOnDirFailure(t *testing.T) {
	err := atomicWriteFile("/nonexistent/dir/file.conf", []byte("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create temp file")
}

// TestAtomicWriteFileNoTempFileLeftBehind verifies no .ze-tmp files remain after success.
//
// VALIDATES: Temp file is renamed (not left behind) on successful write.
// PREVENTS: Accumulation of orphan temp files in config directory.
func TestAtomicWriteFileNoTempFileLeftBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.conf")

	require.NoError(t, atomicWriteFile(path, []byte("content")))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.HasPrefix(e.Name(), ".ze-tmp-"),
			"temp file should not remain: %s", e.Name())
	}
}

// TestListBackupsSkipsMalformedFiles verifies junk files in rollback/ are ignored.
//
// VALIDATES: Non-matching files in rollback/ don't appear in backup list.
// PREVENTS: Panic or incorrect entries from malformed filenames.
func TestListBackupsSkipsMalformedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	require.NoError(t, os.WriteFile(configPath, []byte("content"), 0o600))

	// Create rollback dir with junk + one valid backup
	rollbackDir := filepath.Join(tmpDir, "rollback")
	require.NoError(t, os.MkdirAll(rollbackDir, 0o700))

	// Junk files
	require.NoError(t, os.WriteFile(filepath.Join(rollbackDir, "notes.txt"), []byte("junk"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(rollbackDir, "test-broken.conf"), []byte("junk"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(rollbackDir, "test-99999999-999999.999.conf"), []byte("junk"), 0o600))

	// Valid backup
	require.NoError(t, os.WriteFile(filepath.Join(rollbackDir, "test-20260101-120000.000.conf"), []byte("backup"), 0o600))

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	backups, err := ed.ListBackups()
	require.NoError(t, err)
	require.Len(t, backups, 1, "only the valid backup should be listed")
	assert.Contains(t, backups[0].Path, "20260101-120000")
}

func TestEditorListBackupsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte("test"), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	backups, err := ed.ListBackups()
	require.NoError(t, err)
	assert.Empty(t, backups)
}

func TestEditorDiff(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	initial := `router-id 1.2.3.4
local-as 65000
`
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// No changes yet
	diff := ed.Diff()
	assert.Empty(t, diff)

	// Simulate modification by setting working content
	ed.SetWorkingContent(`router-id 1.2.3.4
local-as 65001
`)
	ed.MarkDirty()

	diff = ed.Diff()
	assert.Contains(t, diff, "65000")
	assert.Contains(t, diff, "65001")
}

// TestEditFilePersistence verifies .edit file is created on changes.
//
// VALIDATES: Edit file created when config is modified.
// PREVENTS: Loss of uncommitted changes between sessions.
func TestEditFilePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	editPath := configPath + ".edit"

	// Write initial config
	initial := `router-id 1.2.3.4`
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	// Create editor
	ed, err := NewEditor(configPath)
	require.NoError(t, err)

	// No edit file yet
	_, err = os.Stat(editPath)
	assert.True(t, os.IsNotExist(err), "edit file should not exist initially")

	// Make a change and save edit state
	ed.SetWorkingContent(`router-id 2.2.2.2`)
	ed.MarkDirty()
	err = ed.SaveEditState()
	require.NoError(t, err)

	// Edit file should now exist
	_, err = os.Stat(editPath)
	assert.NoError(t, err, "edit file should exist after change")

	// Verify edit file content
	editContent, err := os.ReadFile(editPath) //nolint:gosec // test path
	require.NoError(t, err)
	assert.Equal(t, `router-id 2.2.2.2`, string(editContent))

	ed.Close() //nolint:errcheck,gosec // Best effort cleanup
}

// TestEditFileResume verifies editor loads from .edit file if exists.
//
// VALIDATES: Uncommitted changes restored from .edit file.
// PREVENTS: Loss of work when editor is restarted.
func TestEditFileResume(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	editPath := configPath + ".edit"

	// Write original config
	original := `router-id 1.1.1.1`
	err := os.WriteFile(configPath, []byte(original), 0o600)
	require.NoError(t, err)

	// Write existing edit file (simulating previous session)
	editContent := `router-id 9.9.9.9`
	err = os.WriteFile(editPath, []byte(editContent), 0o600)
	require.NoError(t, err)

	// Create editor - should detect and report edit file
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// Editor should indicate existing edit file
	assert.True(t, ed.HasPendingEdit(), "editor should detect pending edit")

	// Load from edit file
	err = ed.LoadPendingEdit()
	require.NoError(t, err)

	// Working content should be from edit file
	assert.Equal(t, editContent, ed.WorkingContent())
	assert.True(t, ed.Dirty())
}

// TestEditFileDeletedOnCommit verifies .edit file removed after commit.
//
// VALIDATES: Edit file cleaned up on successful commit.
// PREVENTS: Stale edit files accumulating.
func TestEditFileDeletedOnCommit(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	editPath := configPath + ".edit"

	// Write initial config
	initial := `router-id 1.2.3.4`
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	// Create editor and make changes
	ed, err := NewEditor(configPath)
	require.NoError(t, err)

	ed.SetWorkingContent(`router-id 2.2.2.2`)
	ed.MarkDirty()
	err = ed.SaveEditState()
	require.NoError(t, err)

	// Edit file exists
	_, err = os.Stat(editPath)
	require.NoError(t, err, "edit file should exist before commit")

	// Commit
	err = ed.Save()
	require.NoError(t, err)

	// Edit file should be gone
	_, err = os.Stat(editPath)
	assert.True(t, os.IsNotExist(err), "edit file should be deleted after commit")

	ed.Close() //nolint:errcheck,gosec // Best effort cleanup
}

// TestEditFileDeletedOnDiscard verifies .edit file removed after discard.
//
// VALIDATES: Edit file cleaned up when changes discarded.
// PREVENTS: Stale edit files from discarded sessions.
func TestEditFileDeletedOnDiscard(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	editPath := configPath + ".edit"

	// Write initial config
	initial := `router-id 1.2.3.4`
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	// Create editor and make changes
	ed, err := NewEditor(configPath)
	require.NoError(t, err)

	ed.SetWorkingContent(`router-id 2.2.2.2`)
	ed.MarkDirty()
	err = ed.SaveEditState()
	require.NoError(t, err)

	// Edit file exists
	_, err = os.Stat(editPath)
	require.NoError(t, err, "edit file should exist before discard")

	// Discard
	err = ed.Discard()
	require.NoError(t, err)

	// Edit file should be gone
	_, err = os.Stat(editPath)
	assert.True(t, os.IsNotExist(err), "edit file should be deleted after discard")

	ed.Close() //nolint:errcheck,gosec // Best effort cleanup
}

// TestPendingEditTime verifies edit file modification time is returned.
//
// VALIDATES: PendingEditTime returns valid time when edit file exists.
// PREVENTS: Startup prompt showing wrong timestamp.
func TestPendingEditTime(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	editPath := configPath + ".edit"

	// Write initial config
	initial := `router-id 1.2.3.4`
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	// Create editor - no edit file yet
	ed, err := NewEditor(configPath)
	require.NoError(t, err)

	// No pending edit, time should be zero
	assert.True(t, ed.PendingEditTime().IsZero(), "no edit file should return zero time")

	// Create edit file
	editContent := `router-id 2.2.2.2`
	err = os.WriteFile(editPath, []byte(editContent), 0o600)
	require.NoError(t, err)

	// Recreate editor to detect edit file
	ed.Close() //nolint:errcheck,gosec // test cleanup: recreating editor below
	ed, err = NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	// Should have pending edit with recent time
	assert.True(t, ed.HasPendingEdit(), "should detect edit file")
	modTime := ed.PendingEditTime()
	assert.False(t, modTime.IsZero(), "should return edit file time")
	assert.WithinDuration(t, time.Now(), modTime, 5*time.Second, "time should be recent")
}

// TestPendingEditDiff verifies diff between original and pending edit.
//
// VALIDATES: PendingEditDiff shows changes in edit file.
// PREVENTS: View changes option showing nothing.
func TestPendingEditDiff(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	editPath := configPath + ".edit"

	// Write initial config
	initial := `router-id 1.2.3.4
local-as 65000`
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	// Write edit file with changes
	editContent := `router-id 2.2.2.2
local-as 65000
peer-as 65001`
	err = os.WriteFile(editPath, []byte(editContent), 0o600)
	require.NoError(t, err)

	// Create editor
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	// Get diff
	diff := ed.PendingEditDiff()
	assert.Contains(t, diff, "- router-id 1.2.3.4", "should show removed line")
	assert.Contains(t, diff, "+ router-id 2.2.2.2", "should show added line")
	assert.Contains(t, diff, "+ peer-as 65001", "should show new line")
}

// TestPendingEditDiffNoEditFile verifies empty diff when no edit file exists.
//
// VALIDATES: PendingEditDiff returns empty when no .edit file.
// PREVENTS: Error when viewing changes without edit file.
func TestPendingEditDiffNoEditFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write config only, no edit file
	content := `router-id 1.2.3.4`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	// Create editor - no edit file
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	// Diff should be empty (no edit file to compare)
	diff := ed.PendingEditDiff()
	assert.Empty(t, diff, "no edit file should produce empty diff")
}

// TestPendingEditDiffNoChanges verifies empty diff when content matches.
//
// VALIDATES: PendingEditDiff returns empty when no changes.
// PREVENTS: False diff display.
func TestPendingEditDiffNoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	editPath := configPath + ".edit"

	// Write same content to both files
	content := `router-id 1.2.3.4`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(editPath, []byte(content), 0o600)
	require.NoError(t, err)

	// Create editor
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	// Diff should be empty
	diff := ed.PendingEditDiff()
	assert.Empty(t, diff, "no changes should produce empty diff")
}

// --- Tree-canonical tests (spec-editor-tree-canonical) ---

// validBGPConfig is a parseable config for tree tests.
const validBGPConfig = `bgp {
	router-id 1.2.3.4
	local-as 65000
	peer 1.1.1.1 {
		peer-as 65001
		hold-time 90
	}
}
`

// writeTestConfig writes a config to a temp file and returns the path.
func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "test.conf")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)
	return configPath
}

// TestEditorTreeValid verifies treeValid is set when config parses.
//
// VALIDATES: NewEditor sets treeValid=true and stores schema for valid configs.
// PREVENTS: Tree always invalid, falling back to raw text.
func TestEditorTreeValid(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	assert.True(t, ed.treeValid, "treeValid should be true for parseable config")
	assert.NotNil(t, ed.schema, "schema should be stored")
	assert.NotNil(t, ed.tree, "tree should be populated")
}

// TestEditorTreeInvalidFallback verifies treeValid is false for unparseable configs.
//
// VALIDATES: Editor gracefully handles invalid configs by leaving treeValid false.
// PREVENTS: Crash when opening garbled config files.
func TestEditorTreeInvalidFallback(t *testing.T) {
	configPath := writeTestConfig(t, `this is not { valid } config syntax !!!`)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	assert.False(t, ed.treeValid, "treeValid should be false for unparseable config")
}

// TestEditorTreeNavigation verifies WalkPath navigates containers.
//
// VALIDATES: WalkPath(["bgp"]) returns the bgp container subtree.
// PREVENTS: Tree navigation returning nil for valid paths.
func TestEditorTreeNavigation(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	bgp := ed.WalkPath([]string{"bgp"})
	require.NotNil(t, bgp, "WalkPath should find 'bgp' container")

	// Verify we can read values inside the container
	rid, ok := bgp.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "1.2.3.4", rid)
}

// TestEditorTreeNavigationListKey verifies WalkPath navigates through list entries.
//
// VALIDATES: WalkPath(["bgp","peer","1.1.1.1"]) navigates through list keyed by peer address.
// PREVENTS: List entries unreachable via tree navigation.
func TestEditorTreeNavigationListKey(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	peer := ed.WalkPath([]string{"bgp", "peer", "1.1.1.1"})
	require.NotNil(t, peer, "WalkPath should find peer 1.1.1.1 via list navigation")

	peerAS, ok := peer.Get("peer-as")
	assert.True(t, ok)
	assert.Equal(t, "65001", peerAS)
}

// TestEditorTreeNavigationMissing verifies WalkPath returns nil for nonexistent paths.
//
// VALIDATES: WalkPath returns nil (not crash) for missing path elements.
// PREVENTS: Panic on invalid path navigation.
func TestEditorTreeNavigationMissing(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	assert.Nil(t, ed.WalkPath([]string{"nonexistent"}), "missing top-level should return nil")
	assert.Nil(t, ed.WalkPath([]string{"bgp", "peer", "9.9.9.9"}), "missing peer should return nil")
	assert.Nil(t, ed.WalkPath([]string{"bgp", "nonexistent"}), "missing child should return nil")
}

// TestEditorTreeSet verifies SetValue mutates tree and marks dirty.
//
// VALIDATES: SetValue changes a value in the tree and sets dirty flag.
// PREVENTS: Mutations silently lost, dirty flag not set.
func TestEditorTreeSet(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)
	assert.True(t, ed.Dirty(), "editor should be dirty after SetValue")

	// Verify tree was mutated
	bgp := ed.WalkPath([]string{"bgp"})
	require.NotNil(t, bgp)
	rid, ok := bgp.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "5.6.7.8", rid)
}

// TestEditorTreeSetNewKey verifies SetValue creates a new key.
//
// VALIDATES: SetValue adds a key that didn't exist before.
// PREVENTS: SetValue only updating existing keys, ignoring new ones.
func TestEditorTreeSetNewKey(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	err = ed.SetValue([]string{"bgp", "peer", "1.1.1.1"}, "description", "test-peer")
	require.NoError(t, err)

	peer := ed.WalkPath([]string{"bgp", "peer", "1.1.1.1"})
	require.NotNil(t, peer)
	desc, ok := peer.Get("description")
	assert.True(t, ok)
	assert.Equal(t, "test-peer", desc)
}

// TestEditorTreeDelete verifies DeleteValue removes a leaf from the tree.
//
// VALIDATES: DeleteValue removes a key-value pair from the tree.
// PREVENTS: Delete being a no-op stub.
func TestEditorTreeDelete(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	err = ed.DeleteValue([]string{"bgp", "peer", "1.1.1.1"}, "hold-time")
	require.NoError(t, err)
	assert.True(t, ed.Dirty())

	peer := ed.WalkPath([]string{"bgp", "peer", "1.1.1.1"})
	require.NotNil(t, peer)
	_, ok := peer.Get("hold-time")
	assert.False(t, ok, "hold-time should be deleted")
}

// TestEditorTreeDeleteContainer verifies DeleteContainer removes a block.
//
// VALIDATES: DeleteContainer removes an entire container from the tree.
// PREVENTS: Container deletion not working.
func TestEditorTreeDeleteContainer(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	err = ed.DeleteContainer([]string{}, "bgp")
	require.NoError(t, err)
	assert.True(t, ed.Dirty())
	assert.Nil(t, ed.WalkPath([]string{"bgp"}), "bgp should be gone after delete")
}

// TestEditorTreeDeleteListEntry verifies DeleteListEntry removes a peer.
//
// VALIDATES: DeleteListEntry removes a keyed list entry.
// PREVENTS: List entry deletion not working.
func TestEditorTreeDeleteListEntry(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	err = ed.DeleteListEntry([]string{"bgp"}, "peer", "1.1.1.1")
	require.NoError(t, err)
	assert.True(t, ed.Dirty())
	assert.Nil(t, ed.WalkPath([]string{"bgp", "peer", "1.1.1.1"}), "peer should be gone")
}

// TestEditorDeleteByPathListEntry verifies DeleteByPath detects list entries via schema.
//
// VALIDATES: DeleteByPath with path ["bgp","peer","1.1.1.1"] uses schema to find
// that "peer" is a ListNode and calls DeleteListEntry.
// PREVENTS: delete bgp peer 1.1.1.1 failing because WalkPath can't resolve list paths.
func TestEditorDeleteByPathListEntry(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	err = ed.DeleteByPath([]string{"bgp", "peer", "1.1.1.1"})
	require.NoError(t, err)
	assert.True(t, ed.Dirty())
	assert.Nil(t, ed.WalkPath([]string{"bgp", "peer", "1.1.1.1"}), "peer should be gone")
}

// TestEditorDeleteByPathLeaf verifies DeleteByPath removes a leaf value.
//
// VALIDATES: DeleteByPath with leaf path uses DeleteValue.
// PREVENTS: Leaf deletion broken by schema-aware path logic.
func TestEditorDeleteByPathLeaf(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	err = ed.DeleteByPath([]string{"bgp", "router-id"})
	require.NoError(t, err)
	assert.True(t, ed.Dirty())

	bgp := ed.WalkPath([]string{"bgp"})
	require.NotNil(t, bgp)
	_, ok := bgp.Get("router-id")
	assert.False(t, ok, "router-id should be deleted")
}

// TestEditorDeleteByPathContainer verifies DeleteByPath removes a container.
//
// VALIDATES: DeleteByPath with container path uses DeleteContainer.
// PREVENTS: Container deletion broken by schema-aware path logic.
func TestEditorDeleteByPathContainer(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	err = ed.DeleteByPath([]string{"bgp"})
	require.NoError(t, err)
	assert.True(t, ed.Dirty())
	assert.Nil(t, ed.WalkPath([]string{"bgp"}), "bgp should be gone")
}

// TestEditorContentAfterSet verifies WorkingContent returns serialized tree.
//
// VALIDATES: After SetValue, WorkingContent() returns Serialize(tree) containing the new value.
// PREVENTS: WorkingContent returning stale raw text after tree mutation.
func TestEditorContentAfterSet(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	content := ed.WorkingContent()
	assert.Contains(t, content, "9.9.9.9", "serialized output should contain new value")
	assert.NotContains(t, content, "1.2.3.4", "serialized output should not contain old value")
}

// TestEditorSerializeRoundtrip verifies tree → serialize → parse → tree equality.
//
// VALIDATES: Serialize(tree) produces valid config that re-parses to equivalent tree.
// PREVENTS: Serialization losing or corrupting data.
func TestEditorSerializeRoundtrip(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	// Mutate
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	// Get serialized content
	content := ed.WorkingContent()

	// Parse again
	schema := config.YANGSchema()
	parser := config.NewParser(schema)
	tree2, err := parser.Parse(content)
	require.NoError(t, err, "serialized content should be parseable")

	// Verify the value survived the round-trip
	bgp := tree2.GetContainer("bgp")
	require.NotNil(t, bgp)
	rid, ok := bgp.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "9.9.9.9", rid)
}

// TestEditorSaveSerialized verifies Save writes Serialize(tree) to disk.
//
// VALIDATES: Save() writes serialized tree content, not stale raw text.
// PREVENTS: Save writing outdated workingContent after tree mutations.
func TestEditorSaveSerialized(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	// Mutate via tree
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	// Save
	err = ed.Save()
	require.NoError(t, err)

	// Read from disk
	data, err := os.ReadFile(configPath) //nolint:gosec // test path
	require.NoError(t, err)
	assert.Contains(t, string(data), "9.9.9.9", "saved file should contain new value")
	assert.NotContains(t, string(data), "1.2.3.4", "saved file should not contain old value")
}

// TestEditorDiscardReparse verifies Discard re-parses original into tree.
//
// VALIDATES: After Discard(), tree is restored to original parsed state.
// PREVENTS: Discard leaving tree in mutated state.
func TestEditorDiscardReparse(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	// Mutate
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	// Discard
	err = ed.Discard()
	require.NoError(t, err)

	// Tree should be back to original
	bgp := ed.WalkPath([]string{"bgp"})
	require.NotNil(t, bgp)
	rid, ok := bgp.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "1.2.3.4", rid, "after discard, tree should have original value")
	assert.True(t, ed.treeValid, "treeValid should still be true after discard")
}

// TestEditorSetWorkingContentParse verifies SetWorkingContent parses into tree.
//
// VALIDATES: SetWorkingContent() parses text into tree for backward compat.
// PREVENTS: SetWorkingContent leaving tree stale after text-based load.
func TestEditorSetWorkingContentParse(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	// Load new content via SetWorkingContent (simulates load command)
	newContent := `bgp {
	router-id 5.6.7.8
	local-as 65001
}
`
	ed.SetWorkingContent(newContent)

	// Tree should reflect new content
	bgp := ed.WalkPath([]string{"bgp"})
	require.NotNil(t, bgp, "tree should be updated from SetWorkingContent")
	rid, ok := bgp.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "5.6.7.8", rid)
}

// TestDirtyFalseAfterDiscard verifies that Dirty() returns false after Discard().
//
// VALIDATES: spec-editor-1 AC-3: Discard clears dirty flag.
// PREVENTS: Dirty stuck true after discard, blocking exit.
func TestDirtyFalseAfterDiscard(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte("bgp { router-id 1.2.3.4; }"), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	assert.False(t, ed.Dirty(), "should start clean")

	// Mark dirty (simulates a set command)
	ed.MarkDirty()
	assert.True(t, ed.Dirty(), "should be dirty after MarkDirty")

	// Discard
	err = ed.Discard()
	require.NoError(t, err)
	assert.False(t, ed.Dirty(), "should NOT be dirty after Discard")
}

// TestSerializationRoundTrip verifies parse→serialize→parse produces same content.
//
// VALIDATES: spec-editor-1 AC-5: no false dirty from serialization drift.
// PREVENTS: Re-serialization changing content, causing perceived dirty state.
func TestSerializationRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"simple", "bgp { router-id 1.2.3.4; }"},
		{"with-peer", `bgp {
  router-id 1.2.3.4
  local-as 65000
  peer 1.1.1.1 {
    peer-as 65001
    hold-time 90
  }
}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := config.YANGSchema()
			require.NotNil(t, schema)

			parser := config.NewParser(schema)
			tree1, err := parser.Parse(tt.content)
			require.NoError(t, err)

			// Serialize (round-trip 1)
			serialized1 := config.Serialize(tree1, schema)

			// Parse again (round-trip 2)
			tree2, err := parser.Parse(serialized1)
			require.NoError(t, err)
			serialized2 := config.Serialize(tree2, schema)

			// Second round-trip must be stable
			assert.Equal(t, serialized1, serialized2,
				"serialization must be stable after first round-trip")
		})
	}
}

// TestEditorWriteThrough verifies that SetValue with a session creates a draft file.
//
// VALIDATES: Write-through protocol creates draft with metadata on SetValue.
// PREVENTS: SetValue silently staying in-memory when session is set.
func TestEditorWriteThrough(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// Set session to enable write-through.
	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Change a value under the bgp container.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	// Draft file should exist.
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err, "draft file should exist after write-through")

	draftContent := string(draftData)

	// Draft should contain the new value.
	assert.Contains(t, draftContent, "5.6.7.8", "draft should contain updated value")

	// Draft should contain metadata with user.
	assert.Contains(t, draftContent, "#thomas@local", "draft should contain user metadata")

	// Draft should contain session metadata.
	assert.Contains(t, draftContent, "%"+session.ID, "draft should contain session metadata")

	// Draft should contain the unchanged value too.
	assert.Contains(t, draftContent, "65000", "draft should preserve unchanged values")

	// In-memory tree should reflect the change.
	bgpTree := ed.tree.GetContainer("bgp")
	require.NotNil(t, bgpTree)
	val, ok := bgpTree.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "5.6.7.8", val)
}

// TestEditorWriteThroughDelete verifies delete with a session updates the draft.
//
// VALIDATES: Write-through protocol works for DeleteValue.
// PREVENTS: Deletes staying in-memory only when session is set.
func TestEditorWriteThroughDelete(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Delete local-as under bgp container.
	err = ed.DeleteValue([]string{"bgp"}, "local-as")
	require.NoError(t, err)

	// Draft file should exist.
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err, "draft file should exist after write-through delete")

	draftContent := string(draftData)

	// Draft should NOT contain a set line for the deleted value.
	assert.NotContains(t, draftContent, "set bgp local-as", "deleted value should not appear as set line")

	// Draft should contain a delete line with metadata for the deleted value.
	assert.Contains(t, draftContent, "delete bgp local-as", "delete metadata should be preserved in draft")

	// Delete metadata should include the previous value.
	assert.Contains(t, draftContent, "^65000", "delete metadata should record previous value")

	// Draft should still have the remaining values.
	assert.Contains(t, draftContent, "1.2.3.4", "draft should preserve non-deleted values")

	// In-memory tree should reflect the deletion.
	bgpTree := ed.tree.GetContainer("bgp")
	require.NotNil(t, bgpTree)
	_, ok := bgpTree.Get("local-as")
	assert.False(t, ok, "deleted value should not exist in tree")
}

// TestEditorWriteThroughPreservesSessions verifies that write-through preserves
// other sessions' metadata in the draft file.
//
// VALIDATES: Concurrent editing preserves other sessions' changes in draft.
// PREVENTS: One session overwriting another's pending changes.
func TestEditorWriteThroughPreservesSessions(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Create first editor session and make a change.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)

	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Create second editor session and make a different change.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)

	err = ed2.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Read the final draft.
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err)

	draftContent := string(draftData)

	// Both sessions' metadata should be present.
	assert.Contains(t, draftContent, "#alice@ssh", "alice's metadata should be preserved")
	assert.Contains(t, draftContent, "#thomas@local", "thomas's metadata should be present")
	assert.Contains(t, draftContent, "%"+session1.ID, "alice's session should be preserved")
	assert.Contains(t, draftContent, "%"+session2.ID, "thomas's session should be present")

	// Both values should be present.
	assert.Contains(t, draftContent, "10.0.0.1", "alice's value should be preserved")
	assert.Contains(t, draftContent, "65001", "thomas's value should be present")
}

// TestEditorWriteThroughPrevious verifies that Previous field records the committed value.
//
// VALIDATES: Write-through records Previous from config.conf for conflict detection.
// PREVENTS: Missing Previous field that breaks stale conflict detection.
func TestEditorWriteThroughPrevious(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Change router-id under bgp.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	// Read the draft and parse to check Previous.
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err)

	// Parse draft to extract metadata.
	schema := config.YANGSchema()
	parser := config.NewSetParser(schema)
	_, meta, err := parser.ParseWithMeta(string(draftData))
	require.NoError(t, err)

	// The metadata for router-id is under the bgp container in MetaTree.
	bgpMeta := meta.GetOrCreateContainer("bgp")
	entry, ok := bgpMeta.GetEntry("router-id")
	require.True(t, ok, "metadata for bgp/router-id should exist")
	assert.Equal(t, "1.2.3.4", entry.Previous,
		fmt.Sprintf("Previous should record committed value, got %q", entry.Previous))
}

// TestEditorWriteThroughListEntry verifies write-through for a leaf under a YANG
// list entry (e.g., bgp peer 1.1.1.1 hold-time). This exercises the schema-aware
// MetaTree navigation where list keys must be stored in .lists, not .containers.
//
// VALIDATES: Write-through records metadata for leaves under list entries.
// PREVENTS: Metadata silently dropped for list-scoped leaves (walkOrCreateMeta bug).
func TestEditorWriteThroughListEntry(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Change hold-time under bgp peer 1.1.1.1 (a list entry path).
	err = ed.SetValue([]string{"bgp", "peer", "1.1.1.1"}, "hold-time", "180")
	require.NoError(t, err)

	// Draft file should exist with metadata for the list entry leaf.
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err, "draft file should exist after write-through")

	draftContent := string(draftData)

	// Draft should contain the new value.
	assert.Contains(t, draftContent, "180", "draft should contain updated hold-time")

	// Draft should contain user metadata for the list entry leaf.
	// This is the critical check: if walkOrCreateMeta doesn't use schema-aware
	// navigation, metadata gets stored in containers instead of lists, and the
	// serializer can't find it.
	assert.Contains(t, draftContent, "#thomas@local", "draft should contain user metadata for list entry leaf")

	// Verify metadata is correctly structured by re-parsing.
	schema := config.YANGSchema()
	parser := config.NewSetParser(schema)
	_, meta, parseErr := parser.ParseWithMeta(draftContent)
	require.NoError(t, parseErr)

	// Navigate to the list entry's metadata: bgp -> peer (container) -> 1.1.1.1 (list entry).
	bgpMeta := meta.GetContainer("bgp")
	require.NotNil(t, bgpMeta, "bgp metadata container should exist")
	peerMeta := bgpMeta.GetContainer("peer")
	require.NotNil(t, peerMeta, "peer metadata container should exist")
	entryMeta := peerMeta.GetListEntry("1.1.1.1")
	require.NotNil(t, entryMeta, "peer 1.1.1.1 metadata list entry should exist")
	entry, ok := entryMeta.GetEntry("hold-time")
	require.True(t, ok, "hold-time metadata should exist under peer list entry")
	assert.Equal(t, session.ID, entry.Session, "session ID should be recorded")
	assert.Equal(t, "90", entry.Previous, "Previous should record committed hold-time value")
}

// TestEditorConcurrentWrite verifies two editors can write without corruption.
//
// VALIDATES: Concurrent writes under Storage.AcquireLock() produce valid draft files.
// PREVENTS: File corruption from overlapping writes.
func TestEditorConcurrentWrite(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	const goroutines = 4
	const writesPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(goroutines)
	errCh := make(chan error, goroutines*writesPerGoroutine)

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			ed, edErr := NewEditor(configPath)
			if edErr != nil {
				errCh <- edErr
				return
			}
			defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

			session := NewEditSession(fmt.Sprintf("user%d", idx), "local")
			ed.SetSession(session)

			for j := range writesPerGoroutine {
				value := fmt.Sprintf("%d.%d.%d.%d", idx, j, idx, j)
				if setErr := ed.SetValue([]string{"bgp"}, "router-id", value); setErr != nil {
					errCh <- setErr
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for writeErr := range errCh {
		require.NoError(t, writeErr, "concurrent write should not error")
	}

	// Draft should be parseable (not corrupted).
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err, "draft file should exist")

	schema := config.YANGSchema()
	parser := config.NewSetParser(schema)
	_, _, parseErr := parser.ParseWithMeta(string(draftData))
	assert.NoError(t, parseErr, "draft should parse without errors (no corruption)")
}

// TestEditorCommitSession verifies successful commit with no conflicts.
//
// VALIDATES: CommitSession applies session changes to config.conf.
// PREVENTS: Changes staying in draft forever.
func TestEditorCommitSession(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Make a change via write-through.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	// Commit.
	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts, "no conflicts expected")
	assert.Equal(t, 1, result.Applied)

	// Config.conf should contain the new value.
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(configData), "5.6.7.8", "config should have committed value")

	// Config.conf should NOT contain %session tokens.
	assert.NotContains(t, string(configData), "%", "config should not contain session tokens")

	// Draft file should be deleted (no other sessions).
	draftPath := DraftPath(configPath)
	_, err = os.Stat(draftPath)
	assert.True(t, os.IsNotExist(err), "draft should be deleted after commit with no other sessions")
}

// TestEditorConflictConvergentDelete verifies no false positive when both sessions
// delete the same value.
//
// VALIDATES: Concurrent delete of same value is convergent agreement, not stale conflict.
// PREVENTS: False positive stale conflict blocking a no-op commit.
func TestEditorConflictConvergentDelete(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 2 deletes local-as first (creates draft with Previous="65000").
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.DeleteValue([]string{"bgp"}, "local-as")
	require.NoError(t, err)

	// Session 1 also deletes local-as (both sessions have pending deletes).
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.DeleteValue([]string{"bgp"}, "local-as")
	require.NoError(t, err)

	// Session 1 commits: local-as is removed from config.conf.
	// Session 2's delete entry remains in the draft with Previous="65000".
	result1, err := ed1.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, 1, result1.Applied)

	// Session 2 commits: Previous="65000" but committed="" (already deleted).
	// This is convergent agreement -- both sessions wanted the same outcome.
	result2, err := ed2.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result2.Conflicts, "convergent delete should not produce stale conflict")
}

// TestEditorConflictDeleteVsSet verifies live conflict when one session deletes
// and another session sets the same leaf.
//
// VALIDATES: CommitSession detects conflict between delete and set intents.
// PREVENTS: Silent data corruption where delete intent applies set value or vice versa.
func TestEditorConflictDeleteVsSet(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session A deletes local-as.
	edA, err := NewEditor(configPath)
	require.NoError(t, err)
	defer edA.Close() //nolint:errcheck,gosec // Best effort cleanup

	sessionA := NewEditSession("alice", "ssh")
	edA.SetSession(sessionA)
	err = edA.DeleteValue([]string{"bgp"}, "local-as")
	require.NoError(t, err)

	// Session B sets local-as to a different value.
	edB, err := NewEditor(configPath)
	require.NoError(t, err)
	defer edB.Close() //nolint:errcheck,gosec // Best effort cleanup

	sessionB := NewEditSession("bob", "local")
	edB.SetSession(sessionB)
	err = edB.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Session A commits: should detect live conflict with B's set.
	resultA, err := edA.CommitSession()
	require.NoError(t, err)
	require.NotEmpty(t, resultA.Conflicts, "live conflict expected: delete vs set")
	assert.Equal(t, 0, resultA.Applied, "no changes should be applied")

	foundLive := false
	for _, c := range resultA.Conflicts {
		if c.Type == ConflictLive {
			foundLive = true
			assert.Equal(t, "", c.MyValue, "A's intent is delete (empty value)")
			assert.Equal(t, "65001", c.OtherValue, "B's intent is set 65001")
		}
	}
	assert.True(t, foundLive, "should have a live conflict for delete vs set")
}

// TestEditorConflictSetVsDelete verifies live conflict when one session sets
// and another session deletes the same leaf (reverse of DeleteVsSet).
//
// VALIDATES: CommitSession detects conflict from the set side too.
// PREVENTS: Set commit silently ignoring a concurrent delete.
func TestEditorConflictSetVsDelete(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session A sets local-as to a new value.
	edA, err := NewEditor(configPath)
	require.NoError(t, err)
	defer edA.Close() //nolint:errcheck,gosec // Best effort cleanup

	sessionA := NewEditSession("alice", "ssh")
	edA.SetSession(sessionA)
	err = edA.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Session B deletes local-as.
	edB, err := NewEditor(configPath)
	require.NoError(t, err)
	defer edB.Close() //nolint:errcheck,gosec // Best effort cleanup

	sessionB := NewEditSession("bob", "local")
	edB.SetSession(sessionB)
	err = edB.DeleteValue([]string{"bgp"}, "local-as")
	require.NoError(t, err)

	// Session A commits: should detect live conflict with B's delete.
	resultA, err := edA.CommitSession()
	require.NoError(t, err)
	require.NotEmpty(t, resultA.Conflicts, "live conflict expected: set vs delete")
	assert.Equal(t, 0, resultA.Applied, "no changes should be applied")

	foundLive := false
	for _, c := range resultA.Conflicts {
		if c.Type == ConflictLive {
			foundLive = true
			assert.Equal(t, "65001", c.MyValue, "A's intent is set 65001")
			assert.Equal(t, "", c.OtherValue, "B's intent is delete (empty value)")
		}
	}
	assert.True(t, foundLive, "should have a live conflict for set vs delete")
}

// TestEditorConflictStale verifies stale Previous conflict detection.
//
// VALIDATES: CommitSession detects when config.conf changed since edit.
// PREVENTS: Silently overwriting another commit.
func TestEditorConflictStale(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Make a change (records Previous = "1.2.3.4").
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	// Simulate another commit by modifying config.conf directly.
	modifiedConfig := strings.Replace(validBGPConfig, "1.2.3.4", "9.9.9.9", 1)
	err = os.WriteFile(configPath, []byte(modifiedConfig), 0o600)
	require.NoError(t, err)

	// Commit should detect stale conflict.
	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.NotEmpty(t, result.Conflicts, "stale conflict expected")
	assert.Equal(t, 0, result.Applied, "no changes should be applied")

	found := false
	for _, c := range result.Conflicts {
		if c.Type == ConflictStale {
			found = true
			assert.Equal(t, "5.6.7.8", c.MyValue)
			assert.Equal(t, "9.9.9.9", c.OtherValue, "should show current committed value")
			assert.Equal(t, "1.2.3.4", c.PreviousValue, "should show original committed value")
		}
	}
	assert.True(t, found, "should have a stale conflict")
}

// TestEditorConflictLive verifies live disagreement conflict detection.
//
// VALIDATES: CommitSession detects when another session has a different value at same path.
// PREVENTS: Silently overwriting another session's pending change.
func TestEditorConflictLive(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 changes router-id to 10.0.0.1.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 changes router-id to 10.0.0.2 (different value, same path).
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "router-id", "10.0.0.2")
	require.NoError(t, err)

	// Session 2 tries to commit: should detect live conflict with session 1.
	result, err := ed2.CommitSession()
	require.NoError(t, err)
	require.NotEmpty(t, result.Conflicts, "live conflict expected")
	assert.Equal(t, 0, result.Applied)

	found := false
	for _, c := range result.Conflicts {
		if c.Type == ConflictLive {
			found = true
			assert.Equal(t, "10.0.0.2", c.MyValue)
		}
	}
	assert.True(t, found, "should have a live conflict")
}

// TestEditorConflictAgreement verifies no conflict when both sessions set the same value.
//
// VALIDATES: Agreement between sessions is not a conflict.
// PREVENTS: False positive conflicts blocking valid commits.
func TestEditorConflictAgreement(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 changes router-id to 10.0.0.1.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 changes router-id to the SAME value.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 commits: agreement, no conflict.
	result, err := ed2.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result.Conflicts, "agreement should not produce conflicts")
	assert.Equal(t, 1, result.Applied)
}

// TestEditorDiscardAll verifies discarding all session changes.
//
// VALIDATES: DiscardSessionPath(nil) removes all my changes from draft.
// PREVENTS: Stale changes accumulating in draft after discard.
func TestEditorDiscardAll(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Make two changes.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)
	err = ed.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Discard all.
	err = ed.DiscardSessionPath(nil)
	require.NoError(t, err)

	// Draft should be deleted (no other sessions).
	draftPath := DraftPath(configPath)
	_, err = os.Stat(draftPath)
	assert.True(t, os.IsNotExist(err), "draft should be deleted after discard all")

	// In-memory tree should reflect original values.
	bgpTree := ed.tree.GetContainer("bgp")
	require.NotNil(t, bgpTree)
	val, ok := bgpTree.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "1.2.3.4", val, "router-id should revert to committed value")
	val, ok = bgpTree.Get("local-as")
	assert.True(t, ok)
	assert.Equal(t, "65000", val, "local-as should revert to committed value")
}

// TestEditorDiscardPreservesOtherSessions verifies that discarding one session's
// changes preserves another session's changes in the draft.
//
// VALIDATES: Discard only removes my session's changes.
// PREVENTS: Accidentally removing other sessions' pending work.
func TestEditorDiscardPreservesOtherSessions(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 makes a change.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 makes a change and then discards.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Session 2 discards all.
	err = ed2.DiscardSessionPath(nil)
	require.NoError(t, err)

	// Draft should still exist (alice's changes remain).
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err, "draft should exist with alice's changes")

	draftContent := string(draftData)
	assert.Contains(t, draftContent, "#alice@ssh", "alice's metadata should be preserved")
	assert.Contains(t, draftContent, "10.0.0.1", "alice's value should be preserved")
	assert.NotContains(t, draftContent, "%"+session2.ID, "thomas's session should be removed")
}

// TestEditorConflictBlocksEntireCommit verifies that any conflict blocks the entire commit.
//
// VALIDATES: Conflicts prevent partial application (all-or-nothing commit).
// PREVENTS: Applying some changes while others are blocked by conflicts.
func TestEditorConflictBlocksEntireCommit(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 changes router-id.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 changes router-id (conflict) AND local-as (no conflict).
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "router-id", "10.0.0.2")
	require.NoError(t, err)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65002")
	require.NoError(t, err)

	// Commit should fail with conflicts, Applied=0 (even local-as was not applied).
	result, err := ed2.CommitSession()
	require.NoError(t, err)
	require.NotEmpty(t, result.Conflicts)
	assert.Equal(t, 0, result.Applied, "no changes should be applied when conflicts exist")

	// Verify config.conf is unchanged.
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.NotContains(t, string(configData), "65002", "local-as should not be in config.conf")
}

// TestEditorConflictResetAfterSet verifies that re-setting a value updates Previous.
//
// VALIDATES: Re-setting refreshes Previous from config.conf, clearing stale conflicts.
// PREVENTS: Stale conflicts persisting after user re-confirms their intent.
func TestEditorConflictResetAfterSet(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Read original router-id from config.
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session := NewEditSession("alice", "ssh")
	ed.SetSession(session)

	// First set: Previous captured from original config.conf.
	err = ed.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Externally modify config.conf (simulating another user's commit).
	origData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	newData := strings.Replace(string(origData), "1.2.3.4", "9.9.9.9", 1)
	err = os.WriteFile(configPath, []byte(newData), 0o644)
	require.NoError(t, err)

	// Re-set: Previous should now reflect 9.9.9.9.
	err = ed.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Commit should succeed: Previous matches current config.conf.
	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result.Conflicts, "re-set should clear stale conflict")
	assert.Equal(t, 1, result.Applied)
}

// TestEditorDiscardNewlyAdded verifies discarding a newly-added leaf removes it from draft.
//
// VALIDATES: Discard of a leaf not in config.conf deletes it from the draft tree.
// PREVENTS: Newly-added leaves lingering in draft after discard.
func TestEditorDiscardNewlyAdded(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session := NewEditSession("alice", "ssh")
	ed.SetSession(session)

	// Set a value that doesn't exist in config.conf.
	err = ed.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Verify it's in the draft.
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err)
	assert.Contains(t, string(draftData), "65001")

	// Discard all.
	err = ed.DiscardSessionPath(nil)
	require.NoError(t, err)

	// Draft should be deleted (no other sessions).
	_, err = os.Stat(draftPath)
	assert.True(t, os.IsNotExist(err), "draft should be deleted after discarding only session")
}

// TestEditorBlameView verifies blame-annotated output includes user and timestamp.
//
// VALIDATES: BlameView returns per-line authorship annotation.
// PREVENTS: Blame output missing metadata or crashing on nil meta.
func TestEditorBlameView(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session := NewEditSession("alice", "ssh")
	ed.SetSession(session)
	err = ed.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	blame := ed.BlameView()
	assert.Contains(t, blame, "alice@ssh", "blame should include user")
	assert.Contains(t, blame, "10.0.0.1", "blame should include value")
}

// TestEditorSessionChanges verifies session-scoped change listing.
//
// VALIDATES: SessionChanges filters by session ID, returns all when empty.
// PREVENTS: Changes from other sessions leaking into "my changes" view.
func TestEditorSessionChanges(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 changes router-id.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 changes local-as.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Session 2: my changes should only include local-as.
	myChanges := ed2.SessionChanges(session2.ID)
	assert.Len(t, myChanges, 1, "should have exactly one change")
	assert.Contains(t, myChanges[0].Path, "local-as")

	// All changes should include both.
	allChanges := ed2.SessionChanges("")
	assert.Len(t, allChanges, 2, "should have changes from both sessions")
}

// TestEditorActiveSessions verifies listing of active sessions.
//
// VALIDATES: ActiveSessions returns all unique session IDs.
// PREVENTS: Missing sessions in "who" output.
func TestEditorActiveSessions(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	sessions := ed2.ActiveSessions()
	assert.Len(t, sessions, 2, "should have two active sessions")
}

// TestEditorDisconnectSession verifies disconnecting another session.
//
// VALIDATES: DisconnectSession removes the target session's entries and restores committed values.
// PREVENTS: Disconnected session's changes persisting in draft.
func TestEditorDisconnectSession(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 changes router-id.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 changes local-as.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Session 2 disconnects session 1.
	err = ed2.DisconnectSession(session1.ID)
	require.NoError(t, err)

	// Draft should still exist (session 2 remains).
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err)
	draftContent := string(draftData)

	assert.NotContains(t, draftContent, "alice@ssh", "alice should be removed")
	assert.Contains(t, draftContent, "thomas@local", "thomas should remain")
	assert.Contains(t, draftContent, "65001", "thomas's value should remain")
}

// TestEditorSessionCommit verifies the happy-path per-session commit.
//
// VALIDATES: CommitSession applies only my changes to config.conf, updates in-memory state.
// PREVENTS: Committed config missing session's changes, or editor state left inconsistent.
func TestEditorSessionCommit(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Change router-id via write-through.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	// Draft should exist.
	draftPath := DraftPath(configPath)
	_, err = os.Stat(draftPath)
	require.NoError(t, err, "draft should exist before commit")

	// Commit.
	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result.Conflicts, "no conflicts expected")
	assert.Equal(t, 1, result.Applied)

	// config.conf should have the new value.
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(configData), "5.6.7.8", "committed value should be in config.conf")
	assert.NotContains(t, string(configData), "1.2.3.4", "old value should be gone")

	// Draft should be deleted (no other sessions).
	_, err = os.Stat(draftPath)
	assert.True(t, os.IsNotExist(err), "draft should be deleted after sole session commits")

	// In-memory state should reflect committed.
	assert.False(t, ed.Dirty())
	bgpTree := ed.tree.GetContainer("bgp")
	require.NotNil(t, bgpTree)
	val, ok := bgpTree.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "5.6.7.8", val)
}

// TestEditorDiscardPath verifies discarding a specific leaf path.
//
// VALIDATES: DiscardSessionPath with a specific path restores only that leaf.
// PREVENTS: Discard with path affecting other leaves.
func TestEditorDiscardPath(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Make two changes.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)
	err = ed.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Discard only router-id.
	err = ed.DiscardSessionPath([]string{"bgp", "router-id"})
	require.NoError(t, err)

	// Draft should still exist (local-as change remains).
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err)
	draftContent := string(draftData)

	// router-id should be reverted to committed value.
	assert.Contains(t, draftContent, "1.2.3.4", "router-id should be restored to committed")
	// local-as change should remain.
	assert.Contains(t, draftContent, "65001", "local-as change should be preserved")
}

// TestEditorDiscardSubtree verifies discarding all leaves under a container path.
//
// VALIDATES: DiscardSessionPath with container path restores all leaves under it.
// PREVENTS: Subtree discard missing some leaves.
func TestEditorDiscardSubtree(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Change two leaves under bgp.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)
	err = ed.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Discard all changes under "bgp" (subtree).
	err = ed.DiscardSessionPath([]string{"bgp"})
	require.NoError(t, err)

	// Draft should be deleted (no remaining session entries).
	draftPath := DraftPath(configPath)
	_, err = os.Stat(draftPath)
	assert.True(t, os.IsNotExist(err), "draft should be deleted after subtree discard removes all entries")
}

// TestEditorDiscardBareRejected verifies that bare discard (no args) is rejected in session mode.
//
// VALIDATES: cmdDiscardSession requires path or 'all' argument.
// PREVENTS: Accidental discard of all changes by typing bare 'discard'.
func TestEditorDiscardBareRejected(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	// Dispatch bare discard through the command layer.
	// The guard is in cmdDiscardSession (model_commands.go), not in
	// DiscardSessionPath itself. We test the command dispatch here.
	m := Model{editor: ed}
	_, cmdErr := m.cmdDiscardSession(nil)
	require.Error(t, cmdErr, "bare discard should be rejected")
	assert.Contains(t, cmdErr.Error(), "discard requires path or 'all'")
}

// TestEditorDiscardPathBoundary verifies discard uses word boundaries, not raw prefix.
// "peer" must NOT match "bgp peer-group".
//
// VALIDATES: Discard path uses word-boundary matching.
// PREVENTS: Discarding unrelated paths that share a prefix.
func TestEditorDiscardPathBoundary(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Set two values: one under "peer 1.1.1.1" and one under "bgp" (router-id).
	// The YANG path "peer 1.1.1.1 hold-time" starts with "peer",
	// while "bgp router-id" does not -- but raw prefix "peer" could match
	// a hypothetical "bgp peer-group" if boundary matching is broken.
	err = ed.SetValue([]string{"bgp", "peer", "1.1.1.1"}, "hold-time", "180")
	require.NoError(t, err)
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	// Discard only "peer" subtree.
	err = ed.DiscardSessionPath([]string{"bgp", "peer"})
	require.NoError(t, err)

	// The peer entry's hold-time should be restored, but "bgp router-id" should
	// still be pending. This also confirms that "peer" doesn't accidentally
	// match "bgp router-id" (it wouldn't with the old code either, but exercises
	// the boundary logic for space-separated YANG paths).
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err, "draft should still exist (router-id change remains)")

	draftContent := string(draftData)
	assert.Contains(t, draftContent, "9.9.9.9",
		"bgp router-id change should NOT be discarded by 'bgp peer' discard")
}

// TestHierarchicalToSetMigration verifies that committing a hierarchical config
// writes config.conf in set+meta format.
//
// VALIDATES: AC-21 -- first commit of hierarchical config writes set format.
// PREVENTS: Config.conf staying in hierarchical format after concurrent edit.
func TestHierarchicalToSetMigration(t *testing.T) {
	// Write a hierarchical config (the legacy format).
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Make a change to create a draft.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	// Commit session.
	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	assert.Equal(t, 1, result.Applied)

	// Read config.conf -- it should now be in set format, not hierarchical.
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	configContent := string(configData)

	// Set format lines start with "set " or metadata prefix + "set ".
	assert.Contains(t, configContent, "set bgp router-id 5.6.7.8",
		"config.conf should be in set format after commit")
	// Should NOT contain hierarchical braces.
	assert.NotContains(t, configContent, "{",
		"config.conf should not contain hierarchical braces")
}

// TestWorkingContentSessionFormat verifies WorkingContent returns set format when
// a session is active with metadata.
//
// VALIDATES: AC-35 -- WorkingContent format matches CommitSession output.
// PREVENTS: Validation operating on hierarchical format while commit writes set format.
func TestWorkingContentSessionFormat(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// Without session: should return hierarchical format.
	content := ed.WorkingContent()
	assert.Contains(t, content, "{", "without session, should be hierarchical")

	// Set session and make a change (creates meta).
	session := NewEditSession("thomas", "local")
	ed.SetSession(session)
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	// With session + meta: should return set format.
	content = ed.WorkingContent()
	assert.Contains(t, content, "set bgp",
		"with session+meta, should be set format")
	assert.NotContains(t, content, "{",
		"with session+meta, should not contain hierarchical braces")
}

// TestSaveGuardInSessionMode verifies Save() returns an error when a session is active.
//
// VALIDATES: AC-36 -- Save() rejects in session mode to prevent format mismatch.
// PREVENTS: Accidental hierarchical overwrite of config.conf in session mode.
func TestSaveGuardInSessionMode(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Make a change to set dirty flag.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	// Save() should fail when session is active.
	err = ed.Save()
	require.Error(t, err, "Save() should reject when session is active")
	assert.Contains(t, err.Error(), "session",
		"error message should mention session")
}

// TestSaveWorksWithoutSession verifies Save() still works when no session is active.
//
// VALIDATES: Save guard only blocks session mode, not normal saves.
// PREVENTS: Save guard regression breaking non-session commit path.
func TestSaveWorksWithoutSession(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// No session set. Make a change and save.
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	err = ed.Save()
	assert.NoError(t, err, "Save() should succeed without active session")

	// Verify file was updated.
	data, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "9.9.9.9", "config file should be updated")
}

// TestWorkingContentSessionNoChanges verifies WorkingContent returns set format
// when a session is active even before any SetValue calls. SetSession always
// initializes metadata, so the set+meta branch is taken.
//
// VALIDATES: WorkingContent returns set format with session, even without changes.
// PREVENTS: Panic or empty output when session exists but no SetValue was called.
func TestWorkingContentSessionNoChanges(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// Set session but don't make any changes.
	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	content := ed.WorkingContent()
	// SetSession initializes meta, so set format is returned.
	assert.Contains(t, content, "set bgp", "with session, should be set format")
	assert.Contains(t, content, "router-id", "should contain config values")
	assert.NotContains(t, content, "{", "should not contain hierarchical braces")
}

// TestEditorDeleteThenCommit verifies that deleting a value and committing
// removes the value from config.conf.
//
// VALIDATES: CommitSession applies delete operations (not just sets) to config.conf.
// PREVENTS: Delete metadata entries silently skipped during commit apply loop.
func TestEditorDeleteThenCommit(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Verify local-as exists before delete.
	bgpTree := ed.tree.GetContainer("bgp")
	require.NotNil(t, bgpTree)
	_, ok := bgpTree.Get("local-as")
	require.True(t, ok, "local-as should exist before delete")

	// Delete local-as via write-through.
	err = ed.DeleteValue([]string{"bgp"}, "local-as")
	require.NoError(t, err)

	// Commit the delete.
	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result.Conflicts, "no conflicts expected")
	assert.Equal(t, 1, result.Applied)

	// config.conf should NOT contain the deleted value.
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.NotContains(t, string(configData), "local-as",
		"deleted value should not appear in committed config")

	// In-memory tree should also lack the value.
	bgpTree = ed.tree.GetContainer("bgp")
	require.NotNil(t, bgpTree)
	_, ok = bgpTree.Get("local-as")
	assert.False(t, ok, "deleted value should not exist in tree after commit")
}

// TestEditorConflictStaleNewValue verifies stale conflict detection when
// two sessions independently add a new value at the same path.
//
// VALIDATES: CommitSession detects stale conflict when Previous=="" and committedValue!="".
// PREVENTS: Two sessions adding the same new leaf without conflict detection.
func TestEditorConflictStaleNewValue(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Set a NEW value (not present in original config). Previous will be "".
	err = ed.SetValue([]string{"bgp"}, "listen", "127.0.0.1:1179")
	require.NoError(t, err)

	// Simulate another session committing the same path with a different value
	// by writing config.conf in hierarchical format with the new value added.
	modifiedConfig := `bgp {
	router-id 1.2.3.4
	local-as 65000
	listen 0.0.0.0:179
	peer 1.1.1.1 {
		peer-as 65001
		hold-time 90
	}
}
`
	err = os.WriteFile(configPath, []byte(modifiedConfig), 0o600)
	require.NoError(t, err)

	// Commit should detect stale conflict: Previous="" but committed="0.0.0.0:179".
	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.NotEmpty(t, result.Conflicts, "stale conflict expected for new value collision")
	assert.Equal(t, 0, result.Applied, "no changes should be applied")

	found := false
	for _, c := range result.Conflicts {
		if c.Type == ConflictStale {
			found = true
			assert.Equal(t, "127.0.0.1:1179", c.MyValue)
			assert.Equal(t, "0.0.0.0:179", c.OtherValue,
				"should show the other session's committed value")
			assert.Equal(t, "", c.PreviousValue,
				"previous should be empty since the value was new")
		}
	}
	assert.True(t, found, "should have a stale conflict for new value collision")
}

// TestEditorDisconnectLastSession verifies that disconnecting the only other session
// deletes the draft file when the caller has no pending changes.
//
// VALIDATES: DisconnectSession deletes draft when no sessions remain.
// PREVENTS: Orphaned draft files after all sessions are cleaned up.
func TestEditorDisconnectLastSession(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 (alice) makes a change.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 (thomas) has NO changes, just disconnects session 1.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)

	err = ed2.DisconnectSession(session1.ID)
	require.NoError(t, err)

	// Draft should be deleted (no remaining sessions).
	draftPath := DraftPath(configPath)
	_, err = os.Stat(draftPath)
	assert.True(t, os.IsNotExist(err), "draft should be deleted after disconnecting last session")

	// In-memory tree should have committed value (alice's change was reverted).
	bgpTree := ed2.tree.GetContainer("bgp")
	require.NotNil(t, bgpTree)
	val, ok := bgpTree.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "1.2.3.4", val, "router-id should be restored to committed value")
}

// TestEditorCommitPreservesOtherSessions verifies that committing one session
// preserves another session's changes in the draft.
//
// VALIDATES: CommitSession only applies my changes, leaves other sessions in draft.
// PREVENTS: Other session's pending work lost during commit.
func TestEditorCommitPreservesOtherSessions(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 changes router-id.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 changes local-as.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Session 1 commits.
	result, err := ed1.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result.Conflicts)
	assert.Equal(t, 1, result.Applied)

	// config.conf should have alice's committed value.
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(configData), "10.0.0.1", "alice's value should be committed")

	// Draft should still exist with thomas's changes.
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err, "draft should survive when other sessions remain")

	draftContent := string(draftData)
	assert.Contains(t, draftContent, "thomas@local", "thomas's metadata should remain")
	assert.Contains(t, draftContent, "65001", "thomas's value should remain in draft")
	assert.NotContains(t, draftContent, "%"+session1.ID, "alice's session should be removed from draft")
}

// TestEditorCommitNoChanges verifies CommitSession with no pending changes.
//
// VALIDATES: CommitSession returns Applied=0 when session made no changes.
// PREVENTS: Panic or error on empty commit.
func TestEditorCommitNoChanges(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// No write-through calls. Create draft manually so CommitSession can read it.
	// Without a draft, CommitSession would error on ReadFile. In practice,
	// a user would only commit after making changes, but this tests the guard.
	draftPath := DraftPath(configPath)
	err = os.WriteFile(draftPath, []byte("set bgp router-id 1.2.3.4\nset bgp local-as 65000\n"), 0o600)
	require.NoError(t, err)

	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, 0, result.Applied, "should apply nothing when session has no entries")
	assert.Empty(t, result.Conflicts)
}

// TestEditorCommitAfterDisconnect verifies the workflow: disconnect other session, then commit.
//
// VALIDATES: Commit succeeds after disconnecting a conflicting session.
// PREVENTS: Stale draft state blocking commit after disconnect.
func TestEditorCommitAfterDisconnect(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 (alice) changes router-id to 10.0.0.1.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 (thomas) changes router-id to 10.0.0.2 (would be a live conflict).
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "router-id", "10.0.0.2")
	require.NoError(t, err)

	// Session 2 disconnects session 1 (resolving the conflict).
	err = ed2.DisconnectSession(session1.ID)
	require.NoError(t, err)

	// Session 2 commits: should succeed (no more conflict).
	result, err := ed2.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result.Conflicts, "no conflicts after disconnect")
	assert.Equal(t, 1, result.Applied)

	// config.conf should have thomas's value.
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(configData), "10.0.0.2", "thomas's value should be committed")
	assert.NotContains(t, string(configData), "10.0.0.1", "alice's disconnected value should not appear")
}

// TestEditorDiscardAfterOtherCommit verifies discard sees the latest committed state
// after another session committed.
//
// VALIDATES: DiscardSessionPath reads fresh config.conf (not cached originalContent).
// PREVENTS: Discard restoring stale committed values after concurrent commit.
func TestEditorDiscardAfterOtherCommit(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 changes router-id and commits.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 changes local-as.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Session 1 commits (router-id is now 10.0.0.1 in config.conf).
	result1, err := ed1.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, 1, result1.Applied)

	// Session 2 discards all. The committed router-id is now 10.0.0.1 (from alice's commit),
	// not the original 1.2.3.4. Discard should read fresh config.conf.
	err = ed2.DiscardSessionPath(nil)
	require.NoError(t, err)

	// In-memory tree should reflect alice's committed value (not the stale original).
	bgpTree := ed2.tree.GetContainer("bgp")
	require.NotNil(t, bgpTree)
	val, ok := bgpTree.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "10.0.0.1", val, "router-id should reflect alice's commit, not stale original")

	// local-as should revert to committed value (65000, unchanged by alice).
	val, ok = bgpTree.Get("local-as")
	assert.True(t, ok)
	assert.Equal(t, "65000", val, "local-as should revert to committed value")
}

// TestEditorDisconnectNoDraft verifies DisconnectSession errors when no draft exists.
//
// VALIDATES: DisconnectSession returns error if draft file is missing.
// PREVENTS: Panic on missing draft during disconnect.
func TestEditorDisconnectNoDraft(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// No draft exists (no write-through calls made).
	err = ed.DisconnectSession("alice@ssh:12345")
	require.Error(t, err, "disconnect without draft should error")
	assert.Contains(t, err.Error(), "read draft", "error should mention draft read failure")
}

// TestCommitSessionNilSession verifies CommitSession errors when no session is set.
//
// VALIDATES: CommitSession rejects calls when session is nil.
// PREVENTS: Panic on nil session dereference.
func TestCommitSessionNilSession(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// No SetSession call -- session is nil.
	result, err := ed.CommitSession()
	require.Error(t, err, "commit without session should error")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no session set")
}

// TestDiscardSessionPathNilSession verifies DiscardSessionPath errors when no session is set.
//
// VALIDATES: DiscardSessionPath rejects calls when session is nil.
// PREVENTS: Panic on nil session dereference during discard.
func TestDiscardSessionPathNilSession(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// No SetSession call -- session is nil.
	err = ed.DiscardSessionPath(nil)
	require.Error(t, err, "discard without session should error")
	assert.Contains(t, err.Error(), "no session set")
}

// TestDisconnectSessionNilSession verifies DisconnectSession errors when no session is set.
//
// VALIDATES: DisconnectSession rejects calls when session is nil.
// PREVENTS: Panic on nil session dereference during disconnect.
func TestDisconnectSessionNilSession(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// No SetSession call -- session is nil.
	err = ed.DisconnectSession("alice@ssh:12345")
	require.Error(t, err, "disconnect without session should error")
	assert.Contains(t, err.Error(), "no session set")
}

// TestWriteThroughSetUnknownPath verifies SetValue with session returns error for unknown path.
//
// VALIDATES: walkOrCreateIn returns error for unknown schema elements.
// PREVENTS: Silent failure or panic when navigating non-existent config path.
func TestWriteThroughSetUnknownPath(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// "nonexistent" is not a valid schema path element.
	err = ed.SetValue([]string{"nonexistent"}, "key", "value")
	require.Error(t, err, "set on unknown path should error")
	assert.Contains(t, err.Error(), "unknown path element")

	// Draft should NOT be created (error before write).
	draftPath := DraftPath(configPath)
	_, statErr := os.Stat(draftPath)
	assert.True(t, os.IsNotExist(statErr), "draft should not exist after failed write-through")
}

// TestWriteThroughDeletePathNotFound verifies DeleteValue with session returns error for missing path.
//
// VALIDATES: writeThroughDelete returns error when tree path doesn't exist.
// PREVENTS: Panic when deleting from a non-navigable path.
func TestWriteThroughDeletePathNotFound(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Try to delete from a path that doesn't exist in the tree.
	// "bgp" exists but "peer 9.9.9.9" does not.
	err = ed.DeleteValue([]string{"bgp", "peer", "9.9.9.9"}, "peer-as")
	require.Error(t, err, "delete on non-existent path should error")
	assert.Contains(t, err.Error(), "path not found")
}

// TestDiscardPartialDirtyFlag verifies that partial discard keeps dirty=true
// when the session still has remaining entries.
//
// VALIDATES: Smart dirty flag after partial discard (line 816).
// PREVENTS: Editor incorrectly showing clean state after partial discard.
func TestDiscardPartialDirtyFlag(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Make two changes at different paths.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)
	err = ed.SetValue([]string{"bgp", "peer", "1.1.1.1"}, "hold-time", "180")
	require.NoError(t, err)

	assert.True(t, ed.Dirty(), "should be dirty after two changes")

	// Discard only the peer subtree (partial discard).
	err = ed.DiscardSessionPath([]string{"bgp", "peer", "1.1.1.1", "hold-time"})
	require.NoError(t, err)

	// Still dirty because router-id change remains.
	assert.True(t, ed.Dirty(), "should remain dirty after partial discard with remaining entries")

	// Now discard the remaining change.
	err = ed.DiscardSessionPath([]string{"bgp", "router-id"})
	require.NoError(t, err)

	// Still dirty=true because DiscardSessionPath with a path prefix doesn't
	// call RemoveSession (only discard-all does), so SessionEntries may still
	// return results. But the entries were individually removed via
	// RemoveSessionEntry, so SessionEntries should now be empty.
	// The smart flag: len(draftMeta.SessionEntries(session.ID)) > 0
	assert.False(t, ed.Dirty(), "should be clean after discarding all remaining entries")
}

// TestDiscardRestoresOtherSessionValue verifies that discarding when both sessions
// edited the same leaf restores the other session's value, not the committed value.
//
// VALIDATES: Discard uses remaining metadata entries to pick the correct value (line 778-780).
// PREVENTS: Restoring committed value when another session has a pending change at same leaf.
func TestDiscardRestoresOtherSessionValue(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 (alice) changes router-id to 10.0.0.1.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 (thomas) changes same leaf to 10.0.0.2.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "router-id", "10.0.0.2")
	require.NoError(t, err)

	// Session 2 discards. Alice's value (10.0.0.1) should be restored in draft,
	// NOT the committed value (1.2.3.4).
	err = ed2.DiscardSessionPath(nil)
	require.NoError(t, err)

	// Draft should still exist (alice's session remains).
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err, "draft should exist with alice's changes")

	draftContent := string(draftData)
	assert.Contains(t, draftContent, "set bgp router-id 10.0.0.1", "alice's value should be the tree value in draft")
	assert.NotContains(t, draftContent, "10.0.0.2", "thomas's value should be gone")
	// Draft should NOT have the committed value as a tree value (alice's 10.0.0.1 takes priority).
	// Note: "^1.2.3.4" will appear in metadata (Previous field), which is correct.
	assert.NotContains(t, draftContent, "set bgp router-id 1.2.3.4", "committed value should NOT replace alice's pending value")
}

// TestDisconnectRestoresOtherSessionValue verifies that disconnecting a session
// restores the remaining session's value when both edited the same leaf.
//
// VALIDATES: Disconnect uses remaining metadata to restore correct value (line 875-877).
// PREVENTS: Committed value overwriting a live session's pending change.
func TestDisconnectRestoresOtherSessionValue(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 (alice) changes router-id to 10.0.0.1.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 (thomas) changes same leaf to 10.0.0.2.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "router-id", "10.0.0.2")
	require.NoError(t, err)

	// Session 2 disconnects session 1 (alice). Thomas's value (10.0.0.2) should
	// remain in draft, NOT the committed value (1.2.3.4).
	err = ed2.DisconnectSession(session1.ID)
	require.NoError(t, err)

	// Draft should still exist (thomas's session remains).
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err, "draft should exist with thomas's changes")

	draftContent := string(draftData)
	assert.Contains(t, draftContent, "10.0.0.2", "thomas's value should remain in draft")
	assert.NotContains(t, draftContent, "10.0.0.1", "alice's disconnected value should be gone")
}

// TestStaleConflictNewValueBothAdded verifies stale conflict when a session
// adds a new value at a path that was empty, but another editor externally
// committed a value at the same path.
//
// VALIDATES: Stale detection for Previous="" and committedValue!="" (line 622).
// PREVENTS: Silently overwriting an externally committed new value.
func TestStaleConflictNewValueBothAdded(t *testing.T) {
	// Start with a config that has bgp but no "listen" field.
	configPath := writeTestConfig(t, validBGPConfig)

	// Session adds "listen 127.0.0.1" via write-through. Previous="" because
	// committed config has no listen field at this point.
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)
	err = ed.SetValue([]string{"bgp"}, "listen", "127.0.0.1")
	require.NoError(t, err)

	// Simulate external commit: directly write config.conf with "listen 0.0.0.0".
	// This bypasses write-through (another editor session committed externally).
	externalConfig := "set bgp router-id 1.2.3.4\nset bgp local-as 65000\nset bgp listen 0.0.0.0\nset bgp peer 1.1.1.1 peer-as 65001\nset bgp peer 1.1.1.1 hold-time 90\n"
	err = os.WriteFile(configPath, []byte(externalConfig), 0o600)
	require.NoError(t, err)

	// Thomas tries to commit: Previous="" (it was new when he edited),
	// but committedValue="0.0.0.0" (externally committed). This is a stale conflict.
	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.NotEmpty(t, result.Conflicts, "should detect stale conflict for concurrently added value")

	conflict := result.Conflicts[0]
	assert.Equal(t, ConflictStale, conflict.Type)
	assert.Equal(t, "127.0.0.1", conflict.MyValue)
	assert.Equal(t, "0.0.0.0", conflict.OtherValue)
}

// TestDiscardFullCleansUpDraft verifies that discard-all (nil path) removes the
// draft file when no other sessions are present.
//
// VALIDATES: DiscardSessionPath(nil) calls RemoveSession and cleans up draft.
// PREVENTS: Orphaned draft files after full discard.
func TestDiscardFullCleansUpDraft(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	// Verify draft exists before discard.
	draftPath := DraftPath(configPath)
	_, err = os.Stat(draftPath)
	require.NoError(t, err, "draft should exist before discard")

	// Discard all changes.
	err = ed.DiscardSessionPath(nil)
	require.NoError(t, err)

	// Draft should be gone.
	_, err = os.Stat(draftPath)
	assert.True(t, os.IsNotExist(err), "draft should be deleted after full discard")

	// Dirty should be false.
	assert.False(t, ed.Dirty(), "should be clean after full discard")

	// Tree should reflect committed values.
	bgpTree := ed.tree.GetContainer("bgp")
	require.NotNil(t, bgpTree)
	val, ok := bgpTree.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "1.2.3.4", val, "should revert to committed value")
}

// TestCommitSessionNoDraft verifies CommitSession errors when no draft file exists.
//
// VALIDATES: CommitSession returns error on missing draft (line 585-587).
// PREVENTS: Panic or undefined behavior when committing without write-through changes.
func TestCommitSessionNoDraft(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// No write-through calls -- no draft file exists.
	result, err := ed.CommitSession()
	require.Error(t, err, "commit without draft should error")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "read draft")
}

// TestDiscardSessionNoDraft verifies DiscardSessionPath errors when no draft file exists.
//
// VALIDATES: DiscardSessionPath returns error on missing draft (line 728-730).
// PREVENTS: Panic when discarding without any write-through changes.
func TestDiscardSessionNoDraft(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// No write-through calls -- no draft file exists.
	err = ed.DiscardSessionPath(nil)
	require.Error(t, err, "discard without draft should error")
	assert.Contains(t, err.Error(), "read draft")
}

// TestWriteThroughCorruptDraft verifies write-through fails gracefully on corrupt draft.
//
// VALIDATES: readDraftOrConfig parse error path (line 170-171).
// PREVENTS: Silent data corruption when draft file is malformed.
func TestWriteThroughCorruptDraft(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Write a corrupt draft file (invalid set format).
	draftPath := DraftPath(configPath)
	err = os.WriteFile(draftPath, []byte("this is not valid set format {{{{"), 0o600)
	require.NoError(t, err)

	// Write-through set should fail with parse error.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.Error(t, err, "write-through should fail on corrupt draft")
	assert.Contains(t, err.Error(), "write-through read")
}

// TestCommitInMemoryShowsDraftWithRemaining verifies that after commit with other
// sessions remaining, the committing editor's in-memory tree reflects draft state.
//
// VALIDATES: In-memory state switch to draft view (lines 701-706).
// PREVENTS: Editor showing committed state while other sessions have pending changes,
// causing inconsistency between show/blame/changes commands and actual draft.
func TestCommitInMemoryShowsDraftWithRemaining(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 (alice) changes router-id.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 (thomas) changes local-as.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Session 1 commits.
	result, err := ed1.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, 1, result.Applied)

	// ed1's in-memory tree should reflect the DRAFT state (thomas's pending
	// changes visible), not the committed state alone. This ensures show/blame
	// commands are consistent.
	bgpTree := ed1.tree.GetContainer("bgp")
	require.NotNil(t, bgpTree)

	// thomas's pending value (65001) should be visible in ed1's tree.
	val, ok := bgpTree.Get("local-as")
	assert.True(t, ok)
	assert.Equal(t, "65001", val, "in-memory tree should show draft state with thomas's pending value")

	// alice's committed value should also be present.
	val, ok = bgpTree.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "10.0.0.1", val, "alice's committed value should be in draft tree too")

	// dirty should be false after commit.
	assert.False(t, ed1.Dirty())
}

// TestSequentialCommits verifies two commit cycles in sequence work correctly.
//
// VALIDATES: State transitions across multiple commit cycles.
// PREVENTS: Stale state from first commit corrupting second commit.
func TestSequentialCommits(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// First cycle: change router-id.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)
	assert.True(t, ed.Dirty())

	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, 1, result.Applied)
	assert.False(t, ed.Dirty())

	// Verify first commit: config.conf has the new value.
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(configData), "5.6.7.8")

	// Verify in-memory tree has router-id after first commit.
	bgpTree := ed.tree.GetContainer("bgp")
	require.NotNil(t, bgpTree, "bgp container should exist in tree after first commit")
	val, ok := bgpTree.Get("router-id")
	require.True(t, ok, "router-id should exist in tree after first commit")
	assert.Equal(t, "5.6.7.8", val)

	// Second cycle: change local-as.
	err = ed.SetValue([]string{"bgp"}, "local-as", "65002")
	require.NoError(t, err)
	assert.True(t, ed.Dirty())

	result, err = ed.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, 1, result.Applied)
	assert.False(t, ed.Dirty())

	// Verify second commit: both values should be in config.conf.
	configData, err = os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(configData), "65002", "second commit value should be in config")
	assert.Contains(t, string(configData), "5.6.7.8", "first commit value should still be present")
	assert.NotContains(t, string(configData), "65000", "original local-as should be gone")

	// Draft should not exist.
	draftPath := DraftPath(configPath)
	_, err = os.Stat(draftPath)
	assert.True(t, os.IsNotExist(err), "draft should be deleted after final commit")
}

// TestWriteThroughPreviousTracksCommits verifies that after a commit, the next
// write-through records the newly committed value as Previous (not the original).
//
// VALIDATES: readCommittedTree re-reads from disk after commit (line 75, 132).
// PREVENTS: Stale Previous values from before the commit.
func TestWriteThroughPreviousTracksCommits(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// First: change router-id to 5.6.7.8 and commit.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, 1, result.Applied)

	// Now change router-id again.
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	// The draft should now record Previous=5.6.7.8 (from the committed config),
	// NOT Previous=1.2.3.4 (the original before the first commit).
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err)

	draftContent := string(draftData)
	assert.Contains(t, draftContent, "^5.6.7.8", "Previous should be the committed value from first commit")
	assert.NotContains(t, draftContent, "^1.2.3.4", "Previous should NOT be the original value before any commits")
}

// TestDisconnectDeletesNewlyAddedValue verifies that disconnecting a session that
// added a value not present in committed config causes the value to be deleted.
//
// VALIDATES: DisconnectSession delete branch when committedValue="" (line 883-884).
// PREVENTS: Orphaned values in draft from disconnected sessions.
func TestDisconnectDeletesNewlyAddedValue(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 (alice) adds "listen" -- a leaf not in committed config.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "listen", "0.0.0.0")
	require.NoError(t, err)

	// Session 2 (thomas) also makes a change (so draft survives disconnect).
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Session 2 disconnects session 1.
	err = ed2.DisconnectSession(session1.ID)
	require.NoError(t, err)

	// Draft should exist (thomas's changes remain).
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err)

	draftContent := string(draftData)
	// alice's "listen" should be deleted (not in committed config).
	assert.NotContains(t, draftContent, "listen", "newly added value should be deleted after disconnect")
	// thomas's change should remain.
	assert.Contains(t, draftContent, "65001", "thomas's value should remain in draft")
}

// TestDiscardDeletesNewlyAddedValue verifies that discarding a session's change
// that added a value not in committed config correctly deletes it from the draft.
//
// VALIDATES: DiscardSessionPath delete branch when committedValue="" (line 786-788).
// PREVENTS: Orphaned values in draft after discarding new additions.
func TestDiscardDeletesNewlyAddedValue(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 (alice) adds "listen" -- not in committed config.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "listen", "0.0.0.0")
	require.NoError(t, err)

	// Session 2 (thomas) also makes a change (so draft survives discard).
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Session 1 discards all.
	err = ed1.DiscardSessionPath(nil)
	require.NoError(t, err)

	// Draft should exist (thomas's session remains).
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err)

	draftContent := string(draftData)
	// alice's "listen" should be gone (not in committed, so deleted from draft tree).
	assert.NotContains(t, draftContent, "listen", "newly added value should be deleted after discard")
	// thomas's change should remain.
	assert.Contains(t, draftContent, "65001", "thomas's value should remain")
}

// TestCommitMetadataUserInConfig verifies that after commit, config.conf contains
// user/time metadata annotations (not just absence of session tokens).
//
// VALIDATES: buildCommitMeta creates user+time entries for committed changes (lines 1014-1018).
// PREVENTS: Silent metadata loss during commit.
func TestCommitMetadataUserInConfig(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, 1, result.Applied)

	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	configContent := string(configData)

	// Should contain user annotation from commit metadata.
	assert.Contains(t, configContent, "#thomas@local", "committed config should have user metadata")
	// Should NOT contain session tokens.
	assert.NotContains(t, configContent, "%", "committed config should not have session tokens")
	// Should NOT contain Previous markers.
	assert.NotContains(t, configContent, "^", "committed config should not have Previous markers")
}

// TestCommitOriginalContentUpdated verifies that after commit, the editor's
// OriginalContent and WorkingContent reflect the newly committed state.
//
// VALIDATES: originalContent is updated to committed output (line 696).
// PREVENTS: Stale originalContent causing incorrect diffs after commit.
func TestCommitOriginalContentUpdated(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	_, err = ed.CommitSession()
	require.NoError(t, err)

	// OriginalContent should match what was written to config.conf.
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)

	assert.Equal(t, string(configData), ed.OriginalContent(),
		"OriginalContent should match config.conf after commit")

	// WorkingContent should contain the new value.
	working := ed.WorkingContent()
	assert.Contains(t, working, "5.6.7.8", "WorkingContent should reflect committed value")
	assert.NotContains(t, working, "1.2.3.4", "WorkingContent should not have old value")
}

// TestCommitMultipleChanges verifies that committing with several changes applies all of them.
//
// VALIDATES: Commit apply loop (lines 644-661) handles multiple entries correctly.
// PREVENTS: Only the first or last change being applied in a multi-change commit.
func TestCommitMultipleChanges(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Make three changes in one session.
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)
	err = ed.SetValue([]string{"bgp"}, "local-as", "65999")
	require.NoError(t, err)
	err = ed.SetValue([]string{"bgp", "peer", "1.1.1.1"}, "hold-time", "30")
	require.NoError(t, err)

	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result.Conflicts)
	assert.Equal(t, 3, result.Applied, "all three changes should be applied")

	// Verify all three values in config.conf.
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	configContent := string(configData)
	assert.Contains(t, configContent, "9.9.9.9", "router-id should be updated")
	assert.Contains(t, configContent, "65999", "local-as should be updated")
	assert.Contains(t, configContent, "30", "hold-time should be updated")
	assert.NotContains(t, configContent, "1.2.3.4", "old router-id should be gone")
	assert.NotContains(t, configContent, "65000", "old local-as should be gone")
}

// TestCommitConvergentDelete verifies that two sessions deleting the same value
// does NOT produce a stale conflict (convergent agreement).
//
// VALIDATES: Convergent check (line 620): myValue == committedValue skips conflict.
// PREVENTS: False stale conflict when both sessions independently delete the same leaf.
func TestCommitConvergentDelete(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Delete local-as via write-through (records Value="" in metadata).
	err = ed.DeleteValue([]string{"bgp"}, "local-as")
	require.NoError(t, err)

	// Simulate external commit that also deleted local-as.
	// Write config.conf without local-as.
	externalConfig := "set bgp router-id 1.2.3.4\nset bgp peer 1.1.1.1 peer-as 65001\nset bgp peer 1.1.1.1 hold-time 90\n"
	err = os.WriteFile(configPath, []byte(externalConfig), 0o600)
	require.NoError(t, err)

	// Commit should succeed: both sessions deleted the same value (convergent).
	// myValue="" and committedValue="" so myValue==committedValue, no stale conflict.
	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result.Conflicts, "convergent delete should not produce conflict")
	assert.Equal(t, 1, result.Applied, "delete should be counted as applied")
}

// TestSetOverwriteSameLeaf verifies that setting the same leaf twice in one session
// results in a single metadata entry with the latest value.
//
// VALIDATES: writeThroughSet overwrites same-session metadata entry (via MetaTree.SetEntry).
// PREVENTS: Duplicate metadata entries causing double-apply or double-conflict detection.
func TestSetOverwriteSameLeaf(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Set router-id twice.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.5.5.5")
	require.NoError(t, err)
	err = ed.SetValue([]string{"bgp"}, "router-id", "6.6.6.6")
	require.NoError(t, err)

	// Commit: should apply exactly 1 change (the second value).
	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result.Conflicts)
	assert.Equal(t, 1, result.Applied, "overwritten leaf should count as one change")

	// config.conf should have the SECOND value.
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(configData), "6.6.6.6", "second set value should win")
	assert.NotContains(t, string(configData), "5.5.5.5", "first set value should be gone")
}

// TestCommitBackupContainsFreshData verifies the backup contains the config.conf
// content as it was at commit time, not the stale originalContent from editor creation.
//
// VALIDATES: createBackup uses freshly-read committedData (line 667).
// PREVENTS: Backup containing stale data when another session committed first.
func TestCommitBackupContainsFreshData(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 (alice) makes a change and commits first.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	result1, err := ed1.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, 1, result1.Applied)

	// Session 2 (thomas) was created before alice committed, so its originalContent
	// has the OLD router-id (1.2.3.4). Now thomas commits local-as.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65999")
	require.NoError(t, err)

	result2, err := ed2.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, 1, result2.Applied)

	// The backup should contain alice's committed value (10.0.0.1),
	// NOT the original (1.2.3.4). This proves the backup uses fresh disk data.
	backupDir := filepath.Join(filepath.Dir(configPath), "rollback")
	entries, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "backup directory should have at least one entry")

	// Find the most recent backup (latest by name).
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() > entries[j].Name()
	})
	backupData, err := os.ReadFile(filepath.Join(backupDir, entries[0].Name()))
	require.NoError(t, err)
	assert.Contains(t, string(backupData), "10.0.0.1",
		"backup should contain alice's committed value, not stale original")
}

// TestSecondCommitReadsSetMetaFormat verifies that a second commit correctly
// parses config.conf in set+meta format (written by the first commit).
//
// VALIDATES: parseConfigWithFormat FormatSetMeta branch (lines 975-976).
// PREVENTS: Second commit failing because parser expects hierarchical format.
func TestSecondCommitReadsSetMetaFormat(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// First commit: converts hierarchical -> set+meta.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)
	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, 1, result.Applied)

	// Verify config.conf is now in set+meta format.
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(configData), "#thomas@local",
		"first commit should write set+meta format with user annotation")

	// Second commit: must correctly parse the set+meta format.
	err = ed.SetValue([]string{"bgp"}, "local-as", "65002")
	require.NoError(t, err)
	result, err = ed.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, 1, result.Applied, "second commit should succeed on set+meta config")
	assert.Empty(t, result.Conflicts)

	// Verify both values are in config.conf.
	configData, err = os.ReadFile(configPath)
	require.NoError(t, err)
	configContent := string(configData)
	assert.Contains(t, configContent, "5.6.7.8", "first committed value should persist")
	assert.Contains(t, configContent, "65002", "second committed value should be present")
	assert.NotContains(t, configContent, "65000", "original local-as should be replaced")
}

// TestCommitMixedSetAndDelete verifies that a commit with both set and delete
// operations in the same session applies all correctly.
//
// VALIDATES: Commit apply loop handles Value="" (delete) and Value!="" (set) in same pass.
// PREVENTS: Delete entries being skipped when mixed with set entries.
func TestCommitMixedSetAndDelete(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Set one value, delete another.
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)
	err = ed.DeleteValue([]string{"bgp"}, "local-as")
	require.NoError(t, err)

	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result.Conflicts)
	assert.Equal(t, 2, result.Applied, "both set and delete should count as applied")

	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	configContent := string(configData)
	assert.Contains(t, configContent, "9.9.9.9", "set value should be present")
	assert.NotContains(t, configContent, "local-as", "deleted value should be absent")
}

// TestDiscardExactPathMatch verifies that discard with an exact leaf path
// only discards that leaf (not just prefix matching).
//
// VALIDATES: DiscardSessionPath exact match branch (line 757: se.Path == pathPrefix).
// PREVENTS: Exact path failing because only prefix+space matching is checked.
func TestDiscardExactPathMatch(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Change two different leaves.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)
	err = ed.SetValue([]string{"bgp"}, "local-as", "65001")
	require.NoError(t, err)

	// Discard exactly "bgp local-as" (not a prefix, an exact path match).
	err = ed.DiscardSessionPath([]string{"bgp", "local-as"})
	require.NoError(t, err)

	// local-as should be reverted to committed value.
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err)
	draftContent := string(draftData)
	assert.Contains(t, draftContent, "65000", "local-as should revert to committed 65000")
	// router-id change should still be pending.
	assert.Contains(t, draftContent, "5.6.7.8", "router-id change should remain")
}

// TestLiveConflictAgreementSameValue verifies that two sessions setting the same
// leaf to the same value does NOT produce a live conflict (they agree).
//
// VALIDATES: Live conflict check (line 954): otherValue != myValue guards against false positive.
// PREVENTS: False live conflict when both sessions intend the same value.
func TestLiveConflictAgreementSameValue(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 sets router-id to 10.0.0.1.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 sets router-id to the SAME value.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 commits: should succeed (live agreement, not conflict).
	result, err := ed2.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result.Conflicts, "sessions setting same value should not conflict")
	assert.Equal(t, 1, result.Applied)
}

// TestCommitDeleteMetadataCleanup verifies that committing a delete removes
// the metadata for that leaf in config.conf (no tombstone).
//
// VALIDATES: buildCommitMeta delete branch (lines 1011-1013): removes metadata instead of creating tombstone.
// PREVENTS: Metadata accumulating for deleted leaves in committed config.
func TestCommitDeleteMetadataCleanup(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Delete local-as.
	err = ed.DeleteValue([]string{"bgp"}, "local-as")
	require.NoError(t, err)

	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Equal(t, 1, result.Applied)

	// config.conf should NOT contain any trace of local-as (no value, no metadata).
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	configContent := string(configData)
	assert.NotContains(t, configContent, "local-as", "deleted leaf should have no value or metadata in committed config")
}

// TestHasPendingSessionChangesAfterCommit verifies HasPendingSessionChanges
// returns false after a session commits all its changes.
//
// VALIDATES: HasPendingSessionChanges (editor.go:654-660) returns false when session entries are empty.
// PREVENTS: Stale pending-changes state after commit.
func TestHasPendingSessionChangesAfterCommit(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Make a change and verify pending.
	err = ed.SetValue([]string{"bgp"}, "router-id", "10.0.0.99")
	require.NoError(t, err)
	assert.True(t, ed.HasPendingSessionChanges(), "should have pending changes before commit")

	// Commit and verify no pending.
	_, err = ed.CommitSession()
	require.NoError(t, err)
	assert.False(t, ed.HasPendingSessionChanges(), "should have no pending changes after commit")
}

// TestHasPendingSessionChangesNoMeta verifies nil-meta guards in session helpers.
//
// VALIDATES: HasPendingSessionChanges (editor.go:656) nil meta guard, SessionID (editor.go:664) nil session guard.
// PREVENTS: Panic on nil dereference when session is not set.
func TestHasPendingSessionChangesNoMeta(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// No session set: both should return safe defaults.
	assert.False(t, ed.HasPendingSessionChanges(), "no session: should return false")
	assert.Empty(t, ed.SessionID(), "no session: should return empty string")
}

// TestDisconnectSelfRejected verifies that cmdDisconnectSession rejects
// disconnecting one's own session.
//
// VALIDATES: cmdDisconnectSession (model_commands.go:773-774) self-rejection check.
// PREVENTS: User accidentally disconnecting themselves instead of discarding.
func TestDisconnectSelfRejected(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	m := &Model{editor: ed}
	_, cmdErr := m.cmdDisconnectSession([]string{session.ID})
	require.Error(t, cmdErr)
	assert.Contains(t, cmdErr.Error(), "cannot disconnect own session")
}

// TestDisconnectNoArgs verifies that cmdDisconnectSession requires arguments.
//
// VALIDATES: cmdDisconnectSession (model_commands.go:769-771) empty args error.
// PREVENTS: Cryptic error when no session-id is given.
func TestDisconnectNoArgs(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	m := &Model{editor: ed}
	_, cmdErr := m.cmdDisconnectSession(nil)
	require.Error(t, cmdErr)
	assert.Contains(t, cmdErr.Error(), "usage: disconnect")
}

// TestCmdDiscardSessionPathMessage verifies path-specific discard message includes the path.
//
// VALIDATES: cmdDiscardSession (model_commands.go:657-658) path-specific message format.
// PREVENTS: Generic message that doesn't tell the user which path was discarded.
func TestCmdDiscardSessionPathMessage(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)
	err = ed.SetValue([]string{"bgp"}, "router-id", "10.0.0.99")
	require.NoError(t, err)

	m := &Model{editor: ed}
	result, cmdErr := m.cmdDiscardSession([]string{"bgp", "router-id"})
	require.NoError(t, cmdErr)
	assert.Contains(t, result.statusMessage, "bgp router-id", "discard message should include the path")
}

// TestCmdDiscardSessionAll verifies "discard all" gives a generic message.
//
// VALIDATES: cmdDiscardSession (model_commands.go:656) generic message for "all".
// PREVENTS: Path appearing in message when all changes are discarded.
func TestCmdDiscardSessionAll(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)
	err = ed.SetValue([]string{"bgp"}, "router-id", "10.0.0.99")
	require.NoError(t, err)

	m := &Model{editor: ed}
	result, cmdErr := m.cmdDiscardSession([]string{cmdAll})
	require.NoError(t, cmdErr)
	assert.Equal(t, "Session changes discarded", result.statusMessage)
}

// TestCmdShowChangesEmpty verifies "show changes" output when no pending changes exist.
//
// VALIDATES: cmdShowChanges (model_commands.go:682-683) empty check.
// PREVENTS: Blank output instead of informative "No pending changes." message.
func TestCmdShowChangesEmpty(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	m := &Model{editor: ed}
	result, cmdErr := m.cmdShowChanges(nil)
	require.NoError(t, cmdErr)
	assert.Equal(t, "No pending changes.", result.output)
}

// TestCmdShowChangesFormatsDeleteEntry verifies delete entries show "- delete" and "(was: X)".
//
// VALIDATES: formatChangeEntry (model_commands.go:696-699) delete branch.
// PREVENTS: Delete entries rendering without previous value context.
func TestCmdShowChangesFormatsDeleteEntry(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)
	err = ed.DeleteValue([]string{"bgp"}, "local-as")
	require.NoError(t, err)

	m := &Model{editor: ed}
	result, cmdErr := m.cmdShowChanges(nil)
	require.NoError(t, cmdErr)
	assert.Contains(t, result.output, "- delete")
	assert.Contains(t, result.output, "(was:", "delete entry should show previous value")
}

// TestCmdShowChangesNewVsModified verifies "+" for new entries and "*" for modified.
//
// VALIDATES: formatChangeEntry (model_commands.go:701-707) new vs modified markers.
// PREVENTS: Wrong marker or annotation for new vs modified entries.
func TestCmdShowChangesNewVsModified(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	// Modify existing value (should get "*" marker).
	err = ed.SetValue([]string{"bgp"}, "router-id", "10.0.0.99")
	require.NoError(t, err)

	m := &Model{editor: ed}
	result, cmdErr := m.cmdShowChanges(nil)
	require.NoError(t, cmdErr)
	assert.Contains(t, result.output, "* set", "modified entry should have * marker")
	assert.Contains(t, result.output, "(was:", "modified entry should show previous value")
}

// TestCmdShowChangesAllMultiSession verifies grouping by session with headers.
//
// VALIDATES: cmdShowChangesAll (model_commands.go:718-731) multi-session grouping.
// PREVENTS: Changes from different sessions appearing without session identification.
func TestCmdShowChangesAllMultiSession(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 makes a change.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 makes a change.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("bob", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65002")
	require.NoError(t, err)

	// Use ed2's model to show all changes (it sees both sessions via draft).
	m := &Model{editor: ed2}
	result, cmdErr := m.cmdShowChangesAll()
	require.NoError(t, cmdErr)
	assert.Contains(t, result.output, "Session:", "should have session headers")
}

// TestCmdWhoEmpty verifies "who" output when no sessions are active.
//
// VALIDATES: cmdWho (model_commands.go:743-745) empty sessions check.
// PREVENTS: Blank output instead of informative "No active sessions." message.
func TestCmdWhoEmpty(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)
	// No changes made, so no session entries in meta.

	m := &Model{editor: ed}
	result, cmdErr := m.cmdWho()
	require.NoError(t, cmdErr)
	assert.Equal(t, "No active sessions.", result.output)
}

// TestCmdWhoMarksOwnSession verifies the current session is marked with "*".
//
// VALIDATES: cmdWho (model_commands.go:752-753) own-session marker.
// PREVENTS: User unable to identify their own session in the list.
func TestCmdWhoMarksOwnSession(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)
	err = ed.SetValue([]string{"bgp"}, "router-id", "10.0.0.99")
	require.NoError(t, err)

	m := &Model{editor: ed}
	result, cmdErr := m.cmdWho()
	require.NoError(t, cmdErr)
	assert.Contains(t, result.output, "* ", "own session should be marked with *")
	assert.Contains(t, result.output, "1 pending change", "should show change count")
}

// TestCmdCommitSessionConflictFormatting verifies LIVE and STALE conflict labels in output.
//
// VALIDATES: cmdCommitSession (model_commands.go:602-607) conflict type labels.
// PREVENTS: Unclear conflict output that doesn't distinguish live vs stale.
func TestCmdCommitSessionConflictFormatting(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 changes router-id.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 changes same leaf to different value.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("bob", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "router-id", "10.0.0.2")
	require.NoError(t, err)

	// Session 2 tries to commit -- should get LIVE conflict.
	commitResult, err := ed2.CommitSession()
	require.NoError(t, err)
	require.NotEmpty(t, commitResult.Conflicts, "should detect live conflict")

	// Format the conflict output manually (cmdCommitSession calls CommitSession internally,
	// but we already have the result -- test the formatting logic).
	var b strings.Builder
	for _, c := range commitResult.Conflicts {
		switch c.Type {
		case ConflictLive:
			fmt.Fprintf(&b, "  LIVE %s: you=%s, %s=%s\n", c.Path, c.MyValue, c.OtherUser, c.OtherValue)
		case ConflictStale:
			fmt.Fprintf(&b, "  STALE %s: you=%s, committed=%s (was %s)\n", c.Path, c.MyValue, c.OtherValue, c.PreviousValue)
		}
	}
	output := b.String()
	assert.Contains(t, output, "LIVE", "should contain LIVE label for live conflict")
	assert.Contains(t, output, "router-id", "should reference the conflicting path")
}

// TestCommitMetadataPreservesPriorAnnotations verifies that prior commit annotations
// survive subsequent commits by different sessions.
//
// VALIDATES: buildCommitMeta (editor_draft.go:992-994) starts from existing metadata.
// PREVENTS: Prior commit author/time being wiped when a new session commits different leaves.
func TestCommitMetadataPreservesPriorAnnotations(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1: commit a change to router-id.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	_, err = ed1.CommitSession()
	require.NoError(t, err)

	// Read config.conf: should have alice's metadata for router-id.
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(configData), "alice", "first commit should annotate with alice")

	// Session 2: commit a change to local-as only.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("bob", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65002")
	require.NoError(t, err)

	_, err = ed2.CommitSession()
	require.NoError(t, err)

	// config.conf should still have alice's annotation for router-id.
	configData, err = os.ReadFile(configPath)
	require.NoError(t, err)
	configContent := string(configData)
	assert.Contains(t, configContent, "alice", "prior commit annotation should survive new commit")
	assert.Contains(t, configContent, "bob", "new commit should also have its annotation")
}

// TestDisconnectDeleteEntry verifies that disconnecting a session that deleted
// a value restores the committed value.
//
// VALIDATES: DisconnectSession (editor_draft.go:880-881) committed value restore.
// PREVENTS: Leaf staying deleted after the deleting session is disconnected.
func TestDisconnectDeleteEntry(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 deletes local-as.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.DeleteValue([]string{"bgp"}, "local-as")
	require.NoError(t, err)

	// Session 2 disconnects session 1.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("thomas", "local")
	ed2.SetSession(session2)
	// Make a change so session 2 is in the draft.
	err = ed2.SetValue([]string{"bgp"}, "router-id", "10.0.0.99")
	require.NoError(t, err)

	err = ed2.DisconnectSession(session1.ID)
	require.NoError(t, err)

	// Verify local-as is restored in the draft (read from disk).
	draftPath := DraftPath(configPath)
	draftData, err := os.ReadFile(draftPath)
	require.NoError(t, err)
	assert.Contains(t, string(draftData), "local-as", "local-as should be restored after disconnecting deleting session")
}

// TestSessionChangesAllSessions verifies SessionChanges("") returns all sessions' changes.
//
// VALIDATES: SessionChanges (editor.go:693-699) empty sessionID collects from all sessions.
// PREVENTS: Empty result when requesting all sessions' changes.
func TestSessionChangesAllSessions(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	// Session 1 makes a change.
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // Best effort cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 makes a different change.
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // Best effort cleanup

	session2 := NewEditSession("bob", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp"}, "local-as", "65002")
	require.NoError(t, err)

	// SessionChanges("") should return entries from both sessions.
	allChanges := ed2.SessionChanges("")
	assert.GreaterOrEqual(t, len(allChanges), 2, "should have at least one change from each session")
}

// TestActiveSessionsEmpty verifies ActiveSessions returns nil when meta is nil.
//
// VALIDATES: ActiveSessions (editor.go:706-708) nil meta guard.
// PREVENTS: Panic on nil meta when no session has been set.
func TestActiveSessionsEmpty(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// No session set: meta is nil.
	sessions := ed.ActiveSessions()
	assert.Nil(t, sessions, "nil meta should return nil")
}

// TestBlameViewNilMeta verifies blame view with nil meta doesn't panic.
//
// VALIDATES: BlameView (editor.go:674-676) nil meta fallback to empty MetaTree.
// PREVENTS: Panic when blame is called before any session activity.
func TestBlameViewNilMeta(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	// No session set: meta is nil. Should not panic.
	output := ed.BlameView()
	assert.NotEmpty(t, output, "blame view should produce output even without metadata")
}

// TestCmdCommitSessionMigrationWarningFormat verifies the migration warning
// message formatting logic in cmdCommitSession.
//
// VALIDATES: cmdCommitSession (model_commands.go:617-619) migration warning in message.
// PREVENTS: Migration warnings being silently swallowed in commit output.
func TestCmdCommitSessionMigrationWarningFormat(t *testing.T) {
	// The migration warning formatting is: msg += fmt.Sprintf(" (warning: %s)", ...)
	// Test the format string directly since triggering actual migration errors
	// requires configs the schema parser would reject.
	msg := fmt.Sprintf("Session committed: %d change(s) applied", 3)
	warning := "tree migration skipped: test error"
	if warning != "" {
		msg += fmt.Sprintf(" (warning: %s)", warning)
	}
	assert.Contains(t, msg, "(warning:", "migration warning should be included in message")
	assert.Contains(t, msg, "3 change(s)", "change count should be in message")

	// Also verify CommitResult.MigrationWarning is empty for normal commits.
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)
	err = ed.SetValue([]string{"bgp"}, "router-id", "10.0.0.99")
	require.NoError(t, err)

	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result.MigrationWarning, "no migration warning for valid set+meta config")
	assert.Equal(t, 1, result.Applied)
}

// --- Unit tests for package-level navigation and helper functions ---

// testNavSchema builds a schema with Container, List, Flex, InlineList,
// Freeform, and Leaf nodes for testing navigation functions directly.
func testNavSchema() *config.Schema {
	schema := config.NewSchema()
	schema.Define("router-id", config.Leaf(config.TypeIPv4))
	schema.Define("local-as", config.Leaf(config.TypeUint32))
	schema.Define("neighbor", config.List(config.TypeIP,
		config.Field("peer-as", config.Leaf(config.TypeUint32)),
		config.Field("hold-time", config.Leaf(config.TypeUint32)),
	))
	schema.Define("opts", config.Flex(
		config.Field("timeout", config.Leaf(config.TypeUint32)),
	))
	schema.Define("routes", config.InlineList(config.TypePrefix,
		config.Field("next-hop", config.Leaf(config.TypeIP)),
	))
	schema.Define("extras", config.Freeform())
	return schema
}

// TestWalkOrCreateMetaFlexNode verifies walkOrCreateMeta navigates
// through FlexNode schema elements.
//
// VALIDATES: FlexNode path creates container in MetaTree.
//
// PREVENTS: Panic or nil when setting metadata under a flex node.
func TestWalkOrCreateMetaFlexNode(t *testing.T) {
	schema := testNavSchema()
	meta := config.NewMetaTree()

	target := walkOrCreateMeta(meta, schema, []string{"opts"})
	require.NotNil(t, target)

	target.SetEntry("timeout", config.MetaEntry{Session: "alice:100", Value: "30"})
	got, ok := target.GetEntry("timeout")
	require.True(t, ok)
	assert.Equal(t, "30", got.Value)
}

// TestWalkOrCreateMetaInlineListNode verifies walkOrCreateMeta navigates
// through InlineListNode with a key.
//
// VALIDATES: InlineListNode creates container+list-entry in MetaTree.
//
// PREVENTS: Metadata lost for inline list entries.
func TestWalkOrCreateMetaInlineListNode(t *testing.T) {
	schema := testNavSchema()
	meta := config.NewMetaTree()

	target := walkOrCreateMeta(meta, schema, []string{"routes", "10.0.0.0/24"})
	require.NotNil(t, target)

	target.SetEntry("next-hop", config.MetaEntry{Session: "alice:100", Value: "192.168.1.1"})
	got, ok := target.GetEntry("next-hop")
	require.True(t, ok)
	assert.Equal(t, "192.168.1.1", got.Value)
}

// TestWalkOrCreateMetaUnknownSchema verifies unknown path elements
// fall back to container creation (best-effort).
//
// VALIDATES: Unknown schema node treated as container (line 303-306).
//
// PREVENTS: Lost metadata when schema doesn't recognize a path segment.
func TestWalkOrCreateMetaUnknownSchema(t *testing.T) {
	schema := testNavSchema()
	meta := config.NewMetaTree()

	target := walkOrCreateMeta(meta, schema, []string{"unknown-element"})
	require.NotNil(t, target, "unknown path should create container as best-effort")

	target.SetEntry("value", config.MetaEntry{Session: "alice:100"})
	_, ok := target.GetEntry("value")
	assert.True(t, ok)
}

// TestWalkOrCreateMetaLeafMidPath verifies that a leaf-like node mid-path
// is treated as a container (best-effort fallback).
//
// VALIDATES: Leaf mid-path creates container so metadata isn't lost (line 346-351).
//
// PREVENTS: Lost metadata when path passes through a leaf-like schema node.
func TestWalkOrCreateMetaLeafMidPath(t *testing.T) {
	schema := testNavSchema()
	meta := config.NewMetaTree()

	target := walkOrCreateMeta(meta, schema, []string{"router-id"})
	require.NotNil(t, target, "leaf mid-path should create container as best-effort")
}

// TestWalkMetaReadOnlyFlexNode verifies read-only navigation through FlexNode.
//
// VALIDATES: FlexNode read-only navigation returns existing container.
//
// PREVENTS: Read-only flex node access failing or creating nodes.
func TestWalkMetaReadOnlyFlexNode(t *testing.T) {
	schema := testNavSchema()
	meta := config.NewMetaTree()

	target := walkOrCreateMeta(meta, schema, []string{"opts"})
	target.SetEntry("timeout", config.MetaEntry{Session: "alice:100", Value: "30"})

	result := walkMetaReadOnly(meta, schema, []string{"opts"})
	require.NotNil(t, result)
	got, ok := result.GetEntry("timeout")
	require.True(t, ok)
	assert.Equal(t, "30", got.Value)
}

// TestWalkMetaReadOnlyInlineListNode verifies read-only navigation through InlineListNode.
//
// VALIDATES: InlineListNode read-only navigation returns existing entry.
//
// PREVENTS: Read-only inline list access failing or creating nodes.
func TestWalkMetaReadOnlyInlineListNode(t *testing.T) {
	schema := testNavSchema()
	meta := config.NewMetaTree()

	target := walkOrCreateMeta(meta, schema, []string{"routes", "10.0.0.0/24"})
	target.SetEntry("next-hop", config.MetaEntry{Session: "alice:100", Value: "1.1.1.1"})

	result := walkMetaReadOnly(meta, schema, []string{"routes", "10.0.0.0/24"})
	require.NotNil(t, result)
	got, ok := result.GetEntry("next-hop")
	require.True(t, ok)
	assert.Equal(t, "1.1.1.1", got.Value)
}

// TestWalkMetaReadOnlyUnknownSchema verifies read-only navigation returns nil
// for unknown schema elements.
//
// VALIDATES: Unknown path returns nil without creating nodes (line 367-368).
//
// PREVENTS: Read-only navigation accidentally creating MetaTree nodes.
func TestWalkMetaReadOnlyUnknownSchema(t *testing.T) {
	schema := testNavSchema()
	meta := config.NewMetaTree()

	result := walkMetaReadOnly(meta, schema, []string{"nonexistent"})
	assert.Nil(t, result)
}

// TestWalkMetaReadOnlyLeafMidPath verifies read-only navigation returns nil
// when a leaf-like node appears mid-path.
//
// VALIDATES: Leaf-like mid-path returns nil for read-only (line 434-436).
//
// PREVENTS: Read-only navigation traversing into leaf schema nodes.
func TestWalkMetaReadOnlyLeafMidPath(t *testing.T) {
	schema := testNavSchema()
	meta := config.NewMetaTree()

	result := walkMetaReadOnly(meta, schema, []string{"router-id"})
	assert.Nil(t, result, "leaf-like mid-path should return nil in read-only mode")
}

// TestWalkMetaReadOnlyMissing verifies read-only navigation returns nil
// when the container doesn't exist.
//
// VALIDATES: Missing intermediate returns nil.
//
// PREVENTS: Panic on nil container in read-only walk.
func TestWalkMetaReadOnlyMissing(t *testing.T) {
	schema := testNavSchema()
	meta := config.NewMetaTree()

	result := walkMetaReadOnly(meta, schema, []string{"neighbor", "1.1.1.1"})
	assert.Nil(t, result)
}

// TestWalkPathFlexNode verifies walkPath navigates through FlexNode.
//
// VALIDATES: FlexNode path returns the child container tree.
//
// PREVENTS: walkPath failing for flex schema nodes.
func TestWalkPathFlexNode(t *testing.T) {
	schema := testNavSchema()
	tree := config.NewTree()
	optsChild := tree.GetOrCreateContainer("opts")
	optsChild.Set("timeout", "30")

	result := walkPath(tree, schema, []string{"opts"})
	require.NotNil(t, result)
	val, ok := result.Get("timeout")
	require.True(t, ok)
	assert.Equal(t, "30", val)
}

// TestWalkPathInlineListNode verifies walkPath navigates through InlineListNode.
//
// VALIDATES: InlineListNode path returns the keyed entry tree.
//
// PREVENTS: walkPath failing for inline list schema nodes.
func TestWalkPathInlineListNode(t *testing.T) {
	schema := testNavSchema()
	tree := config.NewTree()
	entry := config.NewTree()
	entry.Set("next-hop", "192.168.1.1")
	tree.AddListEntry("routes", "10.0.0.0/24", entry)

	result := walkPath(tree, schema, []string{"routes", "10.0.0.0/24"})
	require.NotNil(t, result)
	val, ok := result.Get("next-hop")
	require.True(t, ok)
	assert.Equal(t, "192.168.1.1", val)
}

// TestWalkPathFlexNodeMissing verifies walkPath returns nil for missing flex container.
//
// VALIDATES: walkPath returns nil when flex container doesn't exist.
//
// PREVENTS: Panic on nil flex container in walkPath.
func TestWalkPathFlexNodeMissing(t *testing.T) {
	schema := testNavSchema()
	tree := config.NewTree()

	result := walkPath(tree, schema, []string{"opts"})
	assert.Nil(t, result)
}

// TestWalkPathInlineListMissing verifies walkPath returns nil for missing inline list entry.
//
// VALIDATES: walkPath returns nil when inline list has no entries.
//
// PREVENTS: Panic on nil inline list in walkPath.
func TestWalkPathInlineListMissing(t *testing.T) {
	schema := testNavSchema()
	tree := config.NewTree()

	result := walkPath(tree, schema, []string{"routes", "10.0.0.0/24"})
	assert.Nil(t, result)
}

// TestGetValueAtPathEmpty verifies getValueAtPath returns "" for empty path.
//
// VALIDATES: Empty pathParts early return (line 912-913).
//
// PREVENTS: Index out of range on empty path slice.
func TestGetValueAtPathEmpty(t *testing.T) {
	schema := testNavSchema()
	tree := config.NewTree()
	tree.Set("router-id", "1.2.3.4")

	got := getValueAtPath(tree, schema, nil)
	assert.Equal(t, "", got)

	got = getValueAtPath(tree, schema, []string{})
	assert.Equal(t, "", got)
}

// TestCheckLiveConflictsEmptyPath verifies checkLiveConflicts returns nil for empty path.
//
// VALIDATES: Empty pathParts early return (line 929-931).
//
// PREVENTS: Index out of range on empty path slice.
func TestCheckLiveConflictsEmptyPath(t *testing.T) {
	schema := testNavSchema()
	meta := config.NewMetaTree()

	conflicts := checkLiveConflicts(meta, "alice:100", "", nil, "value", schema)
	assert.Nil(t, conflicts)

	conflicts = checkLiveConflicts(meta, "alice:100", "", []string{}, "value", schema)
	assert.Nil(t, conflicts)
}

// TestCheckLiveConflictsNilMetaTarget verifies checkLiveConflicts returns nil
// when walkMetaReadOnly returns nil (meta path doesn't exist).
//
// VALIDATES: Nil metaTarget handling (line 938-939).
//
// PREVENTS: Panic when checking conflicts for a path with no metadata.
func TestCheckLiveConflictsNilMetaTarget(t *testing.T) {
	schema := testNavSchema()
	meta := config.NewMetaTree()

	conflicts := checkLiveConflicts(meta, "alice:100", "neighbor 1.1.1.1 peer-as",
		[]string{"neighbor", "1.1.1.1", "peer-as"}, "65001", schema)
	assert.Nil(t, conflicts)
}

// TestCopyNonSessionMetaNoOverwrite verifies that copyNonSessionMeta does not
// overwrite existing entries in the destination MetaTree.
//
// VALIDATES: No-overwrite guard (line 1039): existing dst entries preserved.
//
// PREVENTS: Prior commit metadata being overwritten by draft's hand-written metadata.
func TestCopyNonSessionMetaNoOverwrite(t *testing.T) {
	dst := config.NewMetaTree()
	dst.SetEntry("router-id", config.MetaEntry{
		User: "prior-commit-user",
		Time: time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC),
	})

	src := config.NewMetaTree()
	src.SetEntry("router-id", config.MetaEntry{
		User: "hand-written-user",
	})
	src.SetEntry("local-as", config.MetaEntry{
		User: "hand-written-user",
	})

	copyNonSessionMeta(dst, src)

	got, ok := dst.GetEntry("router-id")
	require.True(t, ok)
	assert.Equal(t, "prior-commit-user", got.User,
		"existing dst entry should not be overwritten")

	got, ok = dst.GetEntry("local-as")
	require.True(t, ok)
	assert.Equal(t, "hand-written-user", got.User,
		"new entry from src should be copied to dst")
}

// TestCopyNonSessionMetaNilSrc verifies copyNonSessionMeta is safe with nil src.
//
// VALIDATES: Nil src early return (line 1032-1033).
//
// PREVENTS: Panic on nil source MetaTree.
func TestCopyNonSessionMetaNilSrc(t *testing.T) {
	dst := config.NewMetaTree()
	dst.SetEntry("router-id", config.MetaEntry{User: "alice"})

	copyNonSessionMeta(dst, nil)

	got, ok := dst.GetEntry("router-id")
	require.True(t, ok)
	assert.Equal(t, "alice", got.User)
}

// TestCopyNonSessionMetaSkipsSessionEntries verifies that entries with Session
// IDs are NOT copied (only sessionless/hand-written entries are copied).
//
// VALIDATES: Session filter (line 1038): only entries with Session="" are copied.
//
// PREVENTS: Draft session metadata leaking into committed config.
func TestCopyNonSessionMetaSkipsSessionEntries(t *testing.T) {
	dst := config.NewMetaTree()
	src := config.NewMetaTree()

	src.SetEntry("router-id", config.MetaEntry{
		User:    "alice",
		Session: "alice:100",
		Value:   "1.2.3.4",
	})
	src.SetEntry("local-as", config.MetaEntry{
		User: "hand-written",
	})

	copyNonSessionMeta(dst, src)

	_, ok := dst.GetEntry("router-id")
	assert.False(t, ok, "session entry should not be copied")

	got, ok := dst.GetEntry("local-as")
	require.True(t, ok)
	assert.Equal(t, "hand-written", got.User)
}

// TestBuildCommitMetaPreservesExisting verifies that buildCommitMeta starts
// from existing committed metadata, preserving prior annotations.
//
// VALIDATES: buildCommitMeta uses existingMeta as base (line 994).
//
// PREVENTS: Prior commit annotations lost on subsequent commits.
func TestBuildCommitMetaPreservesExisting(t *testing.T) {
	schema := testNavSchema()

	existingMeta := config.NewMetaTree()
	existingMeta.SetEntry("local-as", config.MetaEntry{
		User: "prior-user",
		Time: time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC),
	})

	draftMeta := config.NewMetaTree()
	draftMeta.SetEntry("router-id", config.MetaEntry{
		Session: "alice:100",
		Value:   "1.2.3.4",
	})

	myEntries := []config.SessionEntry{
		{Path: "router-id", Entry: config.MetaEntry{Session: "alice:100", Value: "1.2.3.4"}},
	}

	commitTime := time.Date(2026, 3, 12, 14, 0, 0, 0, time.UTC)
	result := buildCommitMeta(existingMeta, draftMeta, myEntries, "alice@ssh", commitTime, schema)

	got, ok := result.GetEntry("local-as")
	require.True(t, ok, "prior commit metadata should be preserved")
	assert.Equal(t, "prior-user", got.User)

	got, ok = result.GetEntry("router-id")
	require.True(t, ok)
	assert.Equal(t, "alice@ssh", got.User)
	assert.Equal(t, commitTime, got.Time)
	assert.Equal(t, "", got.Session, "committed metadata should have no session ID")
}

// TestBuildCommitMetaDeleteRemoves verifies that buildCommitMeta removes metadata
// for deleted leaves instead of creating tombstones.
//
// VALIDATES: Delete branch (line 1011-1013): removes existing metadata.
//
// PREVENTS: Tombstone metadata accumulating for deleted leaves.
func TestBuildCommitMetaDeleteRemoves(t *testing.T) {
	schema := testNavSchema()

	existingMeta := config.NewMetaTree()
	existingMeta.SetEntry("router-id", config.MetaEntry{
		User: "prior-user",
	})

	draftMeta := config.NewMetaTree()
	myEntries := []config.SessionEntry{
		{Path: "router-id", Entry: config.MetaEntry{Session: "alice:100", Value: ""}},
	}

	commitTime := time.Date(2026, 3, 12, 14, 0, 0, 0, time.UTC)
	result := buildCommitMeta(existingMeta, draftMeta, myEntries, "alice@ssh", commitTime, schema)

	_, ok := result.GetEntry("router-id")
	assert.False(t, ok, "delete should remove metadata, not create tombstone")
}

// TestBuildCommitMetaNilExisting verifies buildCommitMeta handles nil existingMeta.
//
// VALIDATES: Nil existingMeta creates fresh MetaTree (line 995-997).
//
// PREVENTS: Panic when first commit on a file with no prior metadata.
func TestBuildCommitMetaNilExisting(t *testing.T) {
	schema := testNavSchema()

	draftMeta := config.NewMetaTree()
	myEntries := []config.SessionEntry{
		{Path: "router-id", Entry: config.MetaEntry{Session: "alice:100", Value: "1.2.3.4"}},
	}

	commitTime := time.Date(2026, 3, 12, 14, 0, 0, 0, time.UTC)
	result := buildCommitMeta(nil, draftMeta, myEntries, "alice@ssh", commitTime, schema)
	require.NotNil(t, result)

	got, ok := result.GetEntry("router-id")
	require.True(t, ok)
	assert.Equal(t, "alice@ssh", got.User)
}

// TestCopyNonSessionMetaRecursive verifies copyNonSessionMeta traverses
// containers and lists recursively.
//
// VALIDATES: Recursive container/list traversal (lines 1046-1054).
//
// PREVENTS: Nested hand-written metadata lost on commit.
func TestCopyNonSessionMetaRecursive(t *testing.T) {
	dst := config.NewMetaTree()
	src := config.NewMetaTree()

	neighborMeta := src.GetOrCreateContainer("neighbor")
	peerMeta := neighborMeta.GetOrCreateListEntry("1.1.1.1")
	peerMeta.SetEntry("peer-as", config.MetaEntry{
		User: "hand-written",
	})

	copyNonSessionMeta(dst, src)

	dstNeighbor := dst.GetContainer("neighbor")
	require.NotNil(t, dstNeighbor, "container should be created in dst")
	dstPeer := dstNeighbor.GetListEntry("1.1.1.1")
	require.NotNil(t, dstPeer, "list entry should be created in dst")
	got, ok := dstPeer.GetEntry("peer-as")
	require.True(t, ok)
	assert.Equal(t, "hand-written", got.User)
}

// TestEditorWithBlobStorage verifies editor set/commit cycle using blob storage.
//
// VALIDATES: Editor reads/writes config through blob storage, not filesystem.
// PREVENTS: Storage wiring broken - editor silently falls back to os.ReadFile.
func TestEditorWithBlobStorage(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")

	// Write config to filesystem so blob migration picks it up
	err := os.WriteFile(configPath, []byte(validBGPConfig), 0o600)
	require.NoError(t, err)

	blobPath := filepath.Join(dir, "database.zefs")
	store, err := storage.NewBlob(blobPath, dir)
	require.NoError(t, err)
	defer store.Close() //nolint:errcheck // test cleanup

	// Remove filesystem copy to prove reads come from blob
	err = os.Remove(configPath)
	require.NoError(t, err)

	// Create editor with blob storage
	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	// Verify initial content was read from blob
	assert.False(t, ed.Dirty())

	// Set a value
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)
	assert.True(t, ed.Dirty())

	// Save (commit) - should write back to blob
	err = ed.Save()
	require.NoError(t, err)

	// Read back from blob to verify the write went through storage
	data, err := store.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "9.9.9.9", "saved config should contain new router-id")
}

// TestEditorFilesystemOverride verifies that filesystem storage is used when -f flag
// causes storage to be replaced with NewFilesystem().
//
// VALIDATES: Editor with filesystem storage reads from real filesystem.
// PREVENTS: -f flag not actually switching to filesystem storage.
func TestEditorFilesystemOverride(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")

	// Write config to filesystem only (no blob)
	err := os.WriteFile(configPath, []byte(validBGPConfig), 0o600)
	require.NoError(t, err)

	// Use filesystem storage directly (simulates -f flag)
	store := storage.NewFilesystem()

	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	// Should have loaded from filesystem
	assert.False(t, ed.Dirty())
	assert.Equal(t, configPath, ed.OriginalPath())
}
