package testqemu

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDanglingScratchLinkNamesLinkTargetAndFix proves a QEMU run refuses a
// dangling scratch link with the link, its missing target and the command that
// recreates the target, in place of `mkdir <tree>/tmp/qemu: file exists`.
// Method: a checkout whose tmp/qemu (under a real tmp), tmp itself, or cache
// links to a directory that is gone is handed to ensureScratch and
// scratchShares, the two entry points that meet the link; each refusal is
// compared whole. A checkout whose links all resolve answers nil.
func TestDanglingScratchLinkNamesLinkTargetAndFix(t *testing.T) {
	cases := []struct {
		name string
		link func(tree string) string
	}{
		{"tmp/qemu under a real tmp", func(tree string) string {
			if err := os.Mkdir(filepath.Join(tree, "tmp"), 0o750); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(tree, "tmp", "qemu")
		}},
		{"tmp", func(tree string) string { return filepath.Join(tree, "tmp") }},
		{"cache", func(tree string) string { return filepath.Join(tree, "cache") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := t.TempDir()
			target := filepath.Join(t.TempDir(), "gone")
			link := tc.link(tree)
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			want := link + " is a scratch link to " + target + ", which does not exist\n" +
				"the QEMU run writes its caches and guest shares through this link, so every write through it fails\n" +
				"fix: mkdir -p '" + target + "'"

			run := &Run{Tree: tree}
			if err := run.ensureScratch(); err == nil || err.Error() != want {
				t.Errorf("ensureScratch = %v\nwant %s", err, want)
			}
			if _, err := run.scratchShares(GuestWorkspace); err == nil || err.Error() != want {
				t.Errorf("scratchShares = %v\nwant %s", err, want)
			}
		})
	}

	t.Run("relative target resolves against the link's directory", func(t *testing.T) {
		tree := t.TempDir()
		if err := os.Symlink(filepath.Join("..", "gone-cache"), filepath.Join(tree, "cache")); err != nil {
			t.Fatal(err)
		}
		_, dangling, err := linkTargetMissing(filepath.Join(tree, "cache"))
		if err != nil || !dangling {
			t.Fatalf("relative dangling cache link: dangling %v, err %v", dangling, err)
		}
	})

	t.Run("resolving links pass", func(t *testing.T) {
		tree := t.TempDir()
		target := t.TempDir()
		if err := os.Symlink(target, filepath.Join(tree, "tmp")); err != nil {
			t.Fatal(err)
		}
		if err := danglingScratchLink(tree); err != nil {
			t.Errorf("a resolving tmp link was refused: %v", err)
		}
	})
}
