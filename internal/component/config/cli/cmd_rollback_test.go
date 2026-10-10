// Design: docs/architecture/config/syntax.md — config rollback tests

package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/confirm"
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

	backupData, err := store.ReadVersion(configPath, entries[0].Stamp)
	require.NoError(t, err)
	assert.Equal(t, currentContent, string(backupData), "pre-rollback backup should contain the overwritten config")
}

// TestCmdRollbackRefusedDuringWindow is review round 3 NOTE 6 of
// spec-session-editor-file-mode-parity, under the owner rule of 2026-10-10:
// every writer but the window owner's editor commands is refused while a
// confirmed-commit window is open. `ze config rollback` runs outside the
// daemon, so it reads the window's persisted record.
//
// GOAL: an offline rollback during a window writes nothing and names the
// window's owner.
// METHOD: the store holds a revision and the record of alice's open window;
// cmdRollback runs as `ze config rollback 1 <file>` does.
//
// VALIDATES: exit error, and the config file is unchanged.
// PREVENTS: a rollback that the window's revert later wipes.
func TestCmdRollbackRefusedDuringWindow(t *testing.T) {
	current := "bgp {\n\tpeer peer1 {\n\t\tremote {\n\t\t\tip 127.0.0.1;\n\t\t\tas 2;\n\t\t}\n\t\tlocal {\n\t\t\tas 99;\n\t\t}\n\t}\n}\n"
	configPath := writeTestConfig(t, current)
	store, err := storage.Create(filepath.Dir(configPath))
	require.NoError(t, err)
	require.NoError(t, store.WriteVersion(configPath, []byte("bgp {}\n"), time.Now().Add(-time.Second)))
	record := confirm.NewStoreRecorder(store, configPath)
	require.NoError(t, record.Save(confirm.Pending{User: "alice", Deadline: time.Now().Add(time.Minute), Rollback: []byte(current)}))
	require.NoError(t, store.Close())

	code := cmdRollback([]string{"1", configPath})
	assert.Equal(t, exitError, code, "a rollback during the window is refused")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, current, string(data), "a refused rollback writes nothing")
}

// TestCmdRollbackLeftOpenWindowNamesStart is review round 4 ISSUE 1 of
// spec-session-editor-file-mode-parity. `ze config rollback` opens the store
// as its owner, so a running daemon refuses it as busy before the window's
// record is read: a record it does see was left by a daemon that stopped
// during a window. Nobody can accept or abort that window with the daemon
// down; starting the daemon reverts it (recoverConfirmWindow).
//
// GOAL: the refusal tells the user what is wrong, why the rollback cannot run,
// and the exact command that clears it (ai/rules/user-facing-errors.md).
// METHOD: the store holds a revision and the record of alice's window, with no
// daemon; stderr of cmdRollback is captured.
//
// VALIDATES: the message names alice, says the daemon reverts the commit when
// it starts, and ends with `ze start <file>` on a line of its own; it does not
// tell the user to wait or to have alice run commit accept or abort.
// PREVENTS: a refusal whose only remedies need the stopped daemon.
func TestCmdRollbackLeftOpenWindowNamesStart(t *testing.T) {
	current := "bgp {\n\tpeer peer1 {\n\t\tremote {\n\t\t\tip 127.0.0.1;\n\t\t\tas 2;\n\t\t}\n\t\tlocal {\n\t\t\tas 99;\n\t\t}\n\t}\n}\n"
	configPath := writeTestConfig(t, current)
	store, err := storage.Create(filepath.Dir(configPath))
	require.NoError(t, err)
	require.NoError(t, store.WriteVersion(configPath, []byte("bgp {}\n"), time.Now().Add(-time.Second)))
	record := confirm.NewStoreRecorder(store, configPath)
	require.NoError(t, record.Save(confirm.Pending{User: "alice", Deadline: time.Now().Add(time.Minute), Rollback: []byte(current)}))
	require.NoError(t, store.Close())

	code, stderr := captureStderr(t, func() int { return cmdRollback([]string{"1", configPath}) })
	assert.Equal(t, exitError, code, "a rollback beside a left-open window is refused")
	assert.Contains(t, stderr, "alice", "the refusal names the window's owner")
	assert.Contains(t, stderr, "when the daemon starts", "the refusal says why: the daemon reverts the window at start")
	assert.Contains(t, stderr, "\n  ze start "+configPath+"\n", "the refusal ends with the command that clears it")
	assert.NotContains(t, stderr, "seconds left", "no countdown runs with the daemon stopped")
	assert.NotContains(t, stderr, "commit accept", "nobody can accept the window with the daemon stopped")
}
