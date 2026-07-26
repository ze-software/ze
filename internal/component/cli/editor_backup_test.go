package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestCreateBackupInSameMillisecondKeepsBoth proves back-to-back backups both
// survive.
//
// VALIDATES: N successive createBackup calls produce N listable versions.
// PREVENTS:  silent loss of a rollback point. A version's stamp is its storage
//
//	key and FormatVersionStamp has millisecond resolution, so two
//	backups taken inside one millisecond wrote the same key and the
//	second replaced the first -- nothing logged, nothing returned, one
//	fewer version to roll back to. It surfaced as
//	`backup 2 not found (have 1 backups)` in
//	TestCmdShowPipeCompareRollbackStackFormat, which takes two backups
//	in immediate succession.
func TestCreateBackupInSameMillisecondKeepsBoth(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600))

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // test cleanup

	// Pin the clock so every call asks for the SAME stamp. Without this the test
	// only collides when three real calls happen to land in one millisecond,
	// which they usually do not on a fast host -- it passed with the guard
	// removed, gating nothing. Three calls, so the guard is exercised past a
	// single collision.
	fixed := time.Date(2026, 7, 26, 1, 2, 3, 456_000_000, time.Local)
	ed.now = func() time.Time { return fixed }

	const want = 3
	for i := range want {
		require.NoErrorf(t, ed.createBackup(ed.OriginalContent(), nil), "createBackup %d", i)
	}

	backups, err := ed.ListBackups()
	require.NoError(t, err)
	require.Lenf(t, backups, want,
		"got %d backups from %d createBackup calls; a same-millisecond backup overwrote an earlier one", len(backups), want)
}
