// Design: docs/contributing/gh-pages.md -- checked artifact paths name the media actually served.
package siteterminaldemo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestArtifactCheckRefusesManifestPathDifferentFromPublishedName drives the
// standalone checker with matching digest/size for other.cast but no term.cast.
// The local manifest is inconsistent; a successful digest check cannot repair it.
func TestArtifactCheckRefusesManifestPathDifferentFromPublishedName(t *testing.T) {
	fixture := newDemoFixture(t)
	engine := fixture.engine(&bytes.Buffer{})
	if _, err := engine.RenderAll("26.08.27"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(fixture.artifacts, "term.cast"), filepath.Join(fixture.artifacts, "other.cast")); err != nil {
		t.Fatal(err)
	}
	var generated artifactManifest
	if err := readJSON(fixture.artifacts, &generated); err != nil {
		t.Fatal(err)
	}
	entry := generated.Demos["term"]
	metadata := entry.Assets[castLabel]
	metadata.Path = "other.cast"
	entry.Assets[castLabel] = metadata
	generated.Demos["term"] = entry
	if err := engine.writeArtifactManifest(generated); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(fixture.artifacts, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.checkAll(""); err == nil {
		t.Error("artifact checker verified another filename instead of the published recording")
	}
	after, err := os.ReadFile(filepath.Join(fixture.artifacts, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("artifact checker rewrote the inconsistent manifest")
	}
	if _, err := os.Lstat(filepath.Join(fixture.artifacts, "term.cast")); !os.IsNotExist(err) {
		t.Fatalf("artifact checker manufactured the missing canonical recording: %v", err)
	}
}
