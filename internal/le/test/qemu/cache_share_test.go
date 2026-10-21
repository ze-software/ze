package testqemu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/tmplink"
)

// VALIDATES: a symlinked cache answers one share at the link's own target,
// a real or absent cache answers none (the workspace share carries a real
// one), and a dangling link is an error rather than no share.
// PREVENTS: a run guest whose le cross-compiles with GOCACHE under
// /workspace/cache meeting the link dangling ("mkdir /workspace/cache: file
// exists", 2026-10-12).
func TestACacheLinkIsOneShare(t *testing.T) {
	target := t.TempDir()
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	linked := t.TempDir()
	if err := os.Symlink(target, filepath.Join(linked, "cache")); err != nil {
		t.Fatal(err)
	}
	share, ok, err := cacheShare(linked)
	if err != nil {
		t.Fatal(err)
	}
	want := tmplink.Share{Tag: cacheShareTag, Host: resolved, Guest: target}
	if !ok || share != want {
		t.Fatalf("share = %+v, %v; want %+v, true", share, ok, want)
	}

	plain := t.TempDir()
	if err := os.Mkdir(filepath.Join(plain, "cache"), 0o750); err != nil {
		t.Fatal(err)
	}
	for name, tree := range map[string]string{"real": plain, "absent": t.TempDir()} {
		share, ok, err := cacheShare(tree)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if ok {
			t.Fatalf("%s cache answered a share: %+v", name, share)
		}
	}

	dangling := t.TempDir()
	if err := os.Symlink(filepath.Join(target, "gone"), filepath.Join(dangling, "cache")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cacheShare(dangling); err == nil {
		t.Fatal("a dangling cache link answered no error")
	}
}
