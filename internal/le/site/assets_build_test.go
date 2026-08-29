package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAnAssetEditReachesTheArtifact checks AC-18: an edit to
// website/assets/css/site.css or website/assets/js/site.js is published.
//
// The method is the one a reader would use to notice the defect. The seed
// artifact carries an old bundle, the source carries a new one, and the build
// runs. renderCSS and renderJS used to run only when the output file was
// ABSENT, so the seeded bundle survived every build and no stylesheet edit ever
// reached a reader.
func TestAnAssetEditReachesTheArtifact(t *testing.T) {
	stubLiveCommandCatalog(t, `[{"path":"show test","description":"Show rows","mode":"read-only"}]`)
	stubProducers(t)
	root, output := siteFixture(t)

	writeSourceAsset(t, filepath.Join(root, "website"), filepath.Join("css", "site.css"), ".banner { color: rebeccapurple; }\n")
	writeSourceAsset(t, filepath.Join(root, "website"), filepath.Join("js", "site.js"), "window.zeEdited = true;\n")

	// The seed states what a previous build published, which is what the
	// absent-only guard used to keep.
	seed := filepath.Join(filepath.Dir(root), "gh-pages", "assets")
	if err := os.WriteFile(filepath.Join(seed, "site.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seed, "site.js"), []byte("window.zeStale = true;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(BuildOptions{Repository: root, Output: output}); err != nil {
		t.Fatalf("build: %v", err)
	}

	stylesheet := readArtifact(t, output, "assets/site.css")
	if !strings.Contains(stylesheet, "rebeccapurple") {
		t.Errorf("the published stylesheet must carry the source edit; got %q", stylesheet)
	}
	if strings.Contains(stylesheet, "body{}") {
		t.Errorf("the published stylesheet must not survive from the seed; got %q", stylesheet)
	}
	script := readArtifact(t, output, "assets/site.js")
	if !strings.Contains(script, "zeEdited") || strings.Contains(script, "zeStale") {
		t.Errorf("the published script must carry the source edit and not the seed; got %q", script)
	}
}

// TestTheCommandPageRendererKeepsItsAbsentOnlyGuard pins the other half of the
// asymmetry AC-18 creates.
//
// docvalid.RenderCommandSurfaces emits the contract FIXTURE the documentation
// drift check compares a published page against, not a publishable page.
// Removing ITS guard in commit 9f45348a7 overwrote 396 published pages with
// 481-byte fragments. So the guard stays until a producer writes those pages,
// and this test fails if somebody removes it while tidying up after AC-18.
func TestTheCommandPageRendererKeepsItsAbsentOnlyGuard(t *testing.T) {
	stubLiveCommandCatalog(t, `[{"path":"show test","description":"Show rows","mode":"read-only"}]`)
	stubProducers(t)
	root, output := siteFixture(t)

	page := filepath.Join(filepath.Dir(root), "gh-pages", "reference", "cli", "index.html")
	if err := os.MkdirAll(filepath.Dir(page), 0o755); err != nil {
		t.Fatal(err)
	}
	published := "<!doctype html>\n<html lang=\"en\">the published command page</html>\n"
	if err := os.WriteFile(page, []byte(published), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(BuildOptions{Repository: root, Output: output}); err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := readArtifact(t, output, "reference/cli/index.html"); got != published {
		t.Errorf("a published command page must survive a build until a producer writes it; got %q", got)
	}
}

// writeSourceAssets gives a fixture the two bundles every website source
// carries. A build renders both on every run, so a source without them is not a
// website and the build says so.
func writeSourceAssets(t *testing.T, source string) {
	t.Helper()
	writeSourceAsset(t, source, filepath.Join("css", "site.css"), "body { margin: 0; }\n")
	writeSourceAsset(t, source, filepath.Join("js", "site.js"), "// site\n")
}

// writeSourceAsset writes one file under assets/ in the fixture website source.
func writeSourceAsset(t *testing.T, source, name, content string) {
	t.Helper()
	path := filepath.Join(source, "assets", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readArtifact answers one file of a built artifact.
func readArtifact(t *testing.T, output, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}
