//go:build linux

package imagestore

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestApplyWhiteoutRemovesLowerEntry proves a whiteout removes a name an
// earlier layer applied, with everything under it, and touches nothing
// else.
//
// Method: a first layer applies a folder with a subtree, a file beside it
// and a file of the same prefix; a second layer whites out the folder, the
// lone file under another folder, and a name no layer applied. The folder
// and its subtree must be gone, the whited-out file gone, the rest there.
func TestApplyWhiteoutRemovesLowerEntry(t *testing.T) {
	store := newAppliedStore(t, testLimits())
	store.mustApply(t,
		folderEntry("a", 0o755),
		folderEntry("a/b", 0o755),
		fileEntry("a/b/c", "deep\n", 0o644),
		fileEntry("ab", "keep\n", 0o644),
		folderEntry("etc", 0o755),
		fileEntry("etc/old", "old\n", 0o644),
		fileEntry("etc/kept", "kept\n", 0o644),
	)
	store.mustApply(t,
		fileEntry(".wh.a", "", 0o644),
		fileEntry("etc/.wh.old", "", 0o644),
		fileEntry(".wh.never-applied", "", 0o644),
		fileEntry("missing/.wh.name", "", 0o644),
	)
	requireAbsent(t, store.rootPath, "a")
	requireAbsent(t, store.rootPath, "etc/old")
	requireAbsent(t, store.rootPath, ".wh.a")
	requireAbsent(t, store.rootPath, "etc/.wh.old")
	requireAbsent(t, store.rootPath, "missing")
	requireContent(t, store.rootPath, "ab", "keep\n")
	requireContent(t, store.rootPath, "etc/kept", "kept\n")

	err := store.applyTar(t, layerTar(t, fileEntry("etc/.wh.", "", 0o644)))
	checkRefusal(t, err, ruleWhiteoutTarget)
}

// TestApplyOpaqueDirectoryKeepsSameLayerEntries proves an opaque marker
// removes the children its folder had before the layer, and keeps the
// children the layer itself adds, whatever their place in the archive.
//
// Method: in two staged roots, a first layer applies a folder with two
// children, one of them a folder with a file. A second layer adds two
// children to the folder, one before the marker and one after it, in the
// first root; in the second root the marker comes first. Each folder must
// then hold exactly the two new children. Another marker of the family is
// never created.
func TestApplyOpaqueDirectoryKeepsSameLayerEntries(t *testing.T) {
	lower := []testEntry{
		folderEntry("d", 0o755),
		fileEntry("d/old", "old\n", 0o644),
		folderEntry("d/sub", 0o755),
		fileEntry("d/sub/older", "older\n", 0o644),
		fileEntry("beside", "beside\n", 0o644),
	}
	orders := map[string][]testEntry{
		"marker-between": {
			folderEntry("d", 0o755),
			fileEntry("d/before", "before\n", 0o644),
			fileEntry("d/.wh..wh..opq", "", 0o644),
			fileEntry("d/after", "after\n", 0o644),
			fileEntry("d/.wh..wh.plnk", "", 0o644),
		},
		"marker-first": {
			fileEntry("d/.wh..wh..opq", "", 0o644),
			fileEntry("d/before", "before\n", 0o644),
			fileEntry("d/after", "after\n", 0o644),
		},
	}
	root := preparedRoot(t)
	for staged, upper := range orders {
		store := newAppliedStoreIn(t, root, staged, testLimits())
		store.mustApply(t, lower...)
		store.mustApply(t, upper...)

		entries, err := os.ReadDir(filepath.Join(store.rootPath, "d"))
		if err != nil {
			t.Fatalf("%s: read d: %v", staged, err)
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		slices.Sort(names)
		if !slices.Equal(names, []string{"after", "before"}) {
			t.Errorf("%s: d holds %v, want [after before]", staged, names)
		}
		requireContent(t, store.rootPath, "beside", "beside\n")
	}
}
