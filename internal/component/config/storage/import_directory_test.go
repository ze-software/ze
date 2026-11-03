// Design: docs/architecture/storage-backends.md -- retained import and replay directories.
package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestImportSourceDirectoryReplacement replaces both source and destination
// directories after acquisition. Fresh import and recovery must publish the
// accepted bytes and leave the replacement directories byte-for-byte untouched.
func TestImportSourceDirectoryReplacement(t *testing.T) {
	for _, replay := range []bool{false, true} {
		name := "fresh"
		if replay {
			name = "replay"
		}
		t.Run(name, func(t *testing.T) {
			path, values := importFixture(t)
			dir := t.TempDir()
			policy := policyRetireSource
			if replay {
				path, dir, values, _ = interruptedRestore(t, "pre-move")
				policy = policyKeepSource
			}
			source, err := openImportSource(path)
			if err != nil {
				t.Fatal(err)
			}
			defer source.close() //nolint:errcheck // Test cleanup.
			folder, err := openFolder(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer folder.Close() //nolint:errcheck // Test cleanup.
			owner, err := lockOwner(folder, lockName)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close() //nolint:errcheck // Also released by the returned store.
			sourceDir := filepath.Dir(path)
			for _, replaced := range []string{sourceDir, dir} {
				if err := os.Rename(replaced, replaced+"-held"); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.RemoveAll(replaced + "-held"); err != nil {
						t.Error(err)
					}
				})
				if err := os.Mkdir(replaced, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(replaced, "sentinel"), []byte("foreign"), 0o640); err != nil {
					t.Fatal(err)
				}
			}
			store, err := importOwnedSource(source, folder, owner, policy, replay)
			if err != nil {
				t.Fatal(err)
			}
			assertImportValues(t, store, values)
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			for _, replaced := range []string{sourceDir, dir} {
				entries, err := os.ReadDir(replaced)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 1 {
					t.Errorf("replacement %s was written: %v", replaced, entries)
				}
				file := filepath.Join(replaced, "sentinel")
				got, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, []byte("foreign")) {
					t.Errorf("replacement sentinel changed: %q", got)
				}
				info, err := os.Stat(file)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0o640 {
					t.Errorf("replacement sentinel mode changed: %v", info.Mode())
				}
			}
			if _, err := os.Stat(filepath.Join(dir+"-held", treeName)); err != nil {
				t.Fatal(err)
			}
			heldPath := filepath.Join(sourceDir+"-held", filepath.Base(path))
			if replay {
				if _, err := os.Stat(heldPath); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := os.Stat(heldPath); !os.IsNotExist(err) {
					t.Fatalf("accepted source was not retired: %v", err)
				}
				if len(archivesOf(t, heldPath)) != 1 {
					t.Fatal("accepted source archive missing")
				}
			}
		})
	}
}
