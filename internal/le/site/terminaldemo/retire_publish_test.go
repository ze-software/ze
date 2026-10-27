// Design: docs/contributing/gh-pages.md -- retirement runs after publication media seeding.
package siteterminaldemo

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRetireDeletedPublishesOnlyRemovedManifestIDs exercises the public build hook without a renderer.
func TestRetireDeletedPublishesOnlyRemovedManifestIDs(t *testing.T) {
	root := t.TempDir()
	media := filepath.Join(root, "media")
	if err := os.MkdirAll(filepath.Join(root, "demos", "terminal"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(media, 0o700); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(root, "demos", "terminal", "manifest.json"), Manifest{Schema: manifestSchema, Demos: []Demo{{ID: "live"}}})
	old := ArtifactEntry{Assets: map[string]AssetMetadata{"cast": {Path: "old.cast"}}}
	live := ArtifactEntry{Release: "retained-release", Assets: map[string]AssetMetadata{"cast": {Path: "live.cast"}}}
	writeJSON(t, filepath.Join(media, "manifest.json"), artifactManifest{Schema: manifestSchema, Demos: map[string]ArtifactEntry{"old": old, "live": live}})
	for _, name := range []string{"old.cast", "live.cast", "foreign.webm"} {
		if err := os.WriteFile(filepath.Join(media, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	engine := New(Options{Root: root, ArtifactRoot: media})
	if err := engine.RetireDeleted(); err != nil {
		t.Fatal(err)
	}
	var generated artifactManifest
	if err := readJSON(media, &generated); err != nil {
		t.Fatal(err)
	}
	if len(generated.Demos) != 1 || generated.Demos["live"].Release != live.Release {
		t.Fatalf("retirement changed retained entry: %#v", generated.Demos)
	}
	if _, err := os.Stat(filepath.Join(media, "old.cast")); !os.IsNotExist(err) {
		t.Fatalf("removed recording still exists: %v", err)
	}
	for _, name := range []string{"live.cast", "foreign.webm"} {
		if _, err := os.Stat(filepath.Join(media, name)); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRetireDeletedRefusesSharedOwnership prevents a corrupted inventory from deleting live media.
func TestRetireDeletedRefusesSharedOwnership(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "old.cast"), []byte("shared"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := ArtifactEntry{Assets: map[string]AssetMetadata{"cast": {Path: "old.cast"}}}
	generated := artifactManifest{Demos: map[string]ArtifactEntry{"old": entry, "live": entry}}
	engine := New(Options{Root: root, ArtifactRoot: root})
	if err := engine.retireDeletedAssets(&generated, map[string]Demo{"live": {ID: "live"}}); err == nil {
		t.Fatal("shared ownership accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "old.cast")); err != nil {
		t.Fatal("shared media was deleted:", err)
	}
}
