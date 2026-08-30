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
	stubLiveInputs(t, `[{"path":"show test","description":"Show rows","mode":"read-only"}]`)
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

// TestNoBuildStagePublishesTheCommandFixture pins the replacement for the
// absent-only guard AC-18 left standing on the command pages.
//
// The guard existed because docvalid rendered a contract FIXTURE, and commit
// 9f45348a7 published that fixture over 396 real pages: a bare doctype, a body
// and one definition list, 481 bytes where a 10KB page had been. Phase 4 gives
// the pages a producer, so the guard goes because something better replaced it.
// This test is what stops the fixture coming back: it runs a build with the
// page producers stubbed out and checks that NOTHING in the build's own stages
// wrote a command page.
func TestNoBuildStagePublishesTheCommandFixture(t *testing.T) {
	catalog := `[{"path":"show test","description":"Show rows","mode":"read-only"}]`
	stubLiveInputs(t, catalog)
	stubProducers(t)
	root, output := siteFixture(t)

	if _, err := Build(BuildOptions{Repository: root, Output: output}); err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, fixture := range []string{
		"reference/cli/index.html", "reference/cli/index.md",
		"reference/command-equivalents/index.html", llmsFile,
	} {
		if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(fixture))); !os.IsNotExist(err) {
			t.Errorf("a build stage published %s; only a registered producer may write it: %v", fixture, err)
		}
	}
	// The catalog is the one surface a build stage does own, so the check above
	// must not be passing because the whole command surface stopped working.
	if got := readArtifact(t, output, catalogFile); got != catalog {
		t.Errorf("the published command catalog is %q, want the live one", got)
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
