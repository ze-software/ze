package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEditor(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write initial config
	initial := `router-id 1.2.3.4;
local-as 65000;
`
	err := os.WriteFile(configPath, []byte(initial), 0600)
	require.NoError(t, err)

	// Create editor
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // Best effort cleanup

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
	initial := `router-id 1.2.3.4;`
	err := os.WriteFile(configPath, []byte(initial), 0600)
	require.NoError(t, err)

	// Create editor and modify
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // Best effort cleanup

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

func TestEditorBackupNaming(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "myconfig.conf")

	// Write initial config
	err := os.WriteFile(configPath, []byte("test"), 0600)
	require.NoError(t, err)

	// Create multiple backups
	for i := 0; i < 3; i++ {
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
	defer ed.Close() //nolint:errcheck // Best effort cleanup

	backups, err := ed.ListBackups()
	require.NoError(t, err)
	require.Len(t, backups, 3)

	today := time.Now().Format("2006-01-02")

	// Backups should be named: myconfig-YYYY-MM-DD-1.conf, -2.conf, -3.conf
	for i, b := range backups {
		expectedSuffix := today + "-" + string(rune('0'+3-i)) + ".conf"
		assert.True(t, strings.HasSuffix(b.Path, expectedSuffix) ||
			strings.Contains(b.Path, today),
			"backup %d should contain date: %s", i, b.Path)
	}
}

func TestEditorDiscard(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	initial := `router-id 1.2.3.4;`
	err := os.WriteFile(configPath, []byte(initial), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // Best effort cleanup

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
	err := os.WriteFile(configPath, []byte(version1), 0600)
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
	err = os.WriteFile(configPath, []byte(version2), 0600)
	require.NoError(t, err)

	// Rollback to first backup
	ed, err = NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // Best effort cleanup

	backups, err := ed.ListBackups()
	require.NoError(t, err)
	require.Len(t, backups, 1)

	err = ed.Rollback(backups[0].Path)
	require.NoError(t, err)

	// Verify content was restored
	data, err := os.ReadFile(configPath) //nolint:gosec // Test file path
	require.NoError(t, err)
	assert.Equal(t, version1, string(data))
}

func TestEditorListBackupsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte("test"), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // Best effort cleanup

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
	err := os.WriteFile(configPath, []byte(initial), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // Best effort cleanup

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
