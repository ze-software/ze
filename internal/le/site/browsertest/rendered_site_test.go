// Design: docs/contributing/gh-pages.md -- browser fixtures render current producers over recorded prerequisites

package browsertest

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/le/site"
	siteterminaldemo "github.com/ze-software/ze/internal/le/site/terminaldemo"
)

// renderedBrowserSite supplies the current scripts, stylesheet, fonts and
// header. Only recorded facts come from the archived publication; no published
// HTML or development-launcher session is needed to render the page under test.
func renderedBrowserSite(t *testing.T) site.Paths {
	t.Helper()
	repository, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	paths := site.Paths{Repository: repository, Source: filepath.Join(repository, "website"), Output: t.TempDir()}
	copyBrowserInput(t, filepath.Join(recordedBrowserSite(repository), "data", "site-facts.json"),
		filepath.Join(paths.Output, "data", "site-facts.json"))
	for _, asset := range []string{"ze.svg", "vendor/asciinema-player.min.js", "vendor/asciinema-player.css"} {
		copyBrowserInput(t, filepath.Join(paths.Source, "assets", filepath.FromSlash(asset)),
			filepath.Join(paths.Output, "assets", filepath.FromSlash(asset)))
	}
	fonts := filepath.Join(paths.Source, "assets", "vendor", "fonts")
	if err := filepath.WalkDir(fonts, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(fonts, path)
		if err != nil {
			return err
		}
		copyBrowserInput(t, path, filepath.Join(paths.Output, "assets", "vendor", "fonts", relative))
		return nil
	}); err != nil {
		t.Fatalf("stage the browser's real fonts: %v", err)
	}
	if err := site.RenderAssets(paths); err != nil {
		t.Fatalf("render the current browser assets: %v", err)
	}
	renderBrowserProducer(t, paths, "header")
	return paths
}

func recordedBrowserSite(repository string) string {
	return filepath.Join(filepath.Dir(repository), "gh-pages")
}

func renderBrowserProducer(t *testing.T, paths site.Paths, name string) {
	t.Helper()
	if _, err := site.RenderProducer(paths, name); err != nil {
		t.Fatalf("render the current %s producer: %v", name, err)
	}
}

func copyBrowserInput(t *testing.T, source, destination string) {
	t.Helper()
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read required browser input %s: %v", source, err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, content, 0o644); err != nil {
		t.Fatalf("stage required browser input %s: %v", destination, err)
	}
}

// stageHomeRecordings retains genuine media and verifies every copied payload
// against its recorded digest. The fixture carries only the three demos the
// opening and management panels require, not the whole media archive.
func stageHomeRecordings(t *testing.T, paths site.Paths) {
	t.Helper()
	recorded := recordedBrowserSite(paths.Repository)
	media := filepath.Join(recorded, "assets", "demos")
	_, entries, err := siteterminaldemo.Published(paths.Repository, recorded, media)
	if err != nil {
		t.Fatalf("read required recorded demonstrations: %v", err)
	}
	source, err := siteterminaldemo.OpenMediaRoot(recorded, media, false)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close() //nolint:errcheck // Read-only fixture acquisition has no pending writes.
	destination, err := siteterminaldemo.OpenMediaRoot(paths.Output, filepath.Join(paths.Output, "assets", "demos"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close() //nolint:errcheck // Every publication completes before this descriptor closes.
	copyBrowserInput(t, filepath.Join(media, "manifest.json"), filepath.Join(paths.Output, "assets", "demos", "manifest.json"))
	for _, id := range []string{siteterminaldemo.HeroDemo, "zefs-config", "web-config"} {
		entry, found := entries[id]
		if !found {
			t.Fatalf("required genuine demonstration %s has no recorded assets", id)
		}
		for _, metadata := range entry.Assets {
			asset, err := siteterminaldemo.ReadAsset(source, metadata)
			if err != nil {
				t.Fatalf("verify required demonstration %s: %v", id, err)
			}
			if err := asset.Publish(destination, metadata.Path); err != nil {
				t.Fatalf("stage required demonstration %s: %v", id, err)
			}
		}
	}
}
