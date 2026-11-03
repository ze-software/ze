// Design: docs/architecture/zefs-format.md -- rooted mutation admission.
package zefs

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestRootedMutationHardlink checks the actual write target after opening a
// rooted store, while retaining ordinary path callers' hardlink semantics.
func TestRootedMutationHardlink(t *testing.T) {
	for _, rooted := range []bool{true, false} {
		t.Run(map[bool]string{true: "rooted", false: "ordinary"}[rooted], func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "store")
			store, err := Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.WriteFile("key", []byte("before"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close() //nolint:errcheck // Test directory.
			if rooted {
				store, err = OpenIn(root, "store")
			} else {
				store, err = Open(path)
			}
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close() //nolint:errcheck // Test store.
			outside := filepath.Join(t.TempDir(), "outside")
			if err := os.Link(path, outside); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(outside)
			if err != nil {
				t.Fatal(err)
			}
			err = store.WriteFile("key", []byte("after!"), 0o600)
			if rooted && err == nil {
				t.Error("rooted mutation accepted hardlink")
			}
			if !rooted && err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(outside)
			if err != nil {
				t.Fatal(err)
			}
			if rooted && !bytes.Equal(before, after) {
				t.Error("rooted write changed outside bytes")
			}
			if !rooted && bytes.Equal(before, after) {
				t.Error("ordinary in-place hardlink update lost")
			}
		})
	}
}
