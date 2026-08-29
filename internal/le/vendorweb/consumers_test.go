// Related: sync.go -- consumers and targetedAssets, vendorweb.go -- consumerDirs

package vendorweb

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// repoRoot walks up from the working directory to the checkout holding go.mod.
// This test judges the real tree, because what it guards is a directory
// somebody adds.
func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("read the working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", dir)
		}
		dir = parent
	}
}

// VALIDATES: every assets/ directory the tree holds is named by the sync's
// policy, either in consumers or as the destination of a targeted asset.
// PREVENTS: a new consumer being invisible to BOTH halves of this program --
// unsynced by Sync, which iterates a list that does not name it, and unchecked
// by driftCheck, which compares only the packages a consumer already subscribes
// to and so compares nothing for a directory holding no copies yet.
//
// consumerDirs is the derivation the check half already uses, so this test
// costs no second walk of its own.
func TestEveryAssetDirectoryIsDecided(t *testing.T) {
	root := repoRoot(t)

	found, err := consumerDirs(root)
	if err != nil {
		t.Fatalf("consumerDirs: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("no assets/ directory was found, so this test asserted nothing")
	}

	targeted := map[string]bool{}
	for _, a := range targetedAssets {
		targeted[filepath.ToSlash(a.dest)] = true
	}

	for _, dir := range found {
		rel := filepath.ToSlash(dir)
		if slices.Contains(consumers, rel) || targeted[rel] {
			continue
		}
		t.Errorf("%s holds embedded assets and neither consumers nor targetedAssets names it, "+
			"so Sync never writes to it and driftCheck never compares it", rel)
	}
}
