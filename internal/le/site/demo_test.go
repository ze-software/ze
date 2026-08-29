// Design: website/AI.md -- a recorded demonstration publishes only from verified media
package site

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// demoFixture lays out one checkout that holds a single recorded
// demonstration: the checked-in definition under demos/terminal, and the
// generated media with its own manifest under website/assets/demos.
//
// The cast is a real asciicast v2 file rather than a stub, because the
// renderer reads the grid and the running time out of it and reserves the
// player's box from both.
func demoFixture(t *testing.T) Paths {
	t.Helper()
	root := t.TempDir()
	media := filepath.Join(root, "website", "assets", "demos")
	if err := os.MkdirAll(media, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "demos", "terminal"), 0o755); err != nil {
		t.Fatal(err)
	}
	const cast = `{"version":2,"width":100,"height":25}` + "\n" +
		`[0.5,"o","ze> "]` + "\n" +
		`[7.25,"o","done\r\n"]` + "\n"
	const transcript = "ze> show bgp summary\nPeer 192.0.2.1 established\n"
	writeDemoAsset(t, media, "launcher.cast", cast)
	writeDemoAsset(t, media, "launcher.txt", transcript)

	writeDemoJSON(t, filepath.Join(root, "demos", "terminal", "manifest.json"), map[string]any{
		"schema":       2,
		"renderer":     map[string]any{"name": "ze-demo", "version": "3", "image": "img", "platform": "linux/native"},
		"gallery-page": "guide/terminal-demonstrations.md",
		"demos": []any{map[string]any{
			"id": "launcher", "title": "Start Ze", "description": "Bring a session up.",
			"page": "guide/quickstart.md", "anchor": "start", "platform": "portable",
			"kind": "terminal", "engine": "ze-demo", "source": "tape", "validate": "check",
		}},
	})
	writeDemoJSON(t, filepath.Join(media, "manifest.json"), map[string]any{
		"schema":   2,
		"renderer": map[string]any{"name": "ze-demo", "version": "3", "image": "img", "platform": "linux/native"},
		"demos": map[string]any{"launcher": map[string]any{
			"release": "26.08.27",
			"assets": map[string]any{
				"cast":       demoAssetRecord(t, media, "launcher.cast"),
				"transcript": demoAssetRecord(t, media, "launcher.txt"),
			},
		}},
	})
	return Paths{Repository: root, Source: filepath.Join(root, "website"), Output: t.TempDir()}
}

func writeDemoAsset(t *testing.T, media, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(media, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeDemoJSON(t *testing.T, path string, value any) {
	t.Helper()
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// demoAssetRecord answers the manifest record of one generated file: its path,
// its size and its digest, all read from the file itself.
func demoAssetRecord(t *testing.T, media, name string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(media, name))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	return map[string]any{"path": name, "bytes": len(content), "sha256": hex.EncodeToString(digest[:])}
}

// TestATerminalDemoMarkerBecomesAPlayer covers the whole expansion: the page
// gets the player and the transcript, the mirror gets the same facts as
// Markdown, and the head links the player only for a page that carries one.
func TestATerminalDemoMarkerBecomesAPlayer(t *testing.T) {
	catalog := newDemoCatalog(demoFixture(t))
	body, mirror, head, err := catalog.expand(
		"<p>Before.</p>\n<!-- terminal-demo: launcher -->\n<p>After.</p>",
		"Before.\n\n<!-- terminal-demo: launcher -->\n\nAfter.",
		"../../", "guide/quickstart.md")
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	for _, part := range []string{
		`<figure class="terminal-demo" data-terminal-demo="launcher">`,
		`data-cols="100"`,
		`data-rows="25"`,
		`style="--demo-aspect: 60 / 33"`,
		"<h3>Start Ze</h3>",
		"<span>Ze 26.08.27</span><span>7 seconds</span><span>macOS and Linux</span><span>Terminal</span>",
		"Peer 192.0.2.1 established",
	} {
		if !strings.Contains(body, part) {
			t.Errorf("the published page must carry %s:\n%s", part, body)
		}
	}
	if !strings.Contains(body, "<p>Before.</p>") || !strings.Contains(body, "<p>After.</p>") {
		t.Errorf("the page around the marker must survive:\n%s", body)
	}
	for _, part := range []string{
		"### Demo: Start Ze",
		"[Download the asciicast recording](../../assets/demos/launcher.cast?v=",
		"Recorded with Ze 26.08.27 on macOS and Linux using ze-demo. Duration: 7 seconds.",
		"```console",
	} {
		if !strings.Contains(mirror, part) {
			t.Errorf("the Markdown mirror must carry %s:\n%s", part, mirror)
		}
	}
	if !strings.Contains(head, demoPlayerScript) || !strings.Contains(head, demoPlayerStylesheet) {
		t.Errorf("a page carrying an asciicast must link the player: %q", head)
	}
}

// TestAPageWithNoMarkerLinksNoPlayer keeps the other seven hundred published
// pages from downloading a player they never use, and keeps a checkout with no
// rendered media from failing on a page that shows none.
func TestAPageWithNoMarkerLinksNoPlayer(t *testing.T) {
	catalog := newDemoCatalog(Paths{Repository: t.TempDir(), Source: t.TempDir()})
	body, mirror, head, err := catalog.expand("<p>Nothing here.</p>", "Nothing here.", "../", "guide/rpki.md")
	if err != nil {
		t.Fatalf("a page with no marker must not read a manifest: %v", err)
	}
	if head != "" || body != "<p>Nothing here.</p>" || mirror != "Nothing here." {
		t.Errorf("a page with no marker is left alone: %q %q %q", body, mirror, head)
	}
}

// TestATerminalDemoIsRefusedRatherThanPublishedWrong covers the four refusals.
// Each one publishes a page that lies to a reader if it is not made: media
// that no longer matches its manifest, an id nobody recorded, a demonstration
// copied onto a page it does not belong to, and a checkout with no media at
// all.
func TestATerminalDemoIsRefusedRatherThanPublishedWrong(t *testing.T) {
	// The two tampers are separate because they are caught by different
	// checks. A recording edited in place keeps its length, so only the digest
	// sees it; a recording of another length is caught before the digest is
	// read. Testing the second alone would leave the digest check unproven.
	tampers := map[string]string{
		"media edited in place, at the same length": `{"version":2,"width":100,"height":25}` + "\n" +
			`[0.5,"o","ze> "]` + "\n" + `[7.25,"o","gone\r\n"]` + "\n",
		"media of another length": `{"version":2,"width":100,"height":25}` + "\n" + `[9.0,"o","x"]` + "\n",
	}
	for name, tampered := range tampers {
		t.Run(name, func(t *testing.T) {
			paths := demoFixture(t)
			writeDemoAsset(t, filepath.Join(paths.Source, "assets", "demos"), "launcher.cast", tampered)
			_, _, _, err := newDemoCatalog(paths).expand(
				"<!-- terminal-demo: launcher -->", "<!-- terminal-demo: launcher -->", "../", "guide/quickstart.md")
			if err == nil || !strings.Contains(err.Error(), "launcher") {
				t.Errorf("media that no longer matches its manifest must be refused by name: %v", err)
			}
		})
	}

	t.Run("an id nobody recorded", func(t *testing.T) {
		_, _, _, err := newDemoCatalog(demoFixture(t)).expand(
			"<!-- terminal-demo: invented -->", "<!-- terminal-demo: invented -->", "../", "guide/quickstart.md")
		if err == nil || !strings.Contains(err.Error(), "invented") {
			t.Errorf("a marker naming no recording must be refused by name: %v", err)
		}
	})

	t.Run("a demonstration on a page it does not belong to", func(t *testing.T) {
		_, _, _, err := newDemoCatalog(demoFixture(t)).expand(
			"<!-- terminal-demo: launcher -->", "<!-- terminal-demo: launcher -->", "../", "guide/rpki.md")
		if err == nil || !strings.Contains(err.Error(), "guide/quickstart.md") {
			t.Errorf("a demonstration copied onto another page must name where it belongs: %v", err)
		}
	})

	t.Run("a checkout with no rendered media", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "website"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, _, _, err := newDemoCatalog(Paths{Repository: root, Source: filepath.Join(root, "website")}).expand(
			"<!-- terminal-demo: launcher -->", "<!-- terminal-demo: launcher -->", "../", "guide/quickstart.md")
		if !errors.Is(err, errDemoMediaAbsent) {
			t.Errorf("a checkout with no media must say which command writes it: %v", err)
		}
	})
}

// TestARunningTimeReadsAsAPhrase covers the duration a caption shows, which is
// read from the recording rather than restated in the catalog.
func TestARunningTimeReadsAsAPhrase(t *testing.T) {
	phrases := map[float64]string{
		0: "0 seconds", 1: "1 second", 1.4: "1 second", 59: "59 seconds",
		60: "1 minute", 61: "1 minute 1 second", 125: "2 minutes 5 seconds",
	}
	for seconds, want := range phrases {
		if got := durationPhrase(seconds); got != want {
			t.Errorf("%v seconds reads as %q, want %q", seconds, got, want)
		}
	}
}

// TestTheFixtureCastParsesAsAsciicastV2 keeps the fixture honest: the cast the
// test writes must be a recording the reader parses, or every assertion above
// it is about a file nothing else would accept.
func TestTheFixtureCastParsesAsAsciicastV2(t *testing.T) {
	paths := demoFixture(t)
	facts, err := readCastFacts(filepath.Join(paths.Source, "assets", "demos", "launcher.cast"))
	if err != nil {
		t.Fatalf("read cast: %v", err)
	}
	if facts.Columns != 100 || facts.Rows != 25 || facts.Seconds != 7.25 {
		t.Errorf("the recording states %dx%d over %v seconds", facts.Columns, facts.Rows, facts.Seconds)
	}
	if _, err := readCastFacts(filepath.Join(paths.Source, "assets", "demos", "launcher.txt")); err == nil {
		t.Error("a file that is not an asciicast must be refused")
	}
}
