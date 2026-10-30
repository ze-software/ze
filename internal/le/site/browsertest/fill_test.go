// Design: website/AI.md -- the facts snapshot: one file, filled into every marked element by site.js
//
// Package browsertest drives the published site script in a real browser.
// It lives apart from internal/le/site so that package's tests stay runnable
// on a host with no Chrome; this one refuses to pass without it.
package browsertest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// factsPath is the URL path of the snapshot the browser fetches.
const factsPath = "/data/site-facts.json"

// chromeCandidates are the Chrome or Chromium binaries the test accepts, the
// macOS application bundles first and then the names a Linux package installs.
var chromeCandidates = []string{
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
	"google-chrome",
	"google-chrome-stable",
	"chromium",
	"chromium-browser",
	"chrome",
	"headless-shell",
}

// findChrome answers the first Chrome binary the host carries. A host with
// none fails the test by name: a browser test that skips proves nothing, and
// its silence reads as a pass.
func findChrome(t *testing.T) string {
	t.Helper()
	for _, candidate := range chromeCandidates {
		if filepath.IsAbs(candidate) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	t.Fatalf("no Chrome or Chromium found (looked for %s): this test drives a real browser and does not skip.\n"+
		"Install one: macOS `brew install --cask google-chrome`, Debian or Ubuntu `apt install chromium`.",
		strings.Join(chromeCandidates, ", "))
	return ""
}

// fixtureHeader is the shared header fragment, shaped like the published
// assets/header.html: a header.site-header whose links carry the root
// placeholder site.js replaces, and one marked count.
const fixtureHeader = `
<header class="site-header">
    <nav class="nav" aria-label="Main navigation">
        <a class="brand" href="__ZE_SITE_ROOT__#top" aria-label="Ze home"><span>Ze</span></a>
        <span class="nav-count"><span data-ze-stat="cli_commands_display">fallback</span> commands</span>
    </nav>
</header>
`

// fixturePage is a page shaped like the generator's: the header mount with
// the root and fragment paths for the page's depth, marked figures in the
// body, and the site script loaded deferred from the site root.
func fixturePage(root string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>fixture</title></head>
<body>
<div id="site-header-mount" data-header-src="` + root + `assets/header.html" data-site-root="` + root + `"></div>
<main>
    <p><span id="body-commands" data-ze-stat="cli_commands_display">fallback</span> commands,
    <span id="body-proven" data-ze-stat="rfc.proven_display">fallback</span> requirements proven.</p>
    <div class="th-meter"><span id="body-meter" class="th-meter-fill" data-ze-stat-width="health.rfc_proof.value.percent" style="width:1%"></span></div>
</main>
<script src="` + root + `assets/site.js" defer></script>
</body>
</html>
`
}

// snapshot is one version of data/site-facts.json and the text every marked
// element must show once the page has read it.
type snapshot struct {
	commands string
	proven   string
	percent  string
}

func (s snapshot) json(t *testing.T) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"cli_commands_display": s.commands,
		"rfc":                  map[string]any{"proven_display": s.proven},
		"health": map[string]any{"rfc_proof": map[string]any{
			"value": map[string]any{"percent": s.percent},
		}},
	})
	if err != nil {
		t.Fatalf("encode the fixture snapshot: %v", err)
	}
	return body
}

// writeFixtureSite lays the fixture site out under dir: the real site script
// (website/assets/js/site.js, which the build publishes as assets/site.js
// with only trailing blanks trimmed), the header fragment, a page at the root
// and one two levels down, as the CLI reference sits.
func writeFixtureSite(t *testing.T, dir string) {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "website", "assets", "js", "site.js"))
	if err != nil {
		t.Fatalf("read the site script: %v", err)
	}
	files := map[string][]byte{
		"assets/site.js":           script,
		"assets/header.html":       []byte(fixtureHeader),
		"index.html":               []byte(fixturePage("")),
		"reference/cli/index.html": []byte(fixturePage("../../")),
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

// pageTexts is what a loaded page shows in its three marked elements, and
// the width of its one bar drawn from a percentage.
type pageTexts struct {
	Header       string `json:"header"`
	BodyCommands string `json:"bodyCommands"`
	BodyProven   string `json:"bodyProven"`
	MeterWidth   string `json:"meterWidth"`
}

const readTexts = `(function () {
    function text(selector) {
        var node = document.querySelector(selector);
        return node ? node.textContent : "<missing>";
    }
    return {
        header: text("header.site-header [data-ze-stat]"),
        bodyCommands: text("#body-commands"),
        bodyProven: text("#body-proven"),
        meterWidth: (function () {
            var bar = document.querySelector("#body-meter");
            return bar ? bar.style.width : "<missing>";
        })()
    };
})()`

// waitForTexts polls the page until its marked elements show want, and fails
// with what the page shows when they never do.
func waitForTexts(ctx context.Context, t *testing.T, page string, want snapshot) {
	t.Helper()
	expected := pageTexts{
		Header: want.commands, BodyCommands: want.commands, BodyProven: want.proven, MeterWidth: want.percent + "%",
	}

	deadline := time.Now().Add(10 * time.Second)
	var got pageTexts
	for time.Now().Before(deadline) {
		var err error
		got, err = chromedp.Run(ctx, chromedp.Evaluate[pageTexts](readTexts))
		if err != nil {
			t.Fatalf("%s: read the marked elements: %v", page, err)
		}
		if got == expected {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s: marked elements never showed the snapshot's values\n got: %+v\nwant: %+v", page, got, expected)
}

// TestTheBrowserFillsEveryMarkedValueFromTheSnapshot proves AC-15: a reader
// who loads a page sees the snapshot's values in every marked element, the
// shared header's included, and after data/site-facts.json changes a reload
// shows the new values, with the snapshot fetched once per load. A bar drawn
// from a percentage (data-ze-stat-width) takes its width from the same file.
//
// Method: serve a fixture site carrying the real site script from a temp
// directory, load a root page and a nested page in headless Chrome, read the
// header and body spans, swap the snapshot for different values and reload.
// The server counts snapshot requests per load and sends no-store, so a
// reload cannot answer from the browser cache.
func TestTheBrowserFillsEveryMarkedValueFromTheSnapshot(t *testing.T) {
	chrome := findChrome(t)

	dir := t.TempDir()
	writeFixtureSite(t, dir)
	factsFile := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(factsPath, "/")))

	var factsRequests atomic.Int64
	files := http.FileServer(http.Dir(dir))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == factsPath {
			factsRequests.Add(1)
		}
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	}))
	defer server.Close()

	options := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(chrome))
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(t.Context(), options...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	ctx, cancelTimeout := context.WithTimeout(browserCtx, 60*time.Second)
	defer cancelTimeout()

	versions := []snapshot{
		{commands: "490", proven: "2,888", percent: "63.4"},
		{commands: "512", proven: "3,001", percent: "71.2"},
	}
	pages := []string{"/", "/reference/cli/"}

	for _, page := range pages {
		for index, version := range versions {
			if err := os.MkdirAll(filepath.Dir(factsFile), 0o755); err != nil {
				t.Fatalf("create %s: %v", filepath.Dir(factsFile), err)
			}
			if err := os.WriteFile(factsFile, version.json(t), 0o644); err != nil {
				t.Fatalf("write the snapshot: %v", err)
			}
			factsRequests.Store(0)

			load := chromedp.Navigate(server.URL + page)
			if index > 0 {
				load = chromedp.Reload()
			}
			if _, err := chromedp.Run(ctx, load); err != nil {
				t.Fatalf("%s: load version %d: %v", page, index+1, err)
			}
			waitForTexts(ctx, t, page, version)

			if count := factsRequests.Load(); count != 1 {
				t.Errorf("%s version %d: %s fetched %d times in one page load, want once", page, index+1, factsPath, count)
			}
		}
	}
}
