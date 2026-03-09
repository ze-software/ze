package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

func TestNewEditor(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write initial config
	initial := `router-id 1.2.3.4;
local-as 65000;
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
	initial := `router-id 1.2.3.4;` //nolint:goconst // test value
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

	initial := `router-id 1.2.3.4;`
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
	version1 := `router-id 1.1.1.1;`
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
	version2 := `router-id 2.2.2.2;`
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

	initial := `router-id 1.2.3.4;
local-as 65000;
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
	ed.SetWorkingContent(`router-id 1.2.3.4;
local-as 65001;
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
	initial := `router-id 1.2.3.4;`
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	// Create editor
	ed, err := NewEditor(configPath)
	require.NoError(t, err)

	// No edit file yet
	_, err = os.Stat(editPath)
	assert.True(t, os.IsNotExist(err), "edit file should not exist initially")

	// Make a change and save edit state
	ed.SetWorkingContent(`router-id 2.2.2.2;`)
	ed.MarkDirty()
	err = ed.SaveEditState()
	require.NoError(t, err)

	// Edit file should now exist
	_, err = os.Stat(editPath)
	assert.NoError(t, err, "edit file should exist after change")

	// Verify edit file content
	editContent, err := os.ReadFile(editPath) //nolint:gosec // test path
	require.NoError(t, err)
	assert.Equal(t, `router-id 2.2.2.2;`, string(editContent))

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
	original := `router-id 1.1.1.1;`
	err := os.WriteFile(configPath, []byte(original), 0o600)
	require.NoError(t, err)

	// Write existing edit file (simulating previous session)
	editContent := `router-id 9.9.9.9;`
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
	initial := `router-id 1.2.3.4;`
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	// Create editor and make changes
	ed, err := NewEditor(configPath)
	require.NoError(t, err)

	ed.SetWorkingContent(`router-id 2.2.2.2;`)
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
	initial := `router-id 1.2.3.4;`
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	// Create editor and make changes
	ed, err := NewEditor(configPath)
	require.NoError(t, err)

	ed.SetWorkingContent(`router-id 2.2.2.2;`)
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
	initial := `router-id 1.2.3.4;`
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	// Create editor - no edit file yet
	ed, err := NewEditor(configPath)
	require.NoError(t, err)

	// No pending edit, time should be zero
	assert.True(t, ed.PendingEditTime().IsZero(), "no edit file should return zero time")

	// Create edit file
	editContent := `router-id 2.2.2.2;`
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
	initial := `router-id 1.2.3.4;
local-as 65000;`
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	// Write edit file with changes
	editContent := `router-id 2.2.2.2;
local-as 65000;
peer-as 65001;`
	err = os.WriteFile(editPath, []byte(editContent), 0o600)
	require.NoError(t, err)

	// Create editor
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	// Get diff
	diff := ed.PendingEditDiff()
	assert.Contains(t, diff, "- router-id 1.2.3.4;", "should show removed line")
	assert.Contains(t, diff, "+ router-id 2.2.2.2;", "should show added line")
	assert.Contains(t, diff, "+ peer-as 65001;", "should show new line")
}

// TestPendingEditDiffNoEditFile verifies empty diff when no edit file exists.
//
// VALIDATES: PendingEditDiff returns empty when no .edit file.
// PREVENTS: Error when viewing changes without edit file.
func TestPendingEditDiffNoEditFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write config only, no edit file
	content := `router-id 1.2.3.4;`
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
	content := `router-id 1.2.3.4;`
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
	router-id 1.2.3.4;
	local-as 65000;
	peer 1.1.1.1 {
		peer-as 65001;
		hold-time 90;
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
	router-id 5.6.7.8;
	local-as 65001;
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
  router-id 1.2.3.4;
  local-as 65000;
  peer 1.1.1.1 {
    peer-as 65001;
    hold-time 90;
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
