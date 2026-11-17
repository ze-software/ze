//go:build linux

package imagestore

import (
	"archive/tar"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/plugins/container/registry"
)

// FuzzApplyLayerNeverEscapesRoot proves no archive changes anything outside
// the staged folder: not the marker file beside the store root, not the
// store's other folders, and no new name anywhere outside.
//
// Method: each run opens a fresh store, writes a marker beside its root,
// and applies a fixed lower layer that plants what a hostile image would:
// a link to the folder beside the store root, a link that climbs out, and
// a link to the marker itself. It then applies the fuzzed archive against
// its own digest, so the digest check does not stop it first, and compares
// everything outside the staged folder with what was there before. Any
// answer from the applier is allowed, a refusal included: only a change
// outside fails the run. The seeds are one hostile entry each.
func FuzzApplyLayerNeverEscapesRoot(f *testing.F) {
	seeds := [][]testEntry{
		{fileEntry("../escape", "x", 0o644)},
		{fileEntry("/escape", "x", 0o644)},
		{fileEntry("outside/escape", "x", 0o644)},
		{fileEntry("up/escape", "x", 0o644)},
		{fileEntry("marker", "overwritten", 0o644)},
		{folderEntry("outside/", 0o777)},
		{hardLinkEntry("stolen", "marker"), hardLinkEntry("stolen-too", "outside/marker")},
		{symlinkEntry("outside/link", "/"), fileEntry("outside/link/escape", "x", 0o644)},
		{fileEntry(".wh.outside", "", 0o644), fileEntry("outside/.wh.marker", "", 0o644)},
		{fileEntry("outside/.wh..wh..opq", "", 0o644), fileEntry("up/.wh..wh..opq", "", 0o644)},
		{fifoEntry("up/pipe", 0o600), deviceEntry("dev/null", tar.TypeChar), deviceEntry("outside/null", tar.TypeChar)},
		{folderEntry("./", 0o700), fileEntry("up/../escape", "x", 0o644)},
		{},
	}
	for _, seed := range seeds {
		f.Add(layerTar(f, seed...))
	}
	f.Add([]byte("not an archive"))

	f.Fuzz(func(t *testing.T, archive []byte) {
		store := newAppliedStore(t, applyLimits{expandedBytesMax: 1 << 20, entriesMax: 256})
		t.Cleanup(func() { restoreAccess(t, store.stagedPath) })
		marker := store.writeMarker(t, "marker", "marker\n")
		store.mustApply(t,
			symlinkEntry("outside", store.dataDir),
			symlinkEntry("up", "../../../../../../../.."),
			symlinkEntry("marker", marker),
			folderEntry("dev", 0o755),
		)
		before := store.outsideSnapshot(t)
		err := store.applier.apply(bytes.NewReader(archive), registry.CompressionNone, diffDigestOf(t, archive))
		if err != nil {
			t.Logf("the applier answered: %v", err)
		}
		store.requireOutsideUnchanged(t, before)
	})
}

// restoreAccess opens every folder under the staged folder to its owner, so
// the test's temporary folder can be removed after an archive gave a folder
// a mode its owner cannot enter. Only folders are changed: a chmod of a
// symbolic link would change what it points to.
func restoreAccess(t *testing.T, stagedPath string) {
	t.Helper()
	err := filepath.WalkDir(stagedPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		return os.Chmod(path, 0o700) //nolint:gosec // G302: a test's own folder, opened to its owner only
	})
	if err != nil {
		t.Errorf("open the staged folder for removal: %v", err)
	}
}
