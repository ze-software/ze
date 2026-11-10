// Design: website/AI.md -- parsing and publication retain the exact verified artifact.
package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoArtifactRootRejectsEscapingLinks exercises the page entry against real
// media whose valid manifest bytes live outside the artifact root. Each case
// first proves the page accepts the untouched fixture, then plants one link
// and asserts the refusal names its cause:
//
//   - file: launcher.cast itself is a link to an outside copy. MUTATION:
//     replacing the rooted open in acquireAsset (root.Open) with
//     os.Open(filepath.Join(assetRoot, name)) follows the link, and the page
//     accepts the outside bytes.
//   - media-root: assets/demos, the media directory, is a link to an outside
//     copy of it. The refusal comes from the anchor at the site output, and no
//     outside file is replaced, so nothing is published outside. MUTATION:
//     replacing the anchor in openMedia with os.OpenRoot(catalog.assetRoot())
//     follows the directory link, reads the outside media and publishes back
//     into it.
//   - unbound-path: the manifest names a path through an escaping directory
//     link. It is refused before any acquisition because the path is not the
//     demo's published file name; it proves the name binding, not containment.
func TestDemoArtifactRootRejectsEscapingLinks(t *testing.T) {
	for _, tc := range []struct {
		name    string
		refusal string
	}{
		{name: "file", refusal: "path escapes from parent"},
		{name: "media-root", refusal: "path escapes from parent"},
		{name: "unbound-path", refusal: "does not name its published file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := demoFixture(t)
			catalog := newDemoCatalog(paths)
			if err := catalog.load(); err != nil {
				t.Fatal(err)
			}
			if _, err := catalog.heroMount("launcher"); err != nil {
				t.Fatalf("control: page refused its untouched media: %v", err)
			}
			outside := t.TempDir()
			original := filepath.Join(catalog.assetRoot(), "launcher.cast")
			var published map[string]os.FileInfo
			switch tc.name {
			case "file":
				content, err := os.ReadFile(original)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(outside, "launcher.cast"), content, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(original); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(outside, "launcher.cast"), original); err != nil {
					t.Fatal(err)
				}
			case "media-root":
				moved := filepath.Join(outside, "demos")
				if err := os.Rename(catalog.assetRoot(), moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(moved, catalog.assetRoot()); err != nil {
					t.Fatal(err)
				}
				published = outsideFiles(t, moved)
			case "unbound-path":
				content, err := os.ReadFile(original)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(outside, "launcher.cast"), content, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(catalog.assetRoot(), "escape")); err != nil {
					t.Fatal(err)
				}
				entry := catalog.built["launcher"]
				metadata := entry.Assets[demoCast]
				metadata.Path = "escape/launcher.cast"
				entry.Assets[demoCast] = metadata
				catalog.built["launcher"] = entry
			}
			_, err := catalog.heroMount("launcher")
			if err == nil {
				t.Fatal("page accepted media acquired outside its artifact root")
			}
			if !strings.Contains(err.Error(), tc.refusal) {
				t.Fatalf("refusal %q does not name %q", err, tc.refusal)
			}
			if published == nil {
				return
			}
			after := outsideFiles(t, filepath.Join(outside, "demos"))
			if len(after) != len(published) {
				t.Fatalf("outside media gained or lost files: %d before, %d after", len(published), len(after))
			}
			for name, before := range published {
				now, present := after[name]
				if !present {
					t.Fatalf("outside media lost %s", name)
				}
				if !os.SameFile(before, now) {
					t.Fatalf("outside media %s was replaced: a publication wrote outside the site output", name)
				}
			}
		})
	}
}

// outsideFiles answers each entry of dir by name, so a later call can tell a
// replaced file (a new inode from a publication's rename) from an untouched one.
func outsideFiles(t *testing.T, dir string) map[string]os.FileInfo {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]os.FileInfo, len(entries))
	for _, entry := range entries {
		info, err := os.Lstat(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = info
	}
	return files
}

// catalogMedia opens the catalog's anchored media root for one test.
func catalogMedia(t *testing.T, catalog *demoCatalog) *os.Root {
	t.Helper()
	media, err := catalog.openMedia()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = media.Close() }) //nolint:errcheck // Test cleanup cannot change the verdict.
	return media
}

// TestVerifiedDemoFactsSurviveSourceMutation changes the real recording after
// verification. Both a replacement inode and an in-place write must leave the
// parser reading the verified recording, not the subsequently changed path.
func TestVerifiedDemoFactsSurviveSourceMutation(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		t.Run(map[bool]string{false: "in-place", true: "path-swap"}[replacement], func(t *testing.T) {
			catalog := newDemoCatalog(demoFixture(t))
			if err := catalog.load(); err != nil {
				t.Fatal(err)
			}
			asset, err := catalog.verifyAsset(catalogMedia(t, catalog), "launcher", demoCast, catalog.built["launcher"])
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(catalog.assetRoot(), "launcher.cast")
			if replacement {
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(path, []byte("not a recording\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			facts, err := readCastFacts(asset)
			if err != nil {
				t.Fatalf("verified payload was replaced by mutable source: %v", err)
			}
			if facts.Columns != 100 || facts.Rows != 25 || facts.Seconds != 7.25 {
				t.Fatalf("parsed unverified facts: %+v", facts)
			}
		})
	}
}

// TestArtifactCopyFailurePreservesPublishedBytes makes acquisition fail using a
// directory, then observes the previously published file rather than an error alone.
func TestArtifactCopyFailurePreservesPublishedBytes(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "published")
	if err := os.WriteFile(target, []byte("previous valid publication"), 0o600); err != nil {
		t.Fatal(err)
	}
	sourceRoot := publicationTestRoot(t, filepath.Dir(source))
	targetRoot := publicationTestRoot(t, filepath.Dir(target))
	if err := copyPath(sourceRoot, filepath.Base(source), targetRoot, filepath.Base(target)); err == nil {
		t.Fatal("directory source was accepted as a file")
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed publication removed its valid predecessor: %v", err)
	}
	if string(content) != "previous valid publication" {
		t.Fatalf("failed publication changed predecessor: %q", content)
	}
}

// TestArtifactCopyDoesNotPublishLinks proves the staging entry refuses an
// escaping source symlink rather than reproducing a link in the public tree.
func TestArtifactCopyDoesNotPublishLinks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, source); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "published")
	sourceRoot := publicationTestRoot(t, root)
	targetRoot := publicationTestRoot(t, filepath.Dir(target))
	if err := copyPath(sourceRoot, filepath.Base(source), targetRoot, filepath.Base(target)); err == nil {
		t.Fatal("publisher reproduced an escaping symlink")
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("refused link left a published target: %v", err)
	}
}

func publicationTestRoot(t *testing.T, path string) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	return root
}

// TestArtifactCopyPublishesRootedLinkContent keeps valid in-root source links
// usable without reproducing a public symlink.
func TestArtifactCopyPublishesRootedLinkContent(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "content"), []byte("rooted content"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("content", filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}
	if err := stageSources(source, target, []string{"link"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(target, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("published a link instead of content: %s", info.Mode())
	}
	content, err := os.ReadFile(filepath.Join(target, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "rooted content" {
		t.Fatalf("published wrong linked content: %q", content)
	}
}

// TestArtifactReplacementFailurePreservesDestination forces rename to fail on
// a populated directory and observes the prior bytes behind that destination.
func TestArtifactReplacementFailurePreservesDestination(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "page"), []byte("new page"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(target, "page"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "page", "previous"), []byte("old page"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := stageSources(source, target, []string{"page"}); err == nil {
		t.Fatal("replacement of a populated directory unexpectedly succeeded")
	}
	content, err := os.ReadFile(filepath.Join(target, "page", "previous"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "old page" {
		t.Fatalf("failed replacement changed preceding bytes: %q", content)
	}
}
