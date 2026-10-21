package gen

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// keyWithLinks builds a key directory whose goroot holds a VERSION link and a
// src/go.mod link, each naming a real file or a path that does not exist.
func keyWithLinks(t *testing.T, versionLive, goModLive bool) string {
	t.Helper()
	real := t.TempDir()
	key := filepath.Join(t.TempDir(), "key")
	goroot := filepath.Join(key, "goroot")
	if err := os.MkdirAll(filepath.Join(goroot, "src"), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, link := range []struct {
		name string
		live bool
	}{{"VERSION", versionLive}, {filepath.Join("src", "go.mod"), goModLive}} {
		target := filepath.Join(real, "gone", link.name)
		if link.live {
			target = filepath.Join(real, filepath.Base(link.name))
			if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Symlink(target, filepath.Join(goroot, link.name)); err != nil {
			t.Fatal(err)
		}
	}
	return key
}

// TestDropDeadTreeJudgesTheGoModLink: a tree whose VERSION link still
// resolves and whose src/go.mod link dangles is dead, because without that
// link goroot/src stops being a module of its own and `go list ./...` walks it
// as packages of the main module. dropDeadTree removes it, and keeps a tree
// whose two links both resolve.
// PREVENTS: the src/go.mod half of DeadTree being dropped while the VERSION
// tests stay green.
func TestDropDeadTreeJudgesTheGoModLink(t *testing.T) {
	dead := keyWithLinks(t, true, false)
	if err := dropDeadTree(dead); err != nil {
		t.Fatalf("dropDeadTree: %v", err)
	}
	if _, err := os.Lstat(dead); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a tree whose src/go.mod link dangles was kept (lstat: %v)", err)
	}

	live := keyWithLinks(t, true, true)
	if err := dropDeadTree(live); err != nil {
		t.Fatalf("dropDeadTree: %v", err)
	}
	if _, err := os.Lstat(live); err != nil {
		t.Errorf("a tree whose links both resolve was removed: %v", err)
	}
}
