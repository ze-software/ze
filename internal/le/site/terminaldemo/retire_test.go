// Design: docs/contributing/gh-pages.md -- only retired recordings own deleted media.
package siteterminaldemo

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRetireDeletedAssetsPreservesUnownedFiles checks the manifest ownership boundary.
func TestRetireDeletedAssetsPreservesUnownedFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"old.cast", "old.txt", "old.png", "live.cast", "unrelated.png"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	generated := artifactManifest{Demos: map[string]ArtifactEntry{
		"old":  {Assets: map[string]AssetMetadata{"cast": {Path: "old.cast"}, "transcript": {Path: "old.txt"}}},
		"live": {Assets: map[string]AssetMetadata{"cast": {Path: "live.cast"}}},
	}}
	engine := New(Options{Root: root, ArtifactRoot: root})
	if err := engine.retireDeletedAssets(&generated, map[string]Demo{"live": {ID: "live"}}); err != nil {
		t.Fatal(err)
	}
	if _, exists := generated.Demos["old"]; exists {
		t.Fatal("retired entry retained")
	}
	for _, name := range []string{"old.cast", "old.txt"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("retired %s: %v", name, err)
		}
	}
	for _, name := range []string{"old.png", "live.cast", "unrelated.png"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("unowned %s removed: %v", name, err)
		}
	}
	if err := engine.retireDeletedAssets(&generated, map[string]Demo{"live": {ID: "live"}}); err != nil {
		t.Fatalf("second retirement: %v", err)
	}
}

// TestRetireDeletedAssetsRefusesUnsafeOwnership checks every path before deleting any file.
func TestRetireDeletedAssetsRefusesUnsafeOwnership(t *testing.T) {
	for _, unsafe := range []string{"../outside.cast", "/outside.cast", "live.cast", "nested/old.cast", "old.svg", ""} {
		t.Run(unsafe, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "old.txt"), []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			generated := artifactManifest{Demos: map[string]ArtifactEntry{"old": {Assets: map[string]AssetMetadata{
				"cast": {Path: unsafe}, "transcript": {Path: "old.txt"},
			}}}}
			engine := New(Options{Root: root, ArtifactRoot: root})
			if err := engine.retireDeletedAssets(&generated, map[string]Demo{}); err == nil {
				t.Fatal("unsafe declaration accepted")
			}
			if _, err := os.Stat(filepath.Join(root, "old.txt")); err != nil {
				t.Fatal("partially retired an invalid manifest:", err)
			}
		})
	}
}

// TestRetireDeletedAssetsUnlinksSymlinkWithoutFollowing protects files outside the artifact tree.
func TestRetireDeletedAssetsUnlinksSymlinkWithoutFollowing(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "keep.cast")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "old.cast")); err != nil {
		t.Fatal(err)
	}
	generated := artifactManifest{Demos: map[string]ArtifactEntry{"old": {Assets: map[string]AssetMetadata{"cast": {Path: "old.cast"}}}}}
	engine := New(Options{Root: root, ArtifactRoot: root})
	if err := engine.retireDeletedAssets(&generated, map[string]Demo{}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "keep" {
		t.Fatalf("outside file changed: %q, %v", got, err)
	}
}
