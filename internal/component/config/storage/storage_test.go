// Design: docs/architecture/zefs-format.md -- config storage tests

package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: filesystem storage read/write/remove round-trip
// PREVENTS: data loss through storage abstraction

func TestFilesystemStorageReadWrite(t *testing.T) {
	dir := t.TempDir()
	s := NewFilesystem()

	path := filepath.Join(dir, "test.conf")
	data := []byte("router-id 1.1.1.1\n")

	if err := s.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := s.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("ReadFile: got %q, want %q", got, data)
	}

	if err := s.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if s.Exists(path) {
		t.Error("file should not exist after Remove")
	}
}

// VALIDATES: filesystem storage creates parent directories
// PREVENTS: write failure when rollback/ subdir doesn't exist

func TestFilesystemStorageCreatesDirs(t *testing.T) {
	dir := t.TempDir()
	s := NewFilesystem()

	path := filepath.Join(dir, "rollback", "backup-001.conf")
	if err := s.WriteFile(path, []byte("backup"), 0o600); err != nil {
		t.Fatalf("WriteFile with nested dir: %v", err)
	}

	got, err := s.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "backup" {
		t.Errorf("got %q, want %q", got, "backup")
	}
}

// VALIDATES: filesystem Exists returns correct values
// PREVENTS: false positives/negatives on existence check

func TestFilesystemStorageExists(t *testing.T) {
	dir := t.TempDir()
	s := NewFilesystem()

	path := filepath.Join(dir, "check.conf")

	if s.Exists(path) {
		t.Error("should not exist before write")
	}

	if err := s.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !s.Exists(path) {
		t.Error("should exist after write")
	}
}

// VALIDATES: filesystem List returns matching files
// PREVENTS: backup listing failure

func TestFilesystemStorageList(t *testing.T) {
	dir := t.TempDir()
	s := NewFilesystem()

	// Create several files
	for _, name := range []string{"a.conf", "b.conf", "c.txt"} {
		if err := s.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	matches, err := s.List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(matches) != 3 {
		t.Errorf("List(dir): got %d matches, want 3", len(matches))
	}
}

// VALIDATES: filesystem WriteFile is atomic (temp + rename)
// PREVENTS: partial writes on crash

func TestFilesystemStorageAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	s := NewFilesystem()

	path := filepath.Join(dir, "atomic.conf")

	// Write initial content
	if err := s.WriteFile(path, []byte("version-1"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Overwrite -- should be atomic
	if err := s.WriteFile(path, []byte("version-2-longer"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := s.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "version-2-longer" {
		t.Errorf("got %q after overwrite", got)
	}

	// Verify no temp files left behind
	matches, _ := filepath.Glob(filepath.Join(dir, ".ze-storage-*"))
	if len(matches) != 0 {
		t.Errorf("temp files left behind: %v", matches)
	}
}

// VALIDATES: filesystem AcquireLock + WriteGuard read/write/remove cycle
// PREVENTS: locked operations failing or lock not released

func TestFilesystemStorageLockCycle(t *testing.T) {
	dir := t.TempDir()
	s := NewFilesystem()

	configPath := filepath.Join(dir, "test.conf")
	if err := s.WriteFile(configPath, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}

	guard, err := s.AcquireLock(configPath)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	// Read within lock
	got, err := guard.ReadFile(configPath)
	if err != nil {
		t.Fatalf("guard.ReadFile: %v", err)
	}
	if string(got) != "original" {
		t.Errorf("guard.ReadFile: got %q", got)
	}

	// Write within lock
	draftPath := configPath + ".draft"
	if err := guard.WriteFile(draftPath, []byte("modified"), 0o600); err != nil {
		t.Fatalf("guard.WriteFile: %v", err)
	}

	// Read back within lock
	got, err = guard.ReadFile(draftPath)
	if err != nil {
		t.Fatalf("guard.ReadFile draft: %v", err)
	}
	if string(got) != "modified" {
		t.Errorf("guard.ReadFile draft: got %q", got)
	}

	// Remove within lock
	if err := guard.Remove(draftPath); err != nil {
		t.Fatalf("guard.Remove: %v", err)
	}

	// Release lock
	if err := guard.Release(); err != nil {
		t.Fatalf("guard.Release: %v", err)
	}

	// Verify lock file was created
	if _, err := os.Stat(configPath + ".lock"); err != nil {
		t.Errorf("lock file should exist: %v", err)
	}
}

// VALIDATES: WriteGuard.Release is idempotent
// PREVENTS: double-release causing errors

func TestFilesystemStorageGuardDoubleRelease(t *testing.T) {
	dir := t.TempDir()
	s := NewFilesystem()

	configPath := filepath.Join(dir, "test.conf")
	guard, err := s.AcquireLock(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := guard.Release(); err != nil {
		t.Fatalf("first Release: %v", err)
	}

	// Second release should be a no-op, not an error
	if err := guard.Release(); err != nil {
		t.Errorf("second Release should be nil, got: %v", err)
	}
}

// VALIDATES: ReadFile on non-existent file returns error
// PREVENTS: silent empty reads

func TestFilesystemStorageReadNonExistent(t *testing.T) {
	s := NewFilesystem()
	_, err := s.ReadFile("/nonexistent/path/file.conf")
	if err == nil {
		t.Error("expected error reading non-existent file")
	}
}

// VALIDATES: Remove on non-existent file returns error
// PREVENTS: silent remove failure

func TestFilesystemStorageRemoveNonExistent(t *testing.T) {
	s := NewFilesystem()
	err := s.Remove("/nonexistent/path/file.conf")
	if err == nil {
		t.Error("expected error removing non-existent file")
	}
}

// VALIDATES: WriteFile with zero perm defaults to 0o600
// PREVENTS: world-readable config files

func TestFilesystemStorageDefaultPerm(t *testing.T) {
	dir := t.TempDir()
	s := NewFilesystem()

	path := filepath.Join(dir, "secure.conf")
	if err := s.WriteFile(path, []byte("secret"), 0); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("default perm: got %o, want 600", perm)
	}
}

// --- Blob storage tests ---

func newBlobStorage(t *testing.T) Storage {
	t.Helper()
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "test.zefs")
	s, err := NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	t.Cleanup(func() {
		if bs, ok := s.(*blobStorage); ok {
			bs.Close() //nolint:errcheck // test cleanup
		}
	})
	return s
}

// VALIDATES: blob storage read/write/remove round-trip
// PREVENTS: data loss through blob storage abstraction

func TestBlobStorageReadWrite(t *testing.T) {
	s := newBlobStorage(t)

	path := "/etc/ze/router.conf"
	data := []byte("router-id 1.1.1.1\n")

	if err := s.WriteFile(path, data, 0); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := s.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("ReadFile: got %q, want %q", got, data)
	}

	if err := s.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if s.Exists(path) {
		t.Error("file should not exist after Remove")
	}
}

// VALIDATES: blob storage supports multiple independent configs
// PREVENTS: config data bleeding between entries

func TestBlobStorageMultiConfig(t *testing.T) {
	s := newBlobStorage(t)

	configs := map[string]string{
		"/etc/ze/site-a.conf": "router-id 1.1.1.1\n",
		"/etc/ze/site-b.conf": "router-id 2.2.2.2\n",
		"/etc/ze/site-c.conf": "router-id 3.3.3.3\n",
	}

	for path, content := range configs {
		if err := s.WriteFile(path, []byte(content), 0); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}

	for path, want := range configs {
		got, err := s.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}
		if string(got) != want {
			t.Errorf("ReadFile(%s): got %q, want %q", path, got, want)
		}
	}
}

// VALIDATES: blob Exists returns correct values
// PREVENTS: false positives/negatives

func TestBlobStorageExists(t *testing.T) {
	s := newBlobStorage(t)
	path := "/etc/ze/check.conf"

	if s.Exists(path) {
		t.Error("should not exist before write")
	}

	if err := s.WriteFile(path, []byte("data"), 0); err != nil {
		t.Fatal(err)
	}

	if !s.Exists(path) {
		t.Error("should exist after write")
	}
}

// VALIDATES: blob List returns keys as absolute paths
// PREVENTS: caller getting bare keys instead of paths

func TestBlobStorageList(t *testing.T) {
	s := newBlobStorage(t)

	for _, path := range []string{"/etc/ze/a.conf", "/etc/ze/b.conf", "/etc/ze/c.conf.draft"} {
		if err := s.WriteFile(path, []byte("x"), 0); err != nil {
			t.Fatal(err)
		}
	}

	matches, err := s.List("/etc/ze")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(matches) != 3 {
		t.Errorf("List: got %d matches, want 3: %v", len(matches), matches)
	}
	for _, m := range matches {
		if !strings.HasPrefix(m, "/") {
			t.Errorf("List result should be absolute path: %q", m)
		}
	}
}

// VALIDATES: blob AcquireLock + WriteGuard read/write/remove cycle
// PREVENTS: locked operations failing or lock not released

func TestBlobStorageLockCycle(t *testing.T) {
	s := newBlobStorage(t)

	configPath := "/etc/ze/test.conf"
	if err := s.WriteFile(configPath, []byte("original"), 0); err != nil {
		t.Fatal(err)
	}

	guard, err := s.AcquireLock(configPath)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	// Read within lock
	got, err := guard.ReadFile(configPath)
	if err != nil {
		t.Fatalf("guard.ReadFile: %v", err)
	}
	if string(got) != "original" {
		t.Errorf("guard.ReadFile: got %q", got)
	}

	// Write draft within lock
	draftPath := configPath + ".draft"
	if err := guard.WriteFile(draftPath, []byte("modified"), 0); err != nil {
		t.Fatalf("guard.WriteFile: %v", err)
	}

	// Read back within lock
	got, err = guard.ReadFile(draftPath)
	if err != nil {
		t.Fatalf("guard.ReadFile draft: %v", err)
	}
	if string(got) != "modified" {
		t.Errorf("guard.ReadFile draft: got %q", got)
	}

	// Remove within lock
	if err := guard.Remove(draftPath); err != nil {
		t.Fatalf("guard.Remove: %v", err)
	}

	if err := guard.Release(); err != nil {
		t.Fatalf("guard.Release: %v", err)
	}

	// Draft should be gone after release (flushed)
	if s.Exists(draftPath) {
		t.Error("draft should not exist after remove + release")
	}
}

// VALIDATES: blob migration imports existing files on first create
// PREVENTS: data loss when switching from filesystem to blob

func TestBlobStorageMigration(t *testing.T) {
	dir := t.TempDir()

	// Create files that should be migrated
	if err := os.WriteFile(filepath.Join(dir, "router.conf"), []byte("config-1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "site.conf"), []byte("config-2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "rollback"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollback", "router-001.conf"), []byte("backup"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create blob -- should trigger migration
	blobPath := filepath.Join(dir, "database.zefs")
	s, err := NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	bs, ok := s.(*blobStorage)
	if !ok {
		t.Fatal("expected blobStorage type")
	}
	defer bs.Close() //nolint:errcheck // test cleanup

	// Verify migrated files are in the blob
	routerAbs, err := filepath.Abs(filepath.Join(dir, "router.conf"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadFile(routerAbs)
	if err != nil {
		t.Fatalf("ReadFile router.conf: %v", err)
	}
	if string(got) != "config-1" {
		t.Errorf("router.conf: got %q", got)
	}

	siteAbs, err := filepath.Abs(filepath.Join(dir, "site.conf"))
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.ReadFile(siteAbs)
	if err != nil {
		t.Fatalf("ReadFile site.conf: %v", err)
	}
	if string(got) != "config-2" {
		t.Errorf("site.conf: got %q", got)
	}

	backupAbs, err := filepath.Abs(filepath.Join(dir, "rollback", "router-001.conf"))
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.ReadFile(backupAbs)
	if err != nil {
		t.Fatalf("ReadFile backup: %v", err)
	}
	if string(got) != "backup" {
		t.Errorf("backup: got %q", got)
	}

	// Originals should still exist on filesystem
	if _, statErr := os.Stat(filepath.Join(dir, "router.conf")); statErr != nil {
		t.Error("original router.conf should not be deleted")
	}
}

// VALIDATES: blob storage fallback when blob cannot be created
// PREVENTS: silent failure when blob path is unwritable

func TestBlobStorageFallback(t *testing.T) {
	// Try to create blob in non-existent directory
	_, err := NewBlob("/nonexistent/dir/database.zefs", "/nonexistent/dir")
	if err == nil {
		t.Error("expected error for unwritable blob path")
	}
}

// VALIDATES: blob ReadFile on non-existent key returns error
// PREVENTS: silent empty reads

func TestBlobStorageReadNonExistent(t *testing.T) {
	s := newBlobStorage(t)
	_, err := s.ReadFile("/etc/ze/nonexistent.conf")
	if err == nil {
		t.Error("expected error reading non-existent file")
	}
}

// VALIDATES: blob WriteGuard.Release is idempotent
// PREVENTS: double-release causing errors

func TestBlobStorageGuardDoubleRelease(t *testing.T) {
	s := newBlobStorage(t)

	guard, err := s.AcquireLock("/etc/ze/test.conf")
	if err != nil {
		t.Fatal(err)
	}

	if err := guard.Release(); err != nil {
		t.Fatalf("first Release: %v", err)
	}

	if err := guard.Release(); err != nil {
		t.Errorf("second Release should be nil, got: %v", err)
	}
}

// VALIDATES: blob AcquireLock serializes concurrent goroutine access
// PREVENTS: concurrent writes corrupting blob data

func TestBlobStorageCrossProcessLock(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "test.zefs")
	s, err := NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	defer s.Close() //nolint:errcheck // test cleanup

	configPath := "/etc/ze/test.conf"
	if err := s.WriteFile(configPath, []byte("initial"), 0); err != nil {
		t.Fatal(err)
	}

	// Acquire lock in main goroutine
	guard1, err := s.AcquireLock(configPath)
	if err != nil {
		t.Fatalf("AcquireLock 1: %v", err)
	}

	// Try acquiring lock in second goroutine - should block until first is released
	lockAcquired := make(chan struct{})
	go func() {
		guard2, lockErr := s.AcquireLock(configPath)
		if lockErr != nil {
			t.Errorf("AcquireLock 2: %v", lockErr)
			close(lockAcquired)
			return
		}
		// Write through second guard to prove we have exclusive access
		_ = guard2.WriteFile(configPath, []byte("from-guard2"), 0) //nolint:errcheck // test write
		_ = guard2.Release()                                       //nolint:errcheck // test cleanup
		close(lockAcquired)
	}()

	// Write through first guard
	if err := guard1.WriteFile(configPath, []byte("from-guard1"), 0); err != nil {
		t.Fatalf("guard1.WriteFile: %v", err)
	}

	// Release first lock - second goroutine should proceed
	if err := guard1.Release(); err != nil {
		t.Fatalf("guard1.Release: %v", err)
	}

	// Wait for second goroutine to complete
	<-lockAcquired

	// Final value should be from guard2 (acquired after guard1 released)
	got, err := s.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "from-guard2" {
		t.Errorf("expected 'from-guard2' (last writer), got %q", got)
	}
}

// VALIDATES: blob List filters correctly for .conf files
// PREVENTS: drafts, locks, and non-config files appearing in config selection

func TestBlobStorageListConfigs(t *testing.T) {
	s := newBlobStorage(t)

	// Write a mix of file types
	files := map[string]string{
		"/etc/ze/router.conf":          "config",
		"/etc/ze/site.conf":            "config",
		"/etc/ze/router.conf.draft":    "draft",
		"/etc/ze/router.conf.lock":     "lock",
		"/etc/ze/ssh_host_ed25519_key": "key",
		"/etc/ze/notes.txt":            "text",
	}
	for path, data := range files {
		if err := s.WriteFile(path, []byte(data), 0); err != nil {
			t.Fatal(err)
		}
	}

	matches, err := s.List("/etc/ze")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// List returns all files, not just .conf - filtering is caller's job
	if len(matches) != len(files) {
		t.Errorf("List: got %d matches, want %d: %v", len(matches), len(files), matches)
	}

	// Count .conf files (what doSelectConfig would filter to)
	var confCount int
	for _, m := range matches {
		if strings.HasSuffix(m, ".conf") {
			confCount++
		}
	}
	if confCount != 2 {
		t.Errorf("expected 2 .conf files, got %d", confCount)
	}
}
