// Design: website/AI.md -- the README animation derives from the website's verified recording.
package site

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// VALIDATES: the full-build producer registry reaches a real animated README
// asset made from the published dashboard recording, without claiming a page.
func TestReadmeDemoRegisteredForEveryBuild(t *testing.T) {
	producer := readmeDemoProducer(t)
	paths := readmeDemoFixture(t)
	routes, err := producer.Render(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatalf("README asset claimed website routes: %v", routes)
	}
	content := readArtifact(t, paths.Repository, "docs/demos/cli-dashboard.svg")
	for _, want := range []string{"<svg", "@keyframes", "established"} {
		if !strings.Contains(content, want) {
			t.Errorf("README animation does not contain %q", want)
		}
	}
}

// VALIDATES: rebuilding consumes changed recording bytes while an unchanged
// recording leaves the existing README asset and its modification time alone.
func TestReadmeDemoFollowsPublishedCastChanges(t *testing.T) {
	producer := readmeDemoProducer(t)
	paths := readmeDemoFixture(t)
	if _, err := producer.Render(paths); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(paths.Repository, "docs", "demos", "cli-dashboard.svg")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := producer.Render(paths); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(stamp) {
		t.Fatal("unchanged animation was rewritten")
	}
	writeReadmeDemoRecording(t, paths, "idle")
	if _, err := producer.Render(paths); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("changed website recording left the README animation stale")
	}
	if !bytes.Contains(second, []byte("idle")) {
		t.Fatal("the new animation does not show the new recorded state")
	}
}

// VALIDATES: invalid media is a failed build and never replaces an existing
// README animation with a partial output or silently reports stale success.
func TestReadmeDemoInvalidMediaPreservesExistingAnimation(t *testing.T) {
	producer := readmeDemoProducer(t)
	for _, change := range []string{"missing", "digest", "malformed"} {
		t.Run(change, func(t *testing.T) {
			paths := readmeDemoFixture(t)
			if _, err := producer.Render(paths); err != nil {
				t.Fatal(err)
			}
			before := readArtifact(t, paths.Repository, "docs/demos/cli-dashboard.svg")
			media := filepath.Join(paths.Output, "assets", "demos")
			cast := filepath.Join(media, "cli-dashboard.cast")
			switch change {
			case "missing":
				if err := os.Remove(cast); err != nil {
					t.Fatal(err)
				}
			case "digest":
				writeDemoAsset(t, media, "cli-dashboard.cast", "changed without manifest")
			case "malformed":
				writeDemoAsset(t, media, "cli-dashboard.cast", "not a terminal recording")
				writeReadmeDemoManifest(t, media)
			}
			if _, err := producer.Render(paths); err == nil {
				t.Fatal("invalid website recording was accepted")
			}
			if got := readArtifact(t, paths.Repository, "docs/demos/cli-dashboard.svg"); got != before {
				t.Fatal("failed generation changed the existing README animation")
			}
		})
	}
}

// VALIDATES: neither a linked parent nor a linked destination can make a site
// build overwrite a file outside the source checkout.
func TestReadmeDemoRefusesEscapingDestination(t *testing.T) {
	producer := readmeDemoProducer(t)
	for _, link := range []string{"docs", "docs/demos", "docs/demos/cli-dashboard.svg"} {
		t.Run(link, func(t *testing.T) {
			paths := readmeDemoFixture(t)
			outside := t.TempDir()
			outsideFile := filepath.Join(outside, "keep.svg")
			if err := os.WriteFile(outsideFile, []byte("unrelated"), 0o600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(paths.Repository, filepath.FromSlash(link))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			target := outside
			if strings.HasSuffix(link, ".svg") {
				target = outsideFile
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			if _, err := producer.Render(paths); err == nil {
				t.Fatal("escaping README destination was accepted")
			}
			if got := readArtifact(t, outside, "keep.svg"); got != "unrelated" {
				t.Fatal("generation changed the outside file")
			}
		})
	}
}

// VALIDATES: a partial website build refreshes the README even when it names
// another producer, and explicitly naming the README does not run it twice.
func TestReadmeDemoRefreshesDuringOtherProducerBuild(t *testing.T) {
	producer := readmeDemoProducer(t)
	runs := 0
	render := producer.Render
	producer.Render = func(paths Paths) ([]string, error) {
		runs++
		return render(paths)
	}
	stubProducers(t, Producer{Name: "guides", Render: func(paths Paths) ([]string, error) {
		writeFooterPage(t, paths.Output, "guides", "rendered guides", producerRunNow)
		return []string{"/guides/"}, nil
	}})
	registerDerivedProducer(producer)
	root, output := fullArtifactFixture(t, []Claim{{Route: "/guides/", Producers: []string{"guides"}}})
	paths := Paths{Repository: root, Source: filepath.Join(root, "website"), Output: output}
	installReadmeDemo(t, paths)
	for _, names := range [][]string{{"guides"}, {"readme-demo", "guides"}} {
		report, err := buildProducers(producerBuildOptions{Repository: root, Output: output, Producers: names})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(report.Producers, "readme-demo") {
			t.Fatal("website build did not report its automatic README refresh")
		}
	}
	if runs != 2 {
		t.Fatalf("two website builds refreshed README %d times", runs)
	}
	if got := readArtifact(t, root, "docs/demos/cli-dashboard.svg"); !strings.Contains(got, "established") {
		t.Fatal("website build did not publish the recorded dashboard in README")
	}
}

func readmeDemoProducer(t *testing.T) Producer {
	t.Helper()
	for _, producer := range allProducers() {
		if producer.Name == "readme-demo" {
			return producer
		}
	}
	t.Fatal("website build has no registered readme-demo producer")
	return Producer{}
}

func readmeDemoFixture(t *testing.T) Paths {
	t.Helper()
	paths := Paths{Repository: t.TempDir(), Output: t.TempDir()}
	paths.Source = filepath.Join(paths.Repository, "website")
	installReadmeDemo(t, paths)
	return paths
}

func installReadmeDemo(t *testing.T, paths Paths) {
	t.Helper()
	definitions := filepath.Join(paths.Repository, "demos", "terminal")
	if err := os.MkdirAll(definitions, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(paths.Output, "assets", "demos"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeDemoJSON(t, filepath.Join(definitions, "manifest.json"), map[string]any{
		"schema":       2,
		"renderer":     map[string]any{"name": "ze-demo", "version": "3", "image": "img", "platform": "linux/native"},
		"gallery-page": "guide/terminal-demonstrations.md",
		"demos": []any{map[string]any{
			"id": "cli-dashboard", "title": "Dashboard", "description": "Watch BGP peers.",
			"page": "guide/cli.md", "anchor": "live-bgp-dashboard", "platform": "portable",
			"kind": "terminal", "engine": "ze-demo", "source": "tape", "validate": "check",
		}},
	})
	writeReadmeDemoRecording(t, paths, "established")
}

func writeReadmeDemoRecording(t *testing.T, paths Paths, state string) {
	t.Helper()
	media := filepath.Join(paths.Output, "assets", "demos")
	cast := `{"version":2,"width":40,"height":5}` + "\n" +
		`[0,"o","ze> monitor bgp\r\n"]` + "\n" +
		`[1,"o","192.0.2.1 ` + state + `\r\n"]` + "\n" +
		`[3,"o","\r\n"]` + "\n"
	writeDemoAsset(t, media, "cli-dashboard.cast", cast)
	writeDemoAsset(t, media, "cli-dashboard.txt", "ze> monitor bgp\n192.0.2.1 "+state+"\n")
	writeReadmeDemoManifest(t, media)
}

func writeReadmeDemoManifest(t *testing.T, media string) {
	t.Helper()
	writeDemoJSON(t, filepath.Join(media, "manifest.json"), map[string]any{
		"schema":   2,
		"renderer": map[string]any{"name": "ze-demo", "version": "3", "image": "img", "platform": "linux/native"},
		"demos": map[string]any{"cli-dashboard": map[string]any{
			"release": "26.10.09",
			"assets": map[string]any{
				"cast":       demoAssetRecord(t, media, "cli-dashboard.cast"),
				"transcript": demoAssetRecord(t, media, "cli-dashboard.txt"),
			},
		}},
	})
}
