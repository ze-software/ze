package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewBlobSelfHealsCorruptStore verifies a 0-byte (unreadable) blob does not
// wedge storage: NewBlob moves it aside to .replaced-<date> and recreates a
// usable store instead of returning an error.
//
// VALIDATES: NewBlob recovers from a corrupt store instead of failing forever.
// PREVENTS: regression of the zefs "mmap: empty file" wedge, where NewBlob chose
// Open vs Create by file existence (os.Stat) rather than validity, so a 0-byte
// database.zefs left by an interrupted/concurrent write broke every startup.
func TestNewBlobSelfHealsCorruptStore(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "database.zefs")

	// Simulate the corruption: a 0-byte store left by an interrupted write.
	if err := os.WriteFile(blobPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("NewBlob on 0-byte store returned error, want self-heal: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	// The corrupt file is preserved aside for post-mortem, not destroyed.
	aside, _ := filepath.Glob(blobPath + ".replaced-*")
	if len(aside) != 1 {
		t.Fatalf("want exactly one .replaced-<date> backup, got %d", len(aside))
	}

	// A fresh, non-empty, usable store now exists at the original path.
	info, statErr := os.Stat(blobPath)
	if statErr != nil {
		t.Fatalf("recreated blob missing: %v", statErr)
	}
	if info.Size() == 0 {
		t.Fatal("recreated blob is still 0 bytes")
	}
	name := filepath.Join(dir, "probe.conf")
	if err := st.WriteFile(name, []byte("hello"), 0); err != nil {
		t.Fatalf("recreated store not writable: %v", err)
	}
	got, err := st.ReadFile(name)
	if err != nil || string(got) != "hello" {
		t.Fatalf("recreated store read-back = %q, %v; want \"hello\"", got, err)
	}
}

// TestNewBlobOpensValidStoreInPlace verifies a valid existing store is opened as
// is: no .replaced backup is made and previously written data survives a
// close/reopen. This is the guard that the self-heal path does not fire on
// healthy stores.
func TestNewBlobOpensValidStoreInPlace(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "database.zefs")
	name := filepath.Join(dir, "keep.conf")

	first, err := NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("initial NewBlob: %v", err)
	}
	if err := first.WriteFile(name, []byte("persisted"), 0); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, err := NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("reopen NewBlob: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	if aside, _ := filepath.Glob(blobPath + ".replaced-*"); len(aside) != 0 {
		t.Errorf("valid store must not be moved aside, found %d backups", len(aside))
	}
	got, err := second.ReadFile(name)
	if err != nil || string(got) != "persisted" {
		t.Fatalf("reopened store read-back = %q, %v; want \"persisted\"", got, err)
	}
}
