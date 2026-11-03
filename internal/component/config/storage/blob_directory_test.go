// Design: docs/architecture/storage-backends.md -- retained blob publication directory.
package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestBlobPublicationRetainsDirectory replaces the output folder during seed
// population and proves publication and subsequent writes use the original.
func TestBlobPublicationRetainsDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "owned")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "seed.zefs")
	sentinel := []byte("foreign artifact\n")
	store, err := CreateBlobPopulated(path, func(seed Storage) error {
		if err := seed.WriteKey("meta/test", []byte("seed")); err != nil {
			return err
		}
		if err := os.Rename(dir, dir+"-held"); err != nil {
			return err
		}
		if err := os.Mkdir(dir, 0o700); err != nil {
			return err
		}
		return os.WriteFile(path, sentinel, 0o640)
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteKey("meta/test", []byte("updated retained artifact")); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, sentinel) {
		t.Errorf("foreign artifact changed: %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("foreign artifact mode changed: %v", info.Mode())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("foreign directory has extra entries: %v", entries)
	}
	reopened, err := OpenBlob(filepath.Join(dir+"-held", "seed.zefs"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close() //nolint:errcheck // Read-only test cleanup.
	got, err = reopened.ReadKey("meta/test")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "updated retained artifact" {
		t.Errorf("owned artifact = %q", got)
	}
}

// TestBlobRootRejectsReplacement proves conversion validates the opened roots,
// rather than trusting the former name of an already retained storage folder.
func TestBlobRootRejectsReplacement(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "owned")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	folder, err := openFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer folder.Close() //nolint:errcheck // Test cleanup.
	if err := os.Rename(dir, dir+"-held"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := blobRoot(folder)
	if root != nil {
		root.Close() //nolint:errcheck // Unexpected successful conversion.
		t.Fatal("accepted replacement directory")
	}
	if !errors.Is(err, ErrPermissions) {
		t.Fatalf("replacement error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("replacement directory written: %v", entries)
	}
}
