// Design: docs/architecture/zefs-format.md -- mutation descriptor admission.
package zefs

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestRootedMutationChecksReplacementDescriptor distinguishes checking the
// opened write target from checking the store's earlier mapped descriptor.
func TestRootedMutationChecksReplacementDescriptor(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close() //nolint:errcheck // Test root.
	store, err := CreateIn(root, "blob")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // Test store.
	if err := store.WriteFile("key", []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "blob"))
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, before, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := root.Rename("blob", "held"); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(dir, "blob")); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile("key", []byte("after!"), 0o600); err == nil {
		t.Error("accepted hard-linked replacement")
	}
	after, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("outside bytes changed")
	}
	info, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("outside mode changed: %v", info.Mode())
	}
}
