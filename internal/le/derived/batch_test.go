// Related: derived.go -- WriteAtomicAll, the batch publish
//
// VALIDATES: a batch publishes every file whole, leaves no temporary behind,
// and a failure part way leaves each file it did not reach as it was.
// PREVENTS: a batch that trades the per-file sync for a partial or empty file a
// reader takes for the answer, and temporaries that pile up beside the
// artifacts after a failed run.
package derived_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/derived"
)

// TestWriteAtomicAllPublishesEveryFile writes into two directories, over one
// existing file and beside one new one.
func TestWriteAtomicAllPublishesEveryFile(t *testing.T) {
	root := t.TempDir()
	shards := filepath.Join(root, "shards")
	if err := os.Mkdir(shards, 0o750); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(shards, "a.md")
	writeFile(t, existing, "old a\n")

	files := []derived.File{
		{Path: existing, Content: []byte("new a\n")},
		{Path: filepath.Join(shards, "b.md"), Content: []byte("new b\n")},
		{Path: filepath.Join(root, "index.md"), Content: []byte("index\n")},
	}
	if err := derived.WriteAtomicAll(files); err != nil {
		t.Fatalf("WriteAtomicAll: %v", err)
	}
	for _, file := range files {
		assertContent(t, file.Path, string(file.Content))
		info, err := os.Stat(file.Path)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != 0o644 {
			t.Errorf("%s is %04o, and a tracked page it replaces is 0644", file.Path, mode)
		}
	}
	assertNoTemporary(t, shards)
	assertNoTemporary(t, root)
}

// TestWriteAtomicAllFailingBeforeARenameChangesNothing fails the write of the
// SECOND file, after the first one's temporary exists.
func TestWriteAtomicAllFailingBeforeARenameChangesNothing(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "a.md")
	writeFile(t, existing, "old a\n")

	err := derived.WriteAtomicAll([]derived.File{
		{Path: existing, Content: []byte("new a\n")},
		{Path: filepath.Join(root, "absent", "b.md"), Content: []byte("new b\n")},
	})
	if err == nil {
		t.Fatal("a batch whose second directory is absent answered no error")
	}
	assertContent(t, existing, "old a\n")
	assertNoTemporary(t, root)
}

// TestWriteAtomicAllFailingARenameLeavesEachFileWhole fails the rename of the
// SECOND file: a directory stands at its path.
func TestWriteAtomicAllFailingARenameLeavesEachFileWhole(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "a.md")
	blocked := filepath.Join(root, "b.md")
	last := filepath.Join(root, "c.md")
	writeFile(t, first, "old a\n")
	writeFile(t, last, "old c\n")
	if err := os.Mkdir(blocked, 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(blocked, "keep"), "x")

	err := derived.WriteAtomicAll([]derived.File{
		{Path: first, Content: []byte("new a\n")},
		{Path: blocked, Content: []byte("new b\n")},
		{Path: last, Content: []byte("new c\n")},
	})
	if err == nil {
		t.Fatal("a batch whose second rename cannot happen answered no error")
	}
	assertContent(t, first, "new a\n")
	assertContent(t, last, "old c\n")
	assertNoTemporary(t, root)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s holds %q, want %q", path, got, want)
	}
}

func assertNoTemporary(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Errorf("a temporary is left behind: %s", filepath.Join(directory, entry.Name()))
		}
	}
}
