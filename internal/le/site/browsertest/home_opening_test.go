// Design: website/AI.md -- the homepage opening explains Ze beside genuine playback.
package browsertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestHomeOpeningShowsOverviewAndRecording renders the current checkout and
// checks its initial viewport in Chrome, then starts the real recording and
// observes its playback clock advance. It exercises both themes at desktop,
// laptop and phone sizes without scrolling to find the opening actions.
func TestHomeOpeningShowsOverviewAndRecording(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	build := exec.CommandContext(t.Context(), filepath.Join(repository, "le"),
		"--name", "session", "site", "build", "output", output)
	build.Dir = repository
	if report, err := build.CombinedOutput(); err != nil {
		t.Fatalf("publish the current homepage: %v\n%s", err, report)
	}
	server := httptest.NewServer(http.FileServer(http.Dir(output)))
	t.Cleanup(server.Close)
	options := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(findChrome(t)))
	alloc, cancelAlloc := chromedp.NewExecAllocator(t.Context(), options...)
	t.Cleanup(cancelAlloc)
	browser, cancelBrowser := chromedp.NewContext(alloc)
	t.Cleanup(cancelBrowser)
	ctx, cancel := context.WithTimeout(browser, 180*time.Second)
	t.Cleanup(cancel)
	if _, err := chromedp.Run(ctx, chromedp.Navigate(server.URL)); err != nil {
		t.Fatalf("open the published homepage: %v", err)
	}
	for _, viewport := range []struct {
		name          string
		width, height int64
	}{
		{"desktop", 1440, 900},
		{"laptop", 1366, 768},
		{"phone", 390, 844},
	} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(viewport.name+"/"+theme, func(t *testing.T) {
				if _, err := chromedp.Run(ctx, chromedp.EmulateViewport(viewport.width, viewport.height)); err != nil {
					t.Fatal(err)
				}
				if _, err := chromedp.Run(ctx, chromedp.Evaluate[bool](
					`localStorage.setItem("ze-theme", `+strconv.Quote(theme)+`); true`)); err != nil {
					t.Fatal(err)
				}
				if _, err := chromedp.Run(ctx, chromedp.Navigate(server.URL)); err != nil {
					t.Fatal(err)
				}
				if _, err := chromedp.Run(ctx, chromedp.Poll[bool](homeOpeningReady,
					chromedp.WithPollingTimeout(45*time.Second))); err != nil {
					details, diagnosticErr := chromedp.Run(ctx, chromedp.Evaluate[string](readHomeOpeningReadiness))
					t.Fatalf("the real homepage player did not become ready: %v\ndiagnostics: %s\ndiagnostic error: %v", err, details, diagnosticErr)
				}
				issues, err := chromedp.Run(ctx, chromedp.Evaluate[[]string](readHomeOpening))
				if err != nil {
					t.Fatal(err)
				}
				if len(issues) != 0 {
					t.Fatalf("initial %dx%d viewport: %s", viewport.width, viewport.height, strings.Join(issues, "; "))
				}
				if _, err := chromedp.Run(ctx, chromedp.Click(chromedp.CSS("#product-terminal .ap-overlay-start"))); err != nil {
					t.Fatalf("start the real terminal recording: %v", err)
				}
				if _, err := chromedp.Run(ctx, chromedp.Poll[bool](homeOpeningPlaying,
					chromedp.WithPollingTimeout(15*time.Second))); err != nil {
					t.Fatalf("the recording clock did not advance after Play: %v", err)
				}
			})
		}
	}
}

// Readiness includes the asynchronous header and local fonts because both can
// move the opening after the browser first paints the authored HTML.
const homeOpeningReady = `(() => {
    const player = document.querySelector('#product-terminal .ap-player');
    return document.fonts.status === 'loaded' &&
        document.querySelector('header.site-header') !== null &&
        document.querySelector('[data-product-showcase][data-enhanced="true"]') !== null &&
        player !== null && player.querySelector('.ap-overlay-start') !== null &&
        player.querySelector('.ap-control-bar') !== null;
})()`

// A failed readiness poll reports each condition and the published resources,
// so an initialization failure cannot be mistaken for a viewport regression.
const readHomeOpeningReadiness = `(() => {
    const player = document.querySelector('#product-terminal .ap-player');
    const header = document.querySelector('header.site-header');
    const mount = document.querySelector('#site-header-mount');
    return JSON.stringify({
        url: location.href,
        document: document.readyState,
        fonts: document.fonts.status,
        headerInsideMount: document.querySelector('#site-header-mount .site-header') !== null,
        publishedHeader: header !== null,
        headerHTML: header ? header.outerHTML.slice(0, 600) : null,
        mountHTML: mount ? mount.outerHTML.slice(0, 600) : null,
        showcaseEnhanced: document.querySelector('[data-product-showcase][data-enhanced="true"]') !== null,
        player: player !== null,
        overlay: player ? player.querySelector('.ap-overlay-start') !== null : false,
        controls: player ? player.querySelector('.ap-control-bar') !== null : false,
        playerHTML: player ? player.outerHTML.slice(0, 2000) : null,
        scripts: Array.from(document.scripts).map(script => script.src || script.id),
        resources: performance.getEntriesByType('resource').filter(resource =>
            /(?:site\.js|header\.html|asciinema|showcase\.cast|manifest\.json)/.test(resource.name)
        ).map(resource => ({url: resource.name, duration: resource.duration, bytes: resource.transferSize}))
    });
})()`

// The selectors identify reader-visible purposes. Geometry comes from the
// browser after the real stylesheet and player have established their sizes.
const readHomeOpening = `(() => {
    const issues = [];
    function visible(node, label) {
        if (!node) { issues.push(label + ' is missing'); return null; }
        const box = node.getBoundingClientRect();
        const style = getComputedStyle(node);
        if (box.width <= 0 || box.height <= 0 || style.visibility !== 'visible' || style.display === 'none') {
            issues.push(label + ' is hidden');
        }
        return box;
    }
    function inViewport(node, label) {
        const box = visible(node, label);
        if (box && (box.top < 0 || box.bottom > innerHeight || box.left < 0 || box.right > innerWidth)) {
            issues.push(label + ' falls outside the initial viewport (top=' + Math.round(box.top) + ', bottom=' + Math.round(box.bottom) + ')');
        }
    }
    inViewport(document.querySelector('#hero-title'), 'product heading');
    inViewport(document.querySelector('.hero-intro'), 'product explanation');
    inViewport(document.querySelector('.hero-actions a.primary'), 'quickstart action');
    const watch = Array.from(document.querySelectorAll('.hero-actions a')).find(link => /watch/i.test(link.textContent));
    inViewport(watch, 'watch action');
    const player = document.querySelector('#product-terminal [data-terminal-demo-player]');
    const box = visible(player, 'real recording');
    if (player && !/\.cast(?:\?|$)/.test(player.getAttribute('data-cast-src') || '')) {
        issues.push('recording does not reference terminal event media');
    }
    if (box && innerWidth > 600) {
        inViewport(player, 'recording and controls');
        if (box.width < 400 || box.height < 240) issues.push('desktop recording is too small to inspect');
    }
    if (box && innerWidth <= 600) {
        const shown = Math.max(0, Math.min(box.bottom, innerHeight) - Math.max(box.top, 0));
        if (shown < 120) issues.push('phone shows less than 120px of the recording preview');
        const play = document.querySelector('#product-terminal .ap-overlay-start .ap-play-button');
        inViewport(play, 'phone playback action');
    }
    if (document.documentElement.scrollWidth > innerWidth + 1) issues.push('page overflows horizontally');
    if (window.scrollY !== 0) issues.push('page scrolled before opening inspection');
    return issues;
})()`

const homeOpeningPlaying = `(() => {
    const player = document.querySelector('#product-terminal .ap-player');
    if (!player || player.querySelector('.ap-overlay-start') || player.querySelector('.ap-overlay-error')) return false;
    const clock = player.querySelector('.ap-time-elapsed');
    return clock !== null && /[1-9]/.test(clock.textContent);
})()`
