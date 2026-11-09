// Design: docs/contributing/gh-pages.md -- checked media and published bytes share one payload.
package siteterminaldemo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestArtifactCheckRefusesEscapingMedia exercises the actual check entry with
// matching manifest bytes reached through a link outside the artifact root.
func TestArtifactCheckRefusesEscapingMedia(t *testing.T) {
	fixture := newDemoFixture(t)
	engine := fixture.engine(&bytes.Buffer{})
	if _, err := engine.RenderAll("26.08.27"); err != nil {
		t.Fatal(err)
	}
	cast := filepath.Join(fixture.artifacts, "term.cast")
	outside := filepath.Join(t.TempDir(), "term.cast")
	if err := os.Rename(cast, outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, cast); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.checkAll(""); err == nil {
		t.Fatal("artifact check accepted media outside its root")
	}
}

// TestRejectedRenderKeepsPreviousPublication records a valid fixture, then
// rejects its next recording and observes every previously published file.
func TestRejectedRenderKeepsPreviousPublication(t *testing.T) {
	fixture := newDemoFixture(t)
	engine := fixture.engine(&bytes.Buffer{})
	if _, err := engine.RenderAll("26.08.27"); err != nil {
		t.Fatal(err)
	}
	previous := make(map[string][]byte)
	for _, name := range []string{"term.cast", "term.txt", "manifest.json"} {
		content, err := os.ReadFile(filepath.Join(fixture.artifacts, name))
		if err != nil {
			t.Fatal(err)
		}
		previous[name] = content
	}
	fixture.wrongTranscript = true
	if _, err := engine.RenderOne("term", "26.08.27"); err == nil {
		t.Fatal("render accepted a recording that does not show its transcript")
	}
	for name, want := range previous {
		content, err := os.ReadFile(filepath.Join(fixture.artifacts, name))
		if err != nil {
			t.Fatalf("failed render removed %s: %v", name, err)
		}
		if !bytes.Equal(content, want) {
			t.Errorf("failed render replaced valid %s", name)
		}
	}
}

// TestTranscriptCopyFailureKeepsPreviousPublication observes the old target
// after a real source read failure, rather than checking only the error.
func TestTranscriptCopyFailureKeepsPreviousPublication(t *testing.T) {
	target := filepath.Join(t.TempDir(), "transcript.txt")
	if err := os.WriteFile(target, []byte("valid transcript\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	asset, err := acquireAsset(filepath.Dir(source), filepath.Base(source), nil)
	if err == nil {
		err = copyFile(asset, target)
	}
	if err == nil {
		t.Fatal("copy accepted a directory as transcript bytes")
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "valid transcript\n" {
		t.Fatalf("failed copy replaced prior transcript: %q", content)
	}
}
