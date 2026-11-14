package cli

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestParseRestoreArgs verifies the restore grammar: <file> then the mode
// keyword, then the optional name keyword with its value; anything else is
// refused naming what was wrong.
// VALIDATES: AC-7, AC-20 (name keyword).
// PREVENTS: a mistyped mode or keyword reaching the store.
func TestParseRestoreArgs(t *testing.T) {
	parsed, err := parseRestoreArgs([]string{"/b.zefs", "config", "name", "a.conf"})
	require.NoError(t, err)
	assert.Equal(t, restoreArgs{file: "/b.zefs", mode: restoreModeConfig, sourceName: "a.conf"}, parsed)
	for args, want := range map[string][]string{
		"missing":          {"/b.zefs"},
		"unknown mode":     {"/b.zefs", "partial"},
		"unknown keyword":  {"/b.zefs", "config", "label", "x"},
		"needs a config":   {"/b.zefs", "config", "name"},
		"config mode only": {"/b.zefs", "full", "name", "a.conf"},
	} {
		_, err := parseRestoreArgs(want)
		require.Error(t, err, args)
		assert.Contains(t, err.Error(), args)
	}
}

// TestRestoreConfigRefusedWhileStoppedWindowRecorded proves the offline config
// restore refuses while a confirmed-commit window record stands, which a
// daemon stopped during its window leaves: that daemon's next start reverts
// the window and would undo the restore. The refusal is the window's own and
// the active config is unchanged; once the record is cleared the same restore
// commits the artifact's config, so the refusal is tied to the record.
// VALIDATES: the offline `ze data restore <file> config` keeps the window rule
// the two offline imports keep (confirm.WriteOutsideRecorded).
// PREVENTS: a restore reported committed that the next daemon start reverts.
func TestRestoreConfigRefusedWhileStoppedWindowRecorded(t *testing.T) {
	const deviceName = "ze.conf"
	dir := t.TempDir()
	target, err := storage.Create(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, target.Close()) })
	_, _, err = storage.EnsureActiveVersion(target, filepath.Join(dir, deviceName), []byte("old config"), time.Now().Add(-time.Second))
	require.NoError(t, err)
	recorder := confirm.NewStoreRecorder(target, deviceName)
	pending := confirm.Pending{User: "alice", Deadline: time.Now().Add(time.Minute), Rollback: []byte("older config")}
	require.NoError(t, recorder.Save(pending))
	parsed := restoreArgs{file: dataArtifact(t, deviceName, "restored config"), mode: restoreModeConfig}

	_, _, err = restoreConfig(target, parsed, deviceName)
	require.Error(t, err)
	assert.Equal(t, (&confirm.StoppedWindowError{Owner: "alice", ConfigPath: deviceName}).Error(), err.Error())
	active, err := storage.ReadActiveConfig(target, deviceName)
	require.NoError(t, err)
	assert.Equal(t, "old config", string(active), "a refused restore leaves the active config")

	require.NoError(t, recorder.Clear())
	stamp, source, err := restoreConfig(target, parsed, deviceName)
	require.NoError(t, err)
	assert.NotEmpty(t, stamp)
	assert.Equal(t, deviceName, source)
	active, err = storage.ReadActiveConfig(target, deviceName)
	require.NoError(t, err)
	assert.Equal(t, "restored config", string(active))
}

// TestParseRestoreFull proves `full` parses and that the canonical seed is
// refused before any store is touched (AC-22).
func TestParseRestoreFull(t *testing.T) {
	parsed, err := parseRestoreArgs([]string{"/b.zefs", "full"})
	require.NoError(t, err)
	assert.Equal(t, restoreModeFull, parsed.mode)
	dir := t.TempDir()
	err = refuseCanonicalSeed(filepath.Join(dir, "database.zefs"), dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ze init --from")
	assert.Contains(t, err.Error(), "ze data restore <new-name> full")
	require.NoError(t, refuseCanonicalSeed(filepath.Join(dir, "backup.zefs"), dir))
	_, err = restoreDir(filepath.Join(dir, "other"))
	require.Error(t, err)
	got, err := restoreDir(filepath.Join(dir, "database"))
	require.NoError(t, err)
	assert.Equal(t, dir, got)
}
