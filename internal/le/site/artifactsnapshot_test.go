package site

import (
	"os"
	"path/filepath"
	"testing"
)

// VALIDATES: the sweep removes a snapshot no build holds and leaves the one a
// live build holds, and a release removes the held one.
// PREVENTS: a killed ./le site build leaving a full copy of the published site
// beside it for ever, and a build removing the copy a concurrent build reads.
// METHOD: an abandoned snapshot (no hold, as after a kill), a live snapshot
// taken through takeArtifactSnapshot, and an unrelated sibling directory.
func TestRemoveAbandonedSnapshotsSparesHeldCopies(t *testing.T) {
	parent := t.TempDir()
	abandoned := filepath.Join(parent, artifactSnapshotPrefix+"killed")
	if err := os.MkdirAll(filepath.Join(abandoned, "blog"), 0o755); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(parent, "gh-pages")
	if err := os.MkdirAll(unrelated, 0o755); err != nil {
		t.Fatal(err)
	}
	live, err := takeArtifactSnapshot(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(live.path, "index.html"), []byte("live"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := removeAbandonedSnapshots(parent); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(abandoned); !os.IsNotExist(err) {
		t.Fatalf("abandoned snapshot %s survived the sweep: %v", abandoned, err)
	}
	if _, err := os.Stat(filepath.Join(live.path, "index.html")); err != nil {
		t.Fatalf("the sweep removed a snapshot a live build holds: %v", err)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("the sweep removed a directory without the snapshot prefix: %v", err)
	}

	live.release()
	if _, err := os.Stat(live.path); !os.IsNotExist(err) {
		t.Fatalf("release left the snapshot %s in place: %v", live.path, err)
	}
}
