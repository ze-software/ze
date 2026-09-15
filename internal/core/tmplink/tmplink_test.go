package tmplink

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// VALIDATES: a symlinked tmp answers one share whose host is the resolved
// target and whose guest path is the link's own target text, absolute or
// workspace-joined.
// PREVENTS: a guest mounting the export somewhere the symlink does not lead.
func TestASymlinkedTmpIsOneShare(t *testing.T) {
	target := t.TempDir()
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		link  string
		guest string
	}{
		{name: "absolute", link: target, guest: target},
		{name: "relative", link: "../elsewhere", guest: "/elsewhere"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := t.TempDir()
			if tc.link == "../elsewhere" {
				if err := os.Symlink(target, filepath.Join(filepath.Dir(tree), "elsewhere")); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(tc.link, filepath.Join(tree, "tmp")); err != nil {
				t.Fatal(err)
			}
			shares, err := Shares(tree, "/workspace")
			if err != nil {
				t.Fatal(err)
			}
			want := []Share{{Tag: "zescratch", Host: resolved, Guest: tc.guest}}
			if !reflect.DeepEqual(shares, want) {
				t.Fatalf("shares = %+v, want %+v", shares, want)
			}
		})
	}
}

// VALIDATES: a real tmp answers one share for each Migratable child that is a
// symlink, and none for a real child, an absent child, or a symlink outside
// the allowlist.
// PREVENTS: the guest's /workspace/tmp/qemu dangling after `le scratch
// migrate` relocated only the children (the 2026-09-15 kernel build failure).
func TestARealTmpSharesEachSymlinkedMigratableChild(t *testing.T) {
	tree := t.TempDir()
	target := t.TempDir()
	tmp := filepath.Join(tree, "tmp")
	if err := os.MkdirAll(filepath.Join(tmp, "gokrazy"), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"qemu", "kernel"} {
		if err := os.MkdirAll(filepath.Join(target, name), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(target, name), filepath.Join(tmp, name)); err != nil {
			t.Fatal(err)
		}
	}
	// go-cache is relocated by the cache repoint, and the guest never reads
	// it: it is not Migratable, so it earns no share.
	if err := os.Symlink(target, filepath.Join(tmp, "go-cache")); err != nil {
		t.Fatal(err)
	}
	shares, err := Shares(tree, "/workspace")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	want := []Share{
		{Tag: "zescratch-qemu", Host: filepath.Join(resolved, "qemu"), Guest: filepath.Join(target, "qemu")},
		{Tag: "zescratch-kernel", Host: filepath.Join(resolved, "kernel"), Guest: filepath.Join(target, "kernel")},
	}
	if !reflect.DeepEqual(shares, want) {
		t.Fatalf("shares = %+v, want %+v", shares, want)
	}
}

// VALIDATES: a relative child link is anchored at the guest's tmp, not at the
// workspace root, and a tree with no tmp is an error rather than no shares.
// PREVENTS: a silent empty answer for a tree the guest cannot use.
func TestARelativeChildLinkAnchorsAtTheGuestTmp(t *testing.T) {
	tree := t.TempDir()
	tmp := filepath.Join(tree, "tmp")
	if err := os.MkdirAll(filepath.Join(tmp, "relocated"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("relocated", filepath.Join(tmp, "qemu")); err != nil {
		t.Fatal(err)
	}
	shares, err := Shares(tree, "/workspace")
	if err != nil {
		t.Fatal(err)
	}
	if len(shares) != 1 || shares[0].Guest != "/workspace/tmp/relocated" {
		t.Fatalf("shares = %+v, want one at /workspace/tmp/relocated", shares)
	}
	if _, err := Shares(t.TempDir(), "/workspace"); err == nil {
		t.Fatal("a tree without tmp answered shares")
	}
}
