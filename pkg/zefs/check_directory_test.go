// Design: docs/architecture/zefs-format.md -- retained artifact integrity checks.
package zefs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestCheckInRetainsDirectory checks the accepted artifact after its parent is
// replaced, then proves a closed root and a symlink leaf refuse inspection.
func TestCheckInRetainsDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires platform-specific privileges")
	}
	base := t.TempDir()
	dir := filepath.Join(base, "owned")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := Create(filepath.Join(dir, "blob"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile("key", []byte("accepted"), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close() //nolint:errcheck // Also closed below to check refusal.
	if err := os.Rename(dir, dir+"-held"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "blob"), []byte("foreign"), 0o640); err != nil {
		t.Fatal(err)
	}
	report, err := CheckIn(root, "blob")
	if err != nil {
		t.Fatal(err)
	}
	if !report.MagicOK {
		t.Fatal("accepted artifact magic was not checked")
	}
	if !report.ContainerOK {
		t.Fatal("accepted container was not checked")
	}
	if report.TotalEntries != 1 {
		t.Fatalf("accepted entry count = %d", report.TotalEntries)
	}
	if report.Entries[0].Key != "key" {
		t.Errorf("accepted key = %q", report.Entries[0].Key)
	}
	if err := os.Rename(filepath.Join(dir+"-held", "blob"), filepath.Join(dir+"-held", "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "blob"), filepath.Join(dir+"-held", "blob")); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckIn(root, "blob"); err == nil {
		t.Fatal("accepted symlink replacement")
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckIn(root, "original"); err == nil {
		t.Fatal("accepted closed root")
	}
}
