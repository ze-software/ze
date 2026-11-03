// Design: docs/architecture/zefs-format.md -- retained artifact directory.
package zefs

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestBlobDirectoryReplacement proves both write forms keep the opened folder.
// It replaces the folder name with a sentinel before growing and updating a key.
func TestBlobDirectoryReplacement(t *testing.T) {
	for _, inplace := range []bool{false, true} {
		t.Run(map[bool]string{false: "rewrite", true: "in-place"}[inplace], func(t *testing.T) {
			base := t.TempDir()
			dir := filepath.Join(base, "owned")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "test.zefs")
			store, err := Create(path)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close() //nolint:errcheck // Test cleanup.
			if err := store.WriteFile("key", []byte("original"), 0); err != nil {
				t.Fatal(err)
			}
			held := dir + "-held"
			if err := os.Rename(dir, held); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			sentinel := []byte("foreign bytes must not change")
			if err := os.WriteFile(path, sentinel, 0o640); err != nil {
				t.Fatal(err)
			}
			value := []byte("a longer value requiring a complete rewrite")
			if inplace {
				value = []byte("updated")
			}
			if err := store.WriteFile("key", value, 0); err != nil {
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
				t.Errorf("foreign mode changed: %v", info.Mode())
			}
			reopened, err := Open(filepath.Join(held, "test.zefs"))
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close() //nolint:errcheck // Test cleanup.
			got, err = reopened.ReadFile("key")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, value) {
				t.Errorf("owned value = %q, want %q", got, value)
			}
		})
	}
}

// TestBlobPathAliasContract proves ordinary aliases retain their historical
// in-place-target/full-rewrite-link behavior, while rooted opens refuse links.
func TestBlobPathAliasContract(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires platform-specific privileges")
	}
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "target.zefs")
	seed, err := Create(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.WriteFile("key", []byte("original"), 0); err != nil {
		t.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "alias.zefs")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // Test cleanup.
	if err := store.WriteFile("key", []byte("changed"), 0); err != nil {
		t.Fatal(err)
	}
	targetStore, err := Open(target)
	if err != nil {
		t.Fatal(err)
	}
	got, err := targetStore.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "changed" {
		t.Errorf("in-place target = %q", got)
	}
	if err := targetStore.Close(); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close() //nolint:errcheck // Test cleanup.
	if rooted, err := OpenIn(root, "alias.zefs"); err == nil {
		rooted.Close() //nolint:errcheck // Unexpected successful open.
		t.Fatal("rooted open accepted external symlink")
	}
	if rooted, err := CreateIn(root, "alias.zefs"); err == nil {
		rooted.Close() //nolint:errcheck // Unexpected successful create.
		t.Fatal("rooted create accepted external symlink")
	}
	if err := store.WriteFile("key", []byte("longer value replaces the alias itself"), 0); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Errorf("full rewrite left alias: %v", info.Mode())
	}
	targetStore, err = Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer targetStore.Close() //nolint:errcheck // Test cleanup.
	got, err = targetStore.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "changed" {
		t.Errorf("full rewrite changed target: %q", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "missing"), path); err != nil {
		t.Fatal(err)
	}
	created, err := Create(path)
	if err != nil {
		t.Fatalf("ordinary create over dangling alias: %v", err)
	}
	if err := created.Close(); err != nil {
		t.Fatal(err)
	}
	info, err = os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Errorf("ordinary create left alias: %v", info.Mode())
	}
}

// TestBlobLeafReplacement refuses a replaced leaf without reading or changing
// its external target, for both the full-rewrite and in-place write forms.
func TestBlobLeafReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires platform-specific privileges")
	}
	for _, value := range []string{"small", "a value large enough to require a full rewrite"} {
		t.Run(value, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "blob")
			store, err := Create(path)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close() //nolint:errcheck // Test cleanup.
			if err := store.WriteFile("key", []byte("original"), 0); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(t.TempDir(), "sentinel")
			if err := os.WriteFile(outside, []byte("foreign"), 0o640); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(path, path+"-held"); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
			if err := store.WriteFile("key", []byte(value), 0); err == nil {
				t.Fatal("write accepted replaced leaf")
			}
			got, err := os.ReadFile(outside)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "foreign" {
				t.Errorf("foreign bytes changed: %q", got)
			}
			info, err := os.Stat(outside)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o640 {
				t.Errorf("foreign mode changed: %v", info.Mode())
			}
		})
	}
}

// TestBlobRootConstructors proves the passed root survives both a path swap and
// closure by its caller, and refuses names that could leave the artifact folder.
func TestBlobRootConstructors(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "owned")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close() //nolint:errcheck // Also closed below to test independent ownership.
	for _, name := range []string{"", ".", "../escape", "nested/blob", "/absolute"} {
		if store, err := CreateIn(root, name); err == nil {
			store.Close() //nolint:errcheck // Unexpected successful open.
			t.Errorf("accepted invalid artifact name %q", name)
		}
	}
	if _, err := OpenIn(nil, "blob"); err == nil {
		t.Fatal("accepted nil root")
	}
	if err := os.Rename(dir, dir+"-held"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := CreateIn(root, "blob")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // Test cleanup.
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile("key", []byte("held"), 0); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("replacement directory written: %v", entries)
	}
	reopened, err := Open(filepath.Join(dir+"-held", "blob"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close() //nolint:errcheck // Test cleanup.
	got, err := reopened.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "held" {
		t.Errorf("owned key = %q", got)
	}
}

// TestBlobRootUnavailable refuses invalid resources without publishing a blob.
// Closed roots and directory leaves exercise operating-system errors directly.
func TestBlobRootUnavailable(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Mkdir("directory", 0o700); err != nil {
		t.Fatal(err)
	}
	if store, err := OpenIn(root, "directory"); err == nil {
		store.Close() //nolint:errcheck // Unexpected successful open.
		t.Fatal("accepted directory as a blob")
	}
	if store, err := CreateIn(root, "directory"); err == nil {
		store.Close() //nolint:errcheck // Unexpected successful create.
		t.Fatal("replaced directory with a blob")
	}
	if store, err := OpenIn(root, "missing"); err == nil {
		store.Close() //nolint:errcheck // Unexpected successful open.
		t.Fatal("accepted missing blob")
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if store, err := CreateIn(root, "blob"); err == nil {
		store.Close() //nolint:errcheck // Unexpected successful create.
		t.Fatal("accepted closed root")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "directory" {
		t.Errorf("refused operations changed directory: %v", entries)
	}
}
