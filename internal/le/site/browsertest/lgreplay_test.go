// Design: docs/contributing/gh-pages.md -- the looking glass replay answers its page from the recording.
package browsertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/ze-software/ze/internal/le/site"
	sitelgreplay "github.com/ze-software/ze/internal/le/site/lgreplay"
)

const (
	lgReplaySearchKey = "POST /lg/search form prefix=198.18.0.0/24 htmx"
	lgReplayTextKey   = "GET /lg/graph?format=text&mode=aspath&prefix=198.18.0.0/24"
	lgReplayMount     = `document.querySelector('[data-lg-replay]')`
)

// lgReplayBrowserRecording is a recording in the looking glass's own shapes:
// its real htmx, theme and graph-mode scripts and stylesheet, a page with the
// tab bar of layout.templ, the search form of search.templ, a search result,
// and a graph's text form. The search result also names one request the
// recording does not hold, which htmx asks as soon as the result is shown.
func lgReplayBrowserRecording(t *testing.T, repository string) sitelgreplay.Recording {
	t.Helper()
	asset := func(name, contentType string) sitelgreplay.Asset {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(repository, "internal", "component", "lg", "assets", name))
		if err != nil {
			t.Fatal(err)
		}
		return sitelgreplay.Asset{Path: "/lg/assets/" + name, ContentType: contentType, Body: string(content)}
	}
	const markup = "text/html; charset=utf-8"
	page := `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Peers - Ze Looking Glass</title>` +
		`<link rel="stylesheet" href="/lg/assets/style.css"><script src="/lg/assets/htmx.min.js"></script>` +
		`<script src="/lg/assets/theme.js"></script><script src="/lg/assets/graph-mode.js"></script></head>` +
		`<body><header><nav class="tab-bar">` +
		`<a href="/lg/peers" hx-get="/lg/peers" hx-target="#content" hx-push-url="true" class="tab-active">Peers</a>` +
		`<a href="/lg/search" hx-get="/lg/search" hx-target="#content" hx-push-url="true">Search</a>` +
		`</nav></header><main><div id="content"><p>eleven peers</p></div></main></body></html>`
	form := `<form hx-post="/lg/search" hx-target="#results" class="search-form">` +
		`<input type="text" id="prefix" name="prefix" value=""><input type="text" id="aspath" name="aspath" value="">` +
		`<select id="family" name="family"><option value="">All</option><option value="ipv4/unicast">IPv4 Unicast</option></select>` +
		`<button type="submit">Search</button></form><div id="results"></div>`
	result := `<p>seven routes</p><div id="unrecorded" hx-get="/lg/peer/192.0.2.9" hx-trigger="load"></div>`
	return sitelgreplay.Recording{
		Schema: sitelgreplay.Schema,
		Provenance: sitelgreplay.Provenance{
			Recorder: "browser test", Commit: "0123456789abcdef", CommitDate: "2026-11-18T20:00:00Z",
			Seed: 7, Speakers: 11, Normalized: []sitelgreplay.Normalization{}, Varies: []string{},
		},
		Entry: "GET /lg/peers",
		Tour: []sitelgreplay.TourStep{
			{Caption: "The peers page.", Exchange: "GET /lg/peers"},
			{Caption: "The search page.", Exchange: "GET /lg/search htmx"},
			{Caption: "Search by prefix.", Exchange: lgReplaySearchKey},
			{Caption: "The graph as text.", Exchange: lgReplayTextKey},
		},
		Exchanges: []sitelgreplay.Exchange{
			{Key: "GET /lg/peers", Method: "GET", Path: "/lg/peers", Via: "tour", Status: 200, ContentType: markup, Body: page},
			{Key: "GET /lg/search htmx", Method: "GET", Path: "/lg/search", HTMX: true, Via: "tour", Status: 200, ContentType: markup, Body: form},
			{
				Key: lgReplaySearchKey, Method: "POST", Path: "/lg/search", HTMX: true, Via: "tour",
				Form:   []sitelgreplay.Field{{Name: "prefix", Value: "198.18.0.0/24"}},
				Status: 200, ContentType: markup, Body: result,
			},
			{
				Key: lgReplayTextKey, Method: "GET", Path: "/lg/graph", Via: "tour",
				Query: []sitelgreplay.Field{
					{Name: "prefix", Value: "198.18.0.0/24"}, {Name: "mode", Value: "aspath"}, {Name: "format", Value: "text"},
				},
				Status: 200, ContentType: "text/plain; charset=utf-8", Body: "AS65000 -> AS64501\n",
			},
		},
		Assets: []sitelgreplay.Asset{
			asset("style.css", "text/css; charset=utf-8"),
			asset("htmx.min.js", "text/javascript; charset=utf-8"),
			asset("theme.js", "text/javascript; charset=utf-8"),
			asset("graph-mode.js", "text/javascript; charset=utf-8"),
		},
	}
}

// lgReplayBrowserSite renders the replay of one recording with the real
// producer and the real player, serves it, and answers its address and the
// log of every path the server was asked for.
func lgReplayBrowserSite(t *testing.T) (string, func() []string) {
	t.Helper()
	repository, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	checkout := t.TempDir()
	paths := site.Paths{Repository: checkout, Source: filepath.Join(checkout, "website"), Output: t.TempDir()}
	player := filepath.Join(repository, "website", "assets", "js", "lg-replay.js")
	copyBrowserInput(t, player, filepath.Join(paths.Source, "assets", "js", "lg-replay.js"))
	// The producer publishes the player with the shared assets, and an asset
	// render requires these two files of a checkout.
	for name, content := range map[string]string{
		"assets/js/site.js":   "/* the site */\n",
		"assets/css/site.css": ".lg-replay__frame { width: 100%; height: 30rem; }\n",
	} {
		path := filepath.Join(paths.Source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	content, err := lgReplayBrowserRecording(t, repository).Encode()
	if err != nil {
		t.Fatal(err)
	}
	recorded := filepath.Join(checkout, filepath.FromSlash(sitelgreplay.File))
	if err := os.MkdirAll(filepath.Dir(recorded), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recorded, content, 0o600); err != nil {
		t.Fatal(err)
	}
	renderBrowserProducer(t, paths, "lg-replay")

	// The host page carries what the lg-replay block carries: the mount, one
	// list item per tour step, and the player.
	// Its first script keeps what goes wrong in the host page, where the test
	// reads it; the player keeps what goes wrong in the frame on the mount.
	host := `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Replay</title>` +
		`<link rel="stylesheet" href="../../assets/site.css"><script>window.lgReplayHostErrors = [];` +
		`window.addEventListener('error', function (e) { window.lgReplayHostErrors.push(e.message + ' (' + e.filename + ':' + e.lineno + ')'); });` +
		`window.addEventListener('unhandledrejection', function (e) { window.lgReplayHostErrors.push('unhandled rejection: ' + e.reason); });` +
		`</script></head><body>` +
		`<div class="lg-replay"><div class="lg-replay__player" data-lg-replay data-lg-replay-frame="replay/frame.html"><p class="lg-replay__fallback">fallback</p></div><ol>` +
		`<li data-lg-replay-step="0"><span data-lg-replay-caption>The peers page.</span></li>` +
		`<li data-lg-replay-step="1"><span data-lg-replay-caption>The search page.</span></li>` +
		`<li data-lg-replay-step="2"><span data-lg-replay-caption>Search by prefix.</span></li>` +
		`<li data-lg-replay-step="3"><span data-lg-replay-caption>The graph as text.</span></li>` +
		`</ol><script src="../../assets/lg-replay.js" defer></script></div></body></html>`
	page := filepath.Join(paths.Output, "labs", "looking-glass-graph", "index.html")
	if err := os.WriteFile(page, []byte(host), 0o600); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var asked []string
	files := http.FileServer(http.Dir(paths.Output))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.URL.Path)
		mu.Unlock()
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string{}, asked...)
	}
}

// TestLGReplayAnswersTheTourFromTheRecording proves, in a browser, that the
// replay plays every step of the tour from the recording, shows the notice for
// a request the recording does not hold, never starts by itself, and sends no
// looking glass request to the server.
//
// Method: the frame is sandboxed, so the test reads what the host page
// records about it on the mount, and the web server's own request log.
func TestLGReplayAnswersTheTourFromTheRecording(t *testing.T) {
	base, asked := lgReplayBrowserSite(t)
	ctx := newRFCBrowser(t)
	if _, err := chromedp.Run(ctx, chromedp.Navigate(base+"/labs/looking-glass-graph/")); err != nil {
		t.Fatal(err)
	}
	attribute := func(name string) string {
		t.Helper()
		value, err := chromedp.Run(ctx, chromedp.Evaluate[string](lgReplayMount+`.getAttribute('`+name+`') || ''`))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	// seen answers what the browser did, for a failure message: the host
	// page's own errors, and everything the player recorded on the mount,
	// the frame's errors included.
	seen := func() string {
		value, err := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
            const mount = `+lgReplayMount+`;
            const lines = ['host errors: ' + JSON.stringify(window.lgReplayHostErrors || 'not collected')];
            if (!mount) return lines.concat('the page holds no [data-lg-replay] mount').join('\n');
            for (const attribute of Array.from(mount.attributes)) lines.push(attribute.name + '=' + attribute.value);
            lines.push('mount children: ' + Array.from(mount.children).map(child => child.tagName + '.' + child.className).join(' '));
            const status = mount.querySelector('.lg-replay__status');
            lines.push('status: ' + (status ? status.textContent : 'no status element'));
            const caption = mount.querySelector('.lg-replay__caption');
            lines.push('caption: ' + (caption ? caption.textContent : 'no caption element'));
            return lines.join('\n');
        })()`))
		if err != nil {
			return "the page could not be read: " + err.Error()
		}
		return value
	}
	// await waits for a state of the page, and on a timeout says what was
	// awaited and what the browser did instead.
	await := func(what, condition string) {
		t.Helper()
		if _, err := chromedp.Run(ctx, chromedp.Poll[bool](condition, chromedp.WithPollingTimeout(10*time.Second))); err != nil {
			t.Fatalf("waited for %s (%s): %v\n%s", what, condition, err, seen())
		}
	}
	// text waits for the element, then answers its text.
	text := func(selector string) string {
		t.Helper()
		await("the element "+selector, `document.querySelector('`+selector+`') !== null`)
		value, err := chromedp.Run(ctx, chromedp.Evaluate[string](`document.querySelector('`+selector+`').textContent`))
		if err != nil {
			t.Fatalf("read %s: %v\n%s", selector, err, seen())
		}
		return value
	}
	press := func(label string) {
		t.Helper()
		if _, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => {
            const button = Array.from(document.querySelectorAll('.lg-replay__button')).find(b => b.textContent === '`+label+`');
            if (!button) return false;
            button.click();
            return true;
        })()`)); err != nil {
			t.Fatal(err)
		}
	}

	await("the frame to show the first recorded page",
		lgReplayMount+` !== null && `+lgReplayMount+`.getAttribute('data-lg-replay-state') === 'ready'`)
	// Nothing plays until a control is pressed.
	if step := attribute("data-lg-replay-step"); step != "" {
		t.Fatalf("the tour played step %s with no control pressed\n%s", step, seen())
	}
	if strings.Contains(text(".lg-replay__player"), "fallback") {
		t.Fatalf("the no-script fallback is still shown beside the player\n%s", seen())
	}

	play := func(step string) {
		t.Helper()
		press("Next step")
		await("tour step "+step+" to finish", lgReplayMount+`.getAttribute('data-lg-replay-step') === '`+step+`'`)
	}
	// The page, the search tab, then the search form filled and submitted.
	play("0")
	play("1")
	play("2")
	// The form the page submitted is the request the recording answered.
	if answered := attribute("data-lg-replay-answered"); answered != lgReplaySearchKey {
		t.Fatalf("last request answered from the recording: %q, want %q", answered, lgReplaySearchKey)
	}

	// The search result asks for a peer the recording does not hold.
	await("the notice for the unrecorded request",
		lgReplayMount+`.getAttribute('data-lg-replay-missing') === 'GET /lg/peer/192.0.2.9'`)
	if status := text(".lg-replay__status"); !strings.HasPrefix(status, "Not in this recording: GET /lg/peer/192.0.2.9.") {
		t.Fatalf("notice for an unrecorded request: %q\n%s", status, seen())
	}
	// Nothing went wrong on either side of the frame while the tour played.
	// The unrecorded request is answered by the player, so the frame's policy
	// had nothing to refuse.
	if errors := attribute("data-lg-replay-errors"); errors != "" {
		t.Errorf("the frame reported: %s", errors)
	}

	// The graph's text form has no control on the page: the host shows it.
	play("3")
	if caption := text(".lg-replay__caption"); caption != "Step 4 of 4: The graph as text." {
		t.Fatalf("caption after the last step: %q", caption)
	}
	if answer := text(".lg-replay__answer"); answer != "AS65000 -> AS64501\n" {
		t.Fatalf("the text step shows %q, and the recording holds the graph's text", answer)
	}

	// Restart from a middle step. The tour is taken back to the search page
	// first, with two presses made at once: the second arrives while the
	// first step is still loading, and it is kept.
	press("Restart")
	press("Next step")
	press("Next step")
	await("two presses made at once to reach the search form's result",
		lgReplayMount+`.getAttribute('data-lg-replay-step') === '2'`)
	loads := attribute("data-lg-replay-loads")
	press("Restart")
	// Restart opens the entry page in a new frame document, and only then
	// plays the first step: the page the frame says it shows is the entry,
	// and it was loaded after the press.
	await("Restart to play step 1 on a newly loaded entry page",
		lgReplayMount+`.getAttribute('data-lg-replay-step') === '0' && `+
			lgReplayMount+`.getAttribute('data-lg-replay-page') === 'GET /lg/peers' && `+
			lgReplayMount+`.getAttribute('data-lg-replay-loads') !== '`+loads+`'`)
	if caption := text(".lg-replay__caption"); caption != "Step 1 of 4: The peers page." {
		t.Fatalf("caption after Restart: %q\n%s", caption, seen())
	}
	// More presses than steps are left, none waited for: the tour plays every
	// remaining step, ends on the last one, and no step is reported unfinished.
	for range 5 {
		press("Next step")
	}
	await("presses made without waiting to play the tour to its last step",
		lgReplayMount+`.getAttribute('data-lg-replay-step') === '3'`)
	if caption := text(".lg-replay__caption"); caption != "Step 4 of 4: The graph as text." {
		t.Fatalf("caption after the tour played to its end again: %q\n%s", caption, seen())
	}
	if answer := text(".lg-replay__answer"); answer != "AS65000 -> AS64501\n" {
		t.Fatalf("the last step shows %q after a restart\n%s", answer, seen())
	}
	if status := text(".lg-replay__status"); strings.Contains(status, "did not finish") {
		t.Fatalf("a step was reported unfinished: %q\n%s", status, seen())
	}
	if status := text(".lg-replay__status"); strings.Contains(status, "is loading") {
		t.Fatalf("the status line still says a step is loading after the tour ended: %q\n%s", status, seen())
	}

	// The server was asked for the page, the player and the frame, and for
	// nothing of the looking glass.
	for _, path := range asked() {
		if strings.HasPrefix(path, "/lg/") {
			t.Errorf("a looking glass request left the page: %s", path)
		}
	}
	want := map[string]bool{
		"/labs/looking-glass-graph/": true, "/assets/lg-replay.js": true, "/assets/site.css": true,
		"/labs/looking-glass-graph/replay/frame.html": true, "/favicon.ico": true,
	}
	for _, path := range asked() {
		if !want[path] {
			t.Errorf("the page asked the server for %s", path)
		}
	}
}

// TestLGReplayFrameOpenedDirectlyRunsNothingRecorded proves, in a browser,
// that the frame document opened with no frame around it puts no recorded
// markup in place and runs no recorded script. Opened that way it has the
// site's own origin, which is what the page's sandbox keeps the looking
// glass's scripts out of.
//
// Method: the recorded page loads htmx, which defines window.htmx as it runs,
// and carries the looking glass's tab bar. The test opens frame.html itself and
// reads what the document holds once it has loaded.
func TestLGReplayFrameOpenedDirectlyRunsNothingRecorded(t *testing.T) {
	base, _ := lgReplayBrowserSite(t)
	ctx := newRFCBrowser(t)
	if _, err := chromedp.Run(ctx, chromedp.Navigate(base+"/labs/looking-glass-graph/replay/frame.html")); err != nil {
		t.Fatal(err)
	}
	observed, err := chromedp.Run(ctx, chromedp.Evaluate[string](`[
        'state=' + document.documentElement.getAttribute('data-lg-replay-frame'),
        'htmx=' + typeof window.htmx,
        'recorded scripts and styles in head=' + document.querySelectorAll('head script, head style').length,
        'scripts=' + document.querySelectorAll('script').length,
        'tab bar=' + (document.querySelector('.tab-bar') !== null),
        'title=' + document.title,
        'notice=' + (document.getElementById('lg-replay-notice') !== null),
        'notice link=' + document.querySelector('#lg-replay-notice a').getAttribute('href')
    ].join('\n')`))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"state=refused: opened outside its page",
		"htmx=undefined",
		"recorded scripts and styles in head=0",
		// The recording, as data, and the player: the two the producer wrote.
		"scripts=2",
		"tab bar=false",
		"title=Ze looking glass replay",
		"notice=true",
		"notice link=../",
	}, "\n")
	if observed != want {
		t.Fatalf("the frame document opened directly:\n got:\n%s\nwant:\n%s", observed, want)
	}
}

// TestLGReplayKeepsABoundedListOfFrameErrors proves the mount keeps the first
// twenty errors the frame reports, each cut to three hundred characters, and
// counts the ones after them, so a frame that raises an error on every request
// cannot grow the page without limit.
//
// Method: the frame is sandboxed and raises no error by itself, so the test
// hands the host page the messages the frame would send, with the frame's
// window as their source, which is the one the player listens to.
func TestLGReplayKeepsABoundedListOfFrameErrors(t *testing.T) {
	base, _ := lgReplayBrowserSite(t)
	ctx := newRFCBrowser(t)
	if _, err := chromedp.Run(ctx, chromedp.Navigate(base+"/labs/looking-glass-graph/")); err != nil {
		t.Fatal(err)
	}
	if _, err := chromedp.Run(ctx, chromedp.Poll[bool](
		lgReplayMount+` !== null && `+lgReplayMount+`.getAttribute('data-lg-replay-state') === 'ready'`,
		chromedp.WithPollingTimeout(10*time.Second))); err != nil {
		t.Fatalf("the frame never showed the first recorded page: %v", err)
	}
	observed, err := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
        const mount = `+lgReplayMount+`;
        const frame = mount.querySelector('iframe').contentWindow;
        for (let index = 0; index < 25; index++) {
            window.dispatchEvent(new MessageEvent('message', {
                source: frame,
                data: { from: 'lg-replay-frame', type: 'error', text: 'error ' + index + ' ' + 'x'.repeat(400) }
            }));
        }
        const kept = mount.getAttribute('data-lg-replay-errors').split(' | ');
        return [
            'kept=' + kept.length,
            'longest=' + Math.max(...kept.map(line => line.length)),
            'first=' + kept[0].slice(0, 8),
            'last=' + kept[kept.length - 1].slice(0, 8),
            'dropped=' + mount.getAttribute('data-lg-replay-errors-dropped')
        ].join('\n');
    })()`))
	if err != nil {
		t.Fatal(err)
	}
	want := "kept=20\nlongest=300\nfirst=error 0 \nlast=error 19\ndropped=5"
	if observed != want {
		t.Fatalf("the errors the mount keeps:\n got:\n%s\nwant:\n%s", observed, want)
	}
}
