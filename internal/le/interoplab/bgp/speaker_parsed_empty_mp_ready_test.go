// Design: docs/architecture/testing/interop.md -- the native speaker's readiness fence.
package bgp

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestParsedEmptyMPReadyOwnsItsFile proves the readiness writer creates its own
// marker and refuses existing files and symlinks without changing their contents.
// Every path belongs to this test; the lab's fixed /tmp paths are never touched.
func TestParsedEmptyMPReadyOwnsItsFile(t *testing.T) {
	t.Run("new marker", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ready")
		if err := writeParsedEmptyMPReady(path); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "established\n" {
			t.Fatalf("marker = %q, want establishment fence", content)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("marker permissions = %v, want 0600", info.Mode().Perm())
		}
	})
	for _, symlink := range []bool{false, true} {
		name := "existing file"
		if symlink {
			name = "symlink"
		}
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			target := filepath.Join(directory, "target")
			const original = "owned by another writer\n"
			if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			path := target
			if symlink {
				path = filepath.Join(directory, "ready")
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			if err := writeParsedEmptyMPReady(path); !errors.Is(err, os.ErrExist) {
				t.Errorf("existing marker error = %v, want file-exists refusal", err)
			}
			content, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != original {
				t.Errorf("existing target overwritten: got %q, want %q", content, original)
			}
		})
	}
}
