// Design: docs/architecture/config/syntax.md — config history tests

package cli

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestCmdHistoryDispatch verifies history is reachable from the Run dispatcher.
//
// VALIDATES: "history" subcommand is registered in dispatch map.
// PREVENTS: Wiring failure where history command is unreachable.
func TestCmdHistoryDispatch(t *testing.T) {
	handler, ok := storageHandlers["history"]
	if !ok {
		t.Fatal("'history' not registered in storageHandlers")
	}
	if handler == nil {
		t.Fatal("'history' handler is nil")
		return
	}
}

// TestCmdHistoryNoArgs verifies error on missing arguments.
//
// VALIDATES: Usage shown when args are missing.
// PREVENTS: Panic on empty args.
func TestCmdHistoryNoArgs(t *testing.T) {
	code := cmdHistory([]string{})
	assert.Equal(t, exitError, code)
}

// TestCmdHistoryMissingFile verifies error on nonexistent config file.
//
// VALIDATES: Missing file returns error exit code.
// PREVENTS: Panic on nonexistent file.
func TestCmdHistoryMissingFile(t *testing.T) {
	code := cmdHistory([]string{"/nonexistent/config.conf"})
	assert.Equal(t, exitError, code)
}

// TestCmdHistoryNoBackups verifies empty output when no rollback revisions exist.
//
// VALIDATES: History command works with no backups.
// PREVENTS: Crash on empty rollback directory.
func TestCmdHistoryNoBackups(t *testing.T) {
	configPath := writeTestConfig(t, "bgp {\n\tpeer peer1 {\n\t\tremote {\n\t\t\tip 127.0.0.1;\n\t\t\tas 2;\n\t\t}\n\t\tlocal {\n\t\t\tas 1;\n\t\t}\n\t}\n}\n")
	store, err := storage.Create(filepath.Dir(configPath))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	code := cmdHistory([]string{configPath})
	assert.Equal(t, exitOK, code)
}

// TestCmdHistoryListsBackups verifies backups are listed when present.
//
// VALIDATES: History lists rollback revisions in the correct directory.
// PREVENTS: Failure to find backups in rollback/ subdirectory.
func TestCmdHistoryListsBackups(t *testing.T) {
	configPath := writeTestConfig(t, "bgp {\n\tpeer peer1 {\n\t\tremote {\n\t\t\tip 127.0.0.1;\n\t\t\tas 2;\n\t\t}\n\t\tlocal {\n\t\t\tas 1;\n\t\t}\n\t}\n}\n")

	store, err := storage.Create(filepath.Dir(configPath))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteVersion(configPath, []byte("old config"), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	code := cmdHistory([]string{configPath})
	assert.Equal(t, exitOK, code)
}
