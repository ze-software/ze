// Design: docs/architecture/config/syntax.md — config set tests

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// writeTestConfig creates a temp config file and returns its path.
func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

// writeBlobConfig creates a blob store, writes a config into it, and returns
// the store and the config key (absolute path) used inside the blob.
func writeBlobConfig(t *testing.T, content string) (storage.Storage, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")
	// Write to filesystem first so NewBlob migrates it in
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	blobPath := filepath.Join(dir, "database.zefs")
	store, err := storage.NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("create blob: %v", err)
	}
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	return store, configPath
}

// TestCmdSetBasic verifies setting a simple leaf value.
//
// VALIDATES: ze config set modifies config file correctly.
// PREVENTS: Set command silently failing or corrupting config.
func TestCmdSetBasic(t *testing.T) {
	configPath := writeTestConfig(t, `bgp {
	peer 127.0.0.1 {
		local-as 1;
		peer-as 2;
	}
}
`)

	code := cmdSet([]string{"--no-reload", configPath, "bgp", "peer", "127.0.0.1", "local-as", "65000"})
	if code != exitOK {
		t.Fatalf("cmdSet returned %d, want %d", code, exitOK)
	}

	// Read back and verify
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(data), "65000") {
		t.Errorf("config should contain '65000', got:\n%s", string(data))
	}
}

// TestCmdSetCreatesBackup verifies that set creates a backup file.
//
// VALIDATES: Backup is created before modifying config.
// PREVENTS: Data loss from unintended modifications.
func TestCmdSetCreatesBackup(t *testing.T) {
	configPath := writeTestConfig(t, `bgp {
	peer 127.0.0.1 {
		local-as 1;
		peer-as 2;
	}
}
`)

	code := cmdSet([]string{"--no-reload", configPath, "bgp", "peer", "127.0.0.1", "local-as", "65000"})
	if code != exitOK {
		t.Fatalf("cmdSet returned %d, want %d", code, exitOK)
	}

	// Check for backup file in rollback/ subdirectory
	rollbackDir := filepath.Join(filepath.Dir(configPath), "rollback")
	entries, err := os.ReadDir(rollbackDir)
	if err != nil {
		t.Fatalf("readdir rollback/: %v", err)
	}

	backupFound := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "test-") && strings.HasSuffix(e.Name(), ".conf") {
			backupFound = true
			break
		}
	}
	if !backupFound {
		t.Error("expected backup file to be created in rollback/")
	}
}

// TestCmdSetDryRun verifies dry-run mode does not modify the file.
//
// VALIDATES: --dry-run shows changes without writing.
// PREVENTS: Accidental writes during preview.
func TestCmdSetDryRun(t *testing.T) {
	content := `bgp {
	peer 127.0.0.1 {
		local-as 1;
		peer-as 2;
	}
}
`
	configPath := writeTestConfig(t, content)

	code := cmdSet([]string{"--dry-run", configPath, "bgp", "peer", "127.0.0.1", "local-as", "65000"})
	if code != exitOK {
		t.Fatalf("cmdSet --dry-run returned %d, want %d", code, exitOK)
	}

	// File should be unchanged
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strings.Contains(string(data), "65000") {
		t.Error("dry-run should not modify the file")
	}
}

// TestCmdSetMissingFile verifies error on nonexistent config file.
//
// VALIDATES: Missing file returns error exit code.
// PREVENTS: Panic on nonexistent file.
func TestCmdSetMissingFile(t *testing.T) {
	code := cmdSet([]string{"/nonexistent/config.conf", "bgp", "local-as", "65000"})
	if code != exitError {
		t.Errorf("cmdSet on missing file returned %d, want %d", code, exitError)
	}
}

// TestCmdSetTooFewArgs verifies error on insufficient arguments.
//
// VALIDATES: Usage shown when args are missing.
// PREVENTS: Panic on empty args.
func TestCmdSetTooFewArgs(t *testing.T) {
	code := cmdSet([]string{})
	if code != exitError {
		t.Errorf("cmdSet with no args returned %d, want %d", code, exitError)
	}

	configPath := writeTestConfig(t, "bgp {}\n")
	code = cmdSet([]string{configPath, "bgp"})
	if code != exitError {
		t.Errorf("cmdSet with only file+key returned %d, want %d", code, exitError)
	}
}

// TestCmdSetDispatch verifies set is reachable from the Run dispatcher.
//
// VALIDATES: "set" subcommand is registered in dispatch map.
// PREVENTS: Wiring failure where set command is unreachable.
func TestCmdSetDispatch(t *testing.T) {
	// Verify the handler is registered in storage-aware handlers
	handler, ok := storageHandlers["set"]
	if !ok {
		t.Fatal("'set' not registered in storageHandlers")
	}
	if handler == nil {
		t.Fatal("'set' handler is nil")
	}
}

// TestCmdSetWithBlobStorage verifies set works through blob storage backend.
//
// VALIDATES: cmdSetWithStorage writes through blob, not filesystem.
// PREVENTS: Storage wiring broken — set silently falls back to filesystem.
func TestCmdSetWithBlobStorage(t *testing.T) {
	store, configPath := writeBlobConfig(t, `bgp {
	peer 127.0.0.1 {
		local-as 1;
		peer-as 2;
	}
}
`)

	code := cmdSetWithStorage(store, []string{"--no-reload", configPath, "bgp", "peer", "127.0.0.1", "local-as", "65000"})
	if code != exitOK {
		t.Fatalf("cmdSetWithStorage returned %d, want %d", code, exitOK)
	}

	// Read back from blob storage (not filesystem) and verify
	data, err := store.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read from blob: %v", err)
	}
	if !strings.Contains(string(data), "65000") {
		t.Errorf("blob config should contain '65000', got:\n%s", string(data))
	}
}

// TestRunWithStorageDispatches verifies RunWithStorage routes to storage-aware handlers.
//
// VALIDATES: RunWithStorage passes storage to subcommand handlers.
// PREVENTS: Storage lost during dispatch — subcommands use filesystem instead.
func TestRunWithStorageDispatches(t *testing.T) {
	store, configPath := writeBlobConfig(t, `bgp {
	peer 127.0.0.1 {
		local-as 1;
		peer-as 2;
	}
}
`)

	code := RunWithStorage(store, []string{"set", "--no-reload", configPath, "bgp", "peer", "127.0.0.1", "local-as", "65000"})
	if code != exitOK {
		t.Fatalf("RunWithStorage set returned %d, want %d", code, exitOK)
	}

	// Verify change persisted in blob
	data, err := store.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read from blob: %v", err)
	}
	if !strings.Contains(string(data), "65000") {
		t.Errorf("blob config should contain '65000' after RunWithStorage dispatch")
	}
}
