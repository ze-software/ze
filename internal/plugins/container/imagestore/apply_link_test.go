//go:build linux

package imagestore

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestApplyHardLinkInsideRoot proves a hard link to a file of the root
// gives two names of one inode, in one layer and across two.
//
// Method: link a name to a file of the same layer, and a name of a second
// layer to a file of the first, then compare the inodes and the link count.
func TestApplyHardLinkInsideRoot(t *testing.T) {
	store := newAppliedStore(t, testLimits())
	store.mustApply(t,
		folderEntry("bin", 0o755),
		fileEntry("bin/busybox", "busybox\n", 0o755),
		hardLinkEntry("bin/sh", "bin/busybox"),
	)
	store.mustApply(t, hardLinkEntry("ls", "./bin/busybox"))

	tree := readTree(t, store.rootPath, true)
	target := tree["bin/busybox"]
	for _, name := range []string{"bin/sh", "ls"} {
		if tree[name].inode != target.inode {
			t.Errorf("%s: inode %d, want the inode of bin/busybox, %d", name, tree[name].inode, target.inode)
		}
	}
	if target.links != 3 {
		t.Errorf("bin/busybox: %d links, want 3", target.links)
	}
	requireContent(t, store.rootPath, "ls", "busybox\n")
}

// TestApplyHardLinkOutsideRootRefused proves a hard link never names a file
// outside the root: through a parent-directory path, an absolute path, or
// a symbolic link that points outside, the image is refused and the file
// outside keeps its one link. A link to something that is not a regular
// file is refused too.
//
// Method: write a secret file beside the store root. A first layer plants
// a link to the folder that holds it and a folder; each later layer tries
// one way to reach the secret. Each must be refused by its rule, the
// secret must keep one link, and nothing outside the staged folder may
// change.
func TestApplyHardLinkOutsideRootRefused(t *testing.T) {
	store := newAppliedStore(t, testLimits())
	secret := store.writeMarker(t, "secret", "secret\n")
	store.mustApply(t,
		symlinkEntry("out", store.dataDir),
		folderEntry("folder", 0o755),
	)
	before := store.outsideSnapshot(t)

	cases := []struct {
		target string
		rule   refusalRule
	}{
		{"../../../secret", ruleParentComponent},
		{secret, ruleAbsolutePath},
		{"out/secret", ruleHardLinkTarget},
		{"out", ruleHardLinkTarget},
		{"folder", ruleHardLinkTarget},
		{"absent", ruleHardLinkTarget},
	}
	for _, c := range cases {
		err := store.applyTar(t, layerTar(t, hardLinkEntry("stolen", c.target)))
		checkRefusal(t, err, c.rule)
		requireAbsent(t, store.rootPath, "stolen")
	}

	var stat syscall.Stat_t
	if err := syscall.Stat(secret, &stat); err != nil {
		t.Fatalf("stat the secret: %v", err)
	}
	if stat.Nlink != 1 {
		t.Errorf("the secret has %d links, want 1", stat.Nlink)
	}
	store.requireOutsideUnchanged(t, before)
}

// TestApplySymlinkParentResolvedInsideRoot proves an entry under a linked
// folder lands where a process in the container would find it: a relative
// link inside the root, an absolute link and a link that climbs out are
// each resolved inside the root, and no write leaves it.
//
// Method: a first layer plants a folder, a relative link to it, an
// absolute link to the folder beside the store root, with the folder of the
// same path inside the root, and a link of many parent steps. A second
// layer writes one file under each link. Each file must be inside the root
// at the place the link resolves to there, and nothing outside the staged
// folder may change.
func TestApplySymlinkParentResolvedInsideRoot(t *testing.T) {
	store := newAppliedStore(t, testLimits())
	store.writeMarker(t, "marker", "marker\n")
	store.mustApply(t,
		folderEntry("real", 0o755),
		symlinkEntry("inside", "real"),
		folderEntry(strings.TrimPrefix(store.dataDir, "/")+"/", 0o755),
		symlinkEntry("outside", store.dataDir),
		symlinkEntry("up", strings.Repeat("../", 16)),
	)
	before := store.outsideSnapshot(t)
	store.mustApply(t,
		fileEntry("inside/a", "a\n", 0o644),
		fileEntry("outside/b", "b\n", 0o644),
		fileEntry("up/c", "c\n", 0o644),
	)

	requireContent(t, store.rootPath, "real/a", "a\n")
	requireContent(t, store.rootPath, filepath.Join(strings.TrimPrefix(store.dataDir, "/"), "b"), "b\n")
	requireContent(t, store.rootPath, "c", "c\n")
	if _, err := os.Lstat(filepath.Join(store.dataDir, "b")); err == nil {
		t.Fatalf("outside/b was written beside the store root")
	}
	store.requireOutsideUnchanged(t, before)
}
