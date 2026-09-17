// Design: docs/architecture/config/syntax.md — config rollback tests

package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestCmdRollbackDispatch verifies rollback is reachable from the Run dispatcher.
//
// VALIDATES: "rollback" subcommand is registered in dispatch map.
// PREVENTS: Wiring failure where rollback command is unreachable.
func TestCmdRollbackDispatch(t *testing.T) {
	handler, ok := storageHandlers["rollback"]
	if !ok {
		t.Fatal("'rollback' not registered in storageHandlers")
	}
	if handler == nil {
		t.Fatal("'rollback' handler is nil")
		return
	}
}

// TestCmdRollbackNoArgs verifies error on missing arguments.
//
// VALIDATES: Usage shown when args are missing.
// PREVENTS: Panic on empty args.
func TestCmdRollbackNoArgs(t *testing.T) {
	code := cmdRollback([]string{})
	assert.Equal(t, exitError, code)
}

// TestCmdRollbackInvalidRevision verifies error on non-numeric revision.
//
// VALIDATES: Non-numeric revision returns error.
// PREVENTS: Silent misbehavior on bad input like "abc".
func TestCmdRollbackInvalidRevision(t *testing.T) {
	configPath := writeTestConfig(t, "bgp {}\n")
	code := cmdRollback([]string{"abc", configPath})
	assert.Equal(t, exitError, code)
}

// TestCmdRollbackZero verifies error when revision number is zero.
//
// VALIDATES: Revision 0 is rejected (1-indexed).
// PREVENTS: Off-by-one accessing backups[-1].
func TestCmdRollbackZero(t *testing.T) {
	configPath := writeTestConfig(t, "bgp {}\n")
	store, err := storage.Create(filepath.Dir(configPath))
	require.NoError(t, err)
	require.NoError(t, store.Close())
	code := cmdRollback([]string{"0", configPath})
	assert.Equal(t, exitError, code)
}

// TestCmdRollbackOutOfRange verifies error when revision number exceeds available backups.
//
// VALIDATES: Out-of-range revision returns error.
// PREVENTS: Index-out-of-bounds panic.
func TestCmdRollbackOutOfRange(t *testing.T) {
	configPath := writeTestConfig(t, "bgp {\n\tpeer peer1 {\n\t\tremote {\n\t\t\tip 127.0.0.1;\n\t\t\tas 2;\n\t\t}\n\t\tlocal {\n\t\t\tas 1;\n\t\t}\n\t}\n}\n")
	store, err := storage.Create(filepath.Dir(configPath))
	require.NoError(t, err)
	require.NoError(t, store.Close())
	code := cmdRollback([]string{"99", configPath})
	assert.Equal(t, exitError, code)
}

// TestCmdRollbackRestores verifies that rollback replaces config with backup content.
//
// VALIDATES: Rollback restores from rollback/ subdirectory.
// PREVENTS: Rollback silently succeeding without actually restoring.
func TestCmdRollbackRestores(t *testing.T) {
	originalContent := "bgp {\n\tpeer peer1 {\n\t\tremote {\n\t\t\tip 127.0.0.1;\n\t\t\tas 2;\n\t\t}\n\t\tlocal {\n\t\t\tas 1;\n\t\t}\n\t}\n}\n"
	configPath := writeTestConfig(t, "bgp {\n\tpeer peer1 {\n\t\tremote {\n\t\t\tip 127.0.0.1;\n\t\t\tas 2;\n\t\t}\n\t\tlocal {\n\t\t\tas 99;\n\t\t}\n\t}\n}\n")

	store, err := storage.Create(filepath.Dir(configPath))
	require.NoError(t, err)
	require.NoError(t, store.WriteVersion(configPath, []byte(originalContent), time.Now().Add(-time.Second)))
	require.NoError(t, store.Close())

	currentContent := "bgp {\n\tpeer peer1 {\n\t\tremote {\n\t\t\tip 127.0.0.1;\n\t\t\tas 2;\n\t\t}\n\t\tlocal {\n\t\t\tas 99;\n\t\t}\n\t}\n}\n"

	code := cmdRollback([]string{"1", configPath})
	assert.Equal(t, exitOK, code)

	// Verify config was restored
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(data))

	// Verify rollback backed up the current config before overwriting
	store, err = storage.OpenReadOnly(filepath.Dir(configPath))
	require.NoError(t, err)
	defer store.Close() //nolint:errcheck // Test cleanup.
	entries, err := store.ListVersions(configPath)
	require.NoError(t, err)
	assert.Equal(t, 2, len(entries), "rollback should create a backup of the current config")

	backupData, err := store.ReadFile(entries[0].Path)
	require.NoError(t, err)
	assert.Equal(t, currentContent, string(backupData), "pre-rollback backup should contain the overwritten config")
}
