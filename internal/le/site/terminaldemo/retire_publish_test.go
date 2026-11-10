// Design: docs/contributing/gh-pages.md -- retirement runs after publication media seeding.
package siteterminaldemo

import (
	"os"
	"path/filepath"
	"strings"
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

// TestRetireDeletedStaysInsideItsAnchor runs a site build's retirement with
// the media anchored at the site output. Its control is a real assets/demos
// directory, whose retired recording is removed. When assets/demos is a link
// to an outside copy, retirement is refused ("path escapes from parent") and
// the outside recording and inventory are untouched. MUTATION: replacing the
// anchored open in OpenMediaRoot with os.OpenRoot(media) follows the link and
// deletes outside old.cast.
func TestRetireDeletedStaysInsideItsAnchor(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(map[bool]string{false: "directory", true: "media-link"}[linked], func(t *testing.T) {
			root := t.TempDir()
			output := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "demos", "terminal"), 0o700); err != nil {
				t.Fatal(err)
			}
			writeJSON(t, filepath.Join(root, "demos", "terminal", "manifest.json"), Manifest{Schema: manifestSchema, Demos: []Demo{{ID: "live"}}})
			media := filepath.Join(output, "assets", "demos")
			holder := media
			if linked {
				holder = filepath.Join(t.TempDir(), "demos")
			}
			if err := os.MkdirAll(holder, 0o700); err != nil {
				t.Fatal(err)
			}
			old := ArtifactEntry{Assets: map[string]AssetMetadata{"cast": {Path: "old.cast"}}}
			live := ArtifactEntry{Assets: map[string]AssetMetadata{"cast": {Path: "live.cast"}}}
			writeJSON(t, filepath.Join(holder, "manifest.json"), artifactManifest{Schema: manifestSchema, Demos: map[string]ArtifactEntry{"old": old, "live": live}})
			for _, name := range []string{"old.cast", "live.cast"} {
				if err := os.WriteFile(filepath.Join(holder, name), []byte(name), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if linked {
				if err := os.MkdirAll(filepath.Dir(media), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(holder, media); err != nil {
					t.Fatal(err)
				}
			}
			engine := New(Options{Root: root, ArtifactRoot: media, ArtifactAnchor: output})
			err := engine.RetireDeleted()
			_, statErr := os.Stat(filepath.Join(holder, "old.cast"))
			if !linked {
				if err != nil {
					t.Fatalf("control: anchored retirement refused a real media directory: %v", err)
				}
				if !os.IsNotExist(statErr) {
					t.Fatalf("control: retired recording still exists: %v", statErr)
				}
				return
			}
			if err == nil {
				t.Fatal("retirement followed a media link out of its anchor")
			}
			if !strings.Contains(err.Error(), "path escapes from parent") {
				t.Fatalf("refusal %q does not name the escaping link", err)
			}
			if statErr != nil {
				t.Fatalf("retirement removed media outside its anchor: %v", statErr)
			}
			var generated artifactManifest
			if err := readJSON(holder, &generated); err != nil {
				t.Fatal(err)
			}
			if len(generated.Demos) != 2 {
				t.Fatalf("retirement rewrote the outside inventory: %#v", generated.Demos)
			}
		})
	}
}
