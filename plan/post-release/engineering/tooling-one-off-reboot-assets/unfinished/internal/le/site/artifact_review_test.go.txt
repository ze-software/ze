// Design: docs/contributing/gh-pages.md -- destination identity and served media identity remain bound.
package site

import (
	"os"
	"path/filepath"
	"testing"
)

// TestArtifactSeedRefusesDestinationParentAliases exercises the seed entry with
// real directory links. Refusal must precede every overlay write, preserving Git
// metadata, prior content and the destination link itself.
func TestArtifactSeedRefusesDestinationParentAliases(t *testing.T) {
	for _, alias := range []string{".git", "old-assets"} {
		t.Run(alias, func(t *testing.T) {
			source := t.TempDir()
			target := t.TempDir()
			for _, directory := range []string{filepath.Join(source, "assets"), filepath.Join(target, alias)} {
				if err := os.MkdirAll(directory, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			for path, content := range map[string]string{
				filepath.Join(source, "000-first"):        "new early file",
				filepath.Join(source, "assets", "config"): "new aliased file",
				filepath.Join(target, "000-first"):        "old early file",
				filepath.Join(target, alias, "config"):    "old aliased file",
			} {
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(alias, filepath.Join(target, "assets")); err != nil {
				t.Fatal(err)
			}
			if err := seedArtifact(Paths{Output: target}, source); err == nil {
				t.Error("seed accepted a destination-parent symlink")
			}
			for path, want := range map[string]string{
				filepath.Join(target, "000-first"):     "old early file",
				filepath.Join(target, alias, "config"): "old aliased file",
			} {
				content, err := os.ReadFile(path)
				if err != nil {
					t.Errorf("seed removed prior content %s: %v", path, err)
					continue
				}
				if string(content) != want {
					t.Errorf("seed changed %s before refusing: %q", path, content)
				}
			}
			link, err := os.Readlink(filepath.Join(target, "assets"))
			if err != nil {
				t.Fatal(err)
			}
			if link != alias {
				t.Fatalf("seed changed destination link: %q", link)
			}
		})
	}
}

// TestDemoPageRefusesManifestPathDifferentFromServedURL supplies valid recorded
// bytes under another filename. The page must refuse before replacing that file
// instead of serving launcher.cast under the other file's verified digest.
func TestDemoPageRefusesManifestPathDifferentFromServedURL(t *testing.T) {
	catalog := newDemoCatalog(demoFixture(t))
	if err := catalog.load(); err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(catalog.assetRoot(), "launcher.cast")
	other := filepath.Join(catalog.assetRoot(), "other.cast")
	if err := os.Rename(canonical, other); err != nil {
		t.Fatal(err)
	}
	const served = "unverified served bytes\n"
	if err := os.WriteFile(canonical, []byte(served), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(other)
	if err != nil {
		t.Fatal(err)
	}
	entry := catalog.built["launcher"]
	metadata := entry.Assets[demoCast]
	metadata.Path = "other.cast"
	entry.Assets[demoCast] = metadata
	catalog.built["launcher"] = entry
	if _, _, _, err := catalog.expand("<!-- terminal-demo: launcher -->", "<!-- terminal-demo: launcher -->", "../../", "guide/quickstart.md"); err == nil {
		t.Error("page verified other.cast but linked launcher.cast")
	}
	after, err := os.Stat(other)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Error("refused metadata path was republished before its identity was checked")
	}
	content, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != served {
		t.Fatalf("refusal changed the canonical file: %q", content)
	}
}
