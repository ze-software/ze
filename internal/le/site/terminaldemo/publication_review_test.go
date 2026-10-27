// Design: docs/contributing/gh-pages.md -- publication refuses unproven media and ownership.
package siteterminaldemo

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRetirementPreflightsRetainedAliases rejects uncertain ownership before any file changes.
func TestRetirementPreflightsRetainedAliases(t *testing.T) {
	for _, kind := range []string{"dot-path", "symlink", "directory-symlink", "missing", "escaping-symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			for _, name := range []string{"old.cast", "old.txt", "foreign.webm"} {
				mustWrite(t, filepath.Join(root, name), "keep "+name)
			}
			mustWrite(t, filepath.Join(outside, "outside.cast"), "outside")
			retained := "./old.cast"
			switch kind {
			case "dot-path":
			case "symlink":
				retained = "live.cast"
				if err := os.Symlink("old.cast", filepath.Join(root, retained)); err != nil {
					t.Fatal(err)
				}
			case "directory-symlink":
				retained = "alias/old.cast"
				if err := os.Symlink(".", filepath.Join(root, "alias")); err != nil {
					t.Fatal(err)
				}
			case "missing":
				retained = "missing.cast"
			case "escaping-symlink":
				retained = "live.cast"
				if err := os.Symlink(filepath.Join(outside, "outside.cast"), filepath.Join(root, retained)); err != nil {
					t.Fatal(err)
				}
			}
			generated := artifactManifest{Demos: map[string]ArtifactEntry{
				"old":  {Assets: map[string]AssetMetadata{"cast": {Path: "old.cast"}, "transcript": {Path: "old.txt"}}},
				"live": {Assets: map[string]AssetMetadata{"cast": {Path: retained}}},
			}}
			engine := New(Options{Root: root, ArtifactRoot: root})
			if err := engine.retireDeletedAssets(&generated, map[string]Demo{"live": {ID: "live"}}); err == nil {
				t.Fatal("uncertain retained ownership admitted")
			}
			if len(generated.Demos) != 2 {
				t.Fatal("inventory partially retired")
			}
			for _, name := range []string{"old.cast", "old.txt", "foreign.webm"} {
				data, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || string(data) != "keep "+name {
					t.Fatalf("%s changed: %q, %v", name, data, err)
				}
			}
			data, err := os.ReadFile(filepath.Join(outside, "outside.cast"))
			if err != nil || string(data) != "outside" {
				t.Fatalf("outside changed: %q, %v", data, err)
			}
		})
	}
}
